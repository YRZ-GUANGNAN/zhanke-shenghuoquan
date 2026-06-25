package takeoutpayment

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/takeout"
)

var (
	ErrProviderNotConfigured = errors.New("payment provider is not configured")
	ErrProviderRejected      = errors.New("payment provider rejected request")
	ErrInvalidNotification   = errors.New("invalid payment notification")
)

type Provider interface {
	Provider() takeout.PaymentProvider
	CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error)
	ClientParams(string) (*JSAPIPaymentParameter, error)
	ParseNotification(*http.Request, []byte) (NotificationResult, error)
}

type PrepayRequest struct {
	Description string
	OutTradeNo  string
	AmountCents uint64
	OpenID      string
}

type PrepayResult struct {
	PrepayID string
	JSAPI    JSAPIPaymentParameter
}

type NotificationResult struct {
	OutTradeNo          string
	WechatTransactionID string
	AmountCents         uint64
	PaidAt              time.Time
}

func NewProviderFromConfig(cfg config.Config) (Provider, error) {
	switch {
	case localMockPaymentMode(cfg):
		return MockProvider{}, nil
	case cfg.WechatPayConfigured():
		return NewWechatProvider(WechatProviderConfig{
			AppID: cfg.WechatAppID, MchID: cfg.WechatPayMchID, APIv3Key: cfg.WechatPayAPIv3Key,
			MerchantSerialNo: cfg.WechatPayMerchantSerialNo, PrivateKeyPath: cfg.WechatPayPrivateKeyPath,
			NotifyURL: cfg.WechatPayNotifyURL, APIBase: cfg.WechatPayAPIBase,
			PlatformPublicKeyPath: cfg.WechatPayPlatformKeyPath,
		})
	default:
		return UnconfiguredProvider{}, nil
	}
}

func localMockPaymentMode(cfg config.Config) bool {
	return cfg.AppEnv != "prod" && cfg.AllowMockPayment && !cfg.WechatPayAnyCredentialSet()
}

type MockProvider struct{}

func (MockProvider) Provider() takeout.PaymentProvider { return takeout.PaymentProviderMock }
func (MockProvider) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	return PrepayResult{}, ErrProviderNotConfigured
}
func (MockProvider) ClientParams(string) (*JSAPIPaymentParameter, error) {
	return nil, ErrProviderNotConfigured
}
func (MockProvider) ParseNotification(*http.Request, []byte) (NotificationResult, error) {
	return NotificationResult{}, ErrProviderNotConfigured
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) Provider() takeout.PaymentProvider { return takeout.PaymentProviderWechat }
func (UnconfiguredProvider) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	return PrepayResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ClientParams(string) (*JSAPIPaymentParameter, error) {
	return nil, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ParseNotification(*http.Request, []byte) (NotificationResult, error) {
	return NotificationResult{}, ErrProviderNotConfigured
}

type WechatProviderConfig struct {
	AppID                 string
	MchID                 string
	APIv3Key              string
	MerchantSerialNo      string
	PrivateKeyPath        string
	NotifyURL             string
	APIBase               string
	PlatformPublicKeyPath string
}

type WechatProvider struct {
	config      WechatProviderConfig
	privateKey  *rsa.PrivateKey
	platformKey *rsa.PublicKey
	httpClient  *http.Client
	nonce       func() (string, error)
	now         func() time.Time
}

func NewWechatProvider(cfg WechatProviderConfig) (*WechatProvider, error) {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.mch.weixin.qq.com"
	}
	key, err := loadRSAPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	platformKey, err := loadRSAPublicKeyOptional(cfg.PlatformPublicKeyPath)
	if err != nil {
		return nil, err
	}
	return &WechatProvider{
		config: cfg, privateKey: key, platformKey: platformKey,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		nonce:      randomNonce,
		now:        time.Now,
	}, nil
}

func (p *WechatProvider) Provider() takeout.PaymentProvider { return takeout.PaymentProviderWechat }

func (p *WechatProvider) CreatePrepay(ctx context.Context, request PrepayRequest) (PrepayResult, error) {
	if p.config.AppID == "" || p.config.MchID == "" || p.config.NotifyURL == "" || request.OpenID == "" ||
		request.OutTradeNo == "" || request.AmountCents == 0 {
		return PrepayResult{}, ErrProviderNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"appid":        p.config.AppID,
		"mchid":        p.config.MchID,
		"description":  request.Description,
		"out_trade_no": request.OutTradeNo,
		"notify_url":   p.config.NotifyURL,
		"amount":       map[string]any{"total": request.AmountCents, "currency": "CNY"},
		"payer":        map[string]any{"openid": request.OpenID},
	})
	if err != nil {
		return PrepayResult{}, err
	}
	endpoint := strings.TrimRight(p.config.APIBase, "/") + "/v3/pay/transactions/jsapi"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return PrepayResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := p.authorize(req, body); err != nil {
		return PrepayResult{}, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return PrepayResult{}, err
	}
	defer resp.Body.Close()
	var decoded struct {
		PrepayID string `json:"prepay_id"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return PrepayResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || decoded.PrepayID == "" {
		return PrepayResult{}, fmt.Errorf("%w: http=%d code=%s message=%s", ErrProviderRejected, resp.StatusCode, decoded.Code, decoded.Message)
	}
	params, err := p.ClientParams(decoded.PrepayID)
	if err != nil {
		return PrepayResult{}, err
	}
	return PrepayResult{PrepayID: decoded.PrepayID, JSAPI: *params}, nil
}

func (p *WechatProvider) ClientParams(prepayID string) (*JSAPIPaymentParameter, error) {
	if strings.TrimSpace(prepayID) == "" {
		return nil, ErrInvalidNotification
	}
	nonce, err := p.nonce()
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(p.now().Unix(), 10)
	pkg := "prepay_id=" + strings.TrimSpace(prepayID)
	message := p.config.AppID + "\n" + timestamp + "\n" + nonce + "\n" + pkg + "\n"
	signature, err := signRSA(p.privateKey, []byte(message))
	if err != nil {
		return nil, err
	}
	return &JSAPIPaymentParameter{TimeStamp: timestamp, NonceStr: nonce, Package: pkg, SignType: "RSA", PaySign: signature}, nil
}

func (p *WechatProvider) ParseNotification(req *http.Request, body []byte) (NotificationResult, error) {
	if p.platformKey == nil {
		return NotificationResult{}, ErrProviderNotConfigured
	}
	timestamp := req.Header.Get("Wechatpay-Timestamp")
	nonce := req.Header.Get("Wechatpay-Nonce")
	signature := req.Header.Get("Wechatpay-Signature")
	if timestamp == "" || nonce == "" || signature == "" {
		return NotificationResult{}, ErrInvalidNotification
	}
	signed := []byte(timestamp + "\n" + nonce + "\n" + string(body) + "\n")
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return NotificationResult{}, ErrInvalidNotification
	}
	digest := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(p.platformKey, crypto.SHA256, digest[:], rawSignature); err != nil {
		return NotificationResult{}, ErrInvalidNotification
	}

	var envelope struct {
		Resource struct {
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
			Ciphertext     string `json:"ciphertext"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return NotificationResult{}, err
	}
	plain, err := decryptWechatResource(p.config.APIv3Key, envelope.Resource.AssociatedData, envelope.Resource.Nonce, envelope.Resource.Ciphertext)
	if err != nil {
		return NotificationResult{}, err
	}
	var transaction struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		SuccessTime   string `json:"success_time"`
		Amount        struct {
			Total uint64 `json:"total"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &transaction); err != nil {
		return NotificationResult{}, err
	}
	if transaction.TradeState != "SUCCESS" || transaction.OutTradeNo == "" || transaction.TransactionID == "" || transaction.Amount.Total == 0 {
		return NotificationResult{}, ErrInvalidNotification
	}
	paidAt := time.Now().UTC()
	if transaction.SuccessTime != "" {
		if parsed, err := time.Parse(time.RFC3339, transaction.SuccessTime); err == nil {
			paidAt = parsed.UTC()
		}
	}
	return NotificationResult{
		OutTradeNo: transaction.OutTradeNo, WechatTransactionID: transaction.TransactionID,
		AmountCents: transaction.Amount.Total, PaidAt: paidAt,
	}, nil
}

func (p *WechatProvider) authorize(req *http.Request, body []byte) error {
	nonce, err := p.nonce()
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(p.now().Unix(), 10)
	requestURI := req.URL.RequestURI()
	message := req.Method + "\n" + requestURI + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := signRSA(p.privateKey, []byte(message))
	if err != nil {
		return err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		p.config.MchID, nonce, signature, timestamp, p.config.MerchantSerialNo)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	return nil
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrProviderNotConfigured
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, ErrProviderNotConfigured
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrProviderNotConfigured
	}
	return key, nil
}

func loadRSAPublicKeyOptional(path string) (*rsa.PublicKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, ErrProviderNotConfigured
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		key, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, ErrProviderNotConfigured
		}
		return key, nil
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, ErrProviderNotConfigured
	}
	return key, nil
}

func signRSA(key *rsa.PrivateKey, message []byte) (string, error) {
	digest := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func decryptWechatResource(apiKey, associatedData, nonce, ciphertext string) ([]byte, error) {
	if len(apiKey) != 32 {
		return nil, ErrProviderNotConfigured
	}
	rawCiphertext, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(apiKey))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), rawCiphertext, []byte(associatedData))
}

func randomNonce() (string, error) {
	var buf [16]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func encodeDescription(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "校园外卖订单"
	}
	decoded, err := url.QueryUnescape(value)
	if err == nil {
		value = decoded
	}
	if len([]rune(value)) > 127 {
		runes := []rune(value)
		value = string(runes[:127])
	}
	return value
}
