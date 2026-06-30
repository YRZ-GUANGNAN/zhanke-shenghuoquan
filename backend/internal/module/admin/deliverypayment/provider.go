package deliverypayment

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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
)

var (
	ErrProviderNotConfigured = errors.New("delivery payment provider is not configured")
	ErrProviderRejected      = errors.New("delivery payment provider rejected request")
	ErrInvalidNotification   = errors.New("invalid delivery payment notification")
)

type JSAPIParameter struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type PrepayRequest struct {
	Description string
	OutTradeNo  string
	AmountCents uint64
	OpenID      string
}

type PrepayResult struct {
	PrepayID string
	JSAPI    JSAPIParameter
}

type PaymentNotificationResult struct {
	OutTradeNo          string
	WechatTransactionID string
	AmountCents         uint64
	PaidAt              time.Time
}

type RefundRequest struct {
	OutTradeNo  string
	OutRefundNo string
	AmountCents uint64
	Reason      string
}

type RefundProviderResult struct {
	WechatRefundID string
	Status         string
}

type RefundNotificationResult struct {
	OutRefundNo    string
	WechatRefundID string
	Success        bool
	Reason         string
}

type Provider interface {
	Provider() delivery.PaymentProvider
	CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error)
	ClientParams(string) (*JSAPIParameter, error)
	ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error)
	CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error)
	ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error)
}

func NewProviderFromConfig(cfg config.Config) (Provider, error) {
	switch {
	case localMockMode(cfg):
		return MockProvider{}, nil
	case cfg.DeliveryWechatPayConfigured():
		return NewWechatProvider(WechatProviderConfig{
			AppID: cfg.WechatAppID, MchID: cfg.WechatPayMchID, APIv3Key: cfg.WechatPayAPIv3Key,
			MerchantSerialNo: cfg.WechatPayMerchantSerialNo, PrivateKeyPath: cfg.WechatPayPrivateKeyPath,
			PaymentNotifyURL: cfg.DeliveryWechatPayNotifyURL, RefundNotifyURL: cfg.DeliveryWechatRefundNotifyURL,
			APIBase: cfg.WechatPayAPIBase, PlatformPublicKeyPath: cfg.WechatPayPlatformKeyPath,
		})
	default:
		return UnconfiguredProvider{}, nil
	}
}

func localMockMode(cfg config.Config) bool {
	return cfg.AppEnv != "prod" && cfg.EnableDeliveryPayment && cfg.AllowMockDeliveryPayment &&
		!cfg.DeliveryWechatPayAnyCredentialSet()
}

type MockProvider struct{}

func (MockProvider) Provider() delivery.PaymentProvider { return delivery.PaymentMock }
func (MockProvider) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	return PrepayResult{}, ErrProviderNotConfigured
}
func (MockProvider) ClientParams(string) (*JSAPIParameter, error) {
	return nil, ErrProviderNotConfigured
}
func (MockProvider) ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error) {
	return PaymentNotificationResult{}, ErrProviderNotConfigured
}
func (MockProvider) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, ErrProviderNotConfigured
}
func (MockProvider) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, ErrProviderNotConfigured
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) Provider() delivery.PaymentProvider { return delivery.PaymentWechat }
func (UnconfiguredProvider) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	return PrepayResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ClientParams(string) (*JSAPIParameter, error) {
	return nil, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error) {
	return PaymentNotificationResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, ErrProviderNotConfigured
}

type WechatProviderConfig struct {
	AppID                 string
	MchID                 string
	APIv3Key              string
	MerchantSerialNo      string
	PrivateKeyPath        string
	PaymentNotifyURL      string
	RefundNotifyURL       string
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
	privateKey, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	platformKey, err := loadPublicKey(cfg.PlatformPublicKeyPath)
	if err != nil {
		return nil, err
	}
	return &WechatProvider{
		config: cfg, privateKey: privateKey, platformKey: platformKey,
		httpClient: &http.Client{Timeout: 8 * time.Second}, nonce: randomNonce, now: time.Now,
	}, nil
}

func (p *WechatProvider) Provider() delivery.PaymentProvider { return delivery.PaymentWechat }

func (p *WechatProvider) CreatePrepay(ctx context.Context, input PrepayRequest) (PrepayResult, error) {
	if input.OpenID == "" || input.OutTradeNo == "" || input.AmountCents == 0 ||
		p.config.AppID == "" || p.config.MchID == "" || p.config.PaymentNotifyURL == "" {
		return PrepayResult{}, ErrProviderNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"appid": p.config.AppID, "mchid": p.config.MchID,
		"description": input.Description, "out_trade_no": input.OutTradeNo,
		"notify_url": p.config.PaymentNotifyURL,
		"amount":     map[string]any{"total": input.AmountCents, "currency": "CNY"},
		"payer":      map[string]any{"openid": input.OpenID},
	})
	if err != nil {
		return PrepayResult{}, err
	}
	var response struct {
		PrepayID string `json:"prepay_id"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "/v3/pay/transactions/jsapi", body, &response); err != nil {
		return PrepayResult{}, err
	}
	if response.PrepayID == "" {
		return PrepayResult{}, fmt.Errorf("%w: code=%s message=%s", ErrProviderRejected, response.Code, response.Message)
	}
	params, err := p.ClientParams(response.PrepayID)
	if err != nil {
		return PrepayResult{}, err
	}
	return PrepayResult{PrepayID: response.PrepayID, JSAPI: *params}, nil
}

func (p *WechatProvider) ClientParams(prepayID string) (*JSAPIParameter, error) {
	prepayID = strings.TrimSpace(prepayID)
	if prepayID == "" {
		return nil, ErrInvalidNotification
	}
	nonce, err := p.nonce()
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(p.now().Unix(), 10)
	pkg := "prepay_id=" + prepayID
	message := p.config.AppID + "\n" + timestamp + "\n" + nonce + "\n" + pkg + "\n"
	signature, err := signRSA(p.privateKey, []byte(message))
	if err != nil {
		return nil, err
	}
	return &JSAPIParameter{TimeStamp: timestamp, NonceStr: nonce, Package: pkg, SignType: "RSA", PaySign: signature}, nil
}

func (p *WechatProvider) ParsePaymentNotification(req *http.Request, body []byte) (PaymentNotificationResult, error) {
	plain, err := p.verifyAndDecrypt(req, body)
	if err != nil {
		return PaymentNotificationResult{}, err
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
		return PaymentNotificationResult{}, err
	}
	if transaction.TradeState != "SUCCESS" || transaction.OutTradeNo == "" ||
		transaction.TransactionID == "" || transaction.Amount.Total == 0 {
		return PaymentNotificationResult{}, ErrInvalidNotification
	}
	paidAt := p.now().UTC()
	if parsed, err := time.Parse(time.RFC3339, transaction.SuccessTime); err == nil {
		paidAt = parsed.UTC()
	}
	return PaymentNotificationResult{
		OutTradeNo: transaction.OutTradeNo, WechatTransactionID: transaction.TransactionID,
		AmountCents: transaction.Amount.Total, PaidAt: paidAt,
	}, nil
}

func (p *WechatProvider) CreateRefund(ctx context.Context, input RefundRequest) (RefundProviderResult, error) {
	if input.OutTradeNo == "" || input.OutRefundNo == "" || input.AmountCents == 0 || p.config.RefundNotifyURL == "" {
		return RefundProviderResult{}, ErrProviderNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"out_trade_no": input.OutTradeNo, "out_refund_no": input.OutRefundNo,
		"reason": input.Reason, "notify_url": p.config.RefundNotifyURL,
		"amount": map[string]any{"refund": input.AmountCents, "total": input.AmountCents, "currency": "CNY"},
	})
	if err != nil {
		return RefundProviderResult{}, err
	}
	var response struct {
		RefundID string `json:"refund_id"`
		Status   string `json:"status"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "/v3/refund/domestic/refunds", body, &response); err != nil {
		return RefundProviderResult{}, err
	}
	if response.RefundID == "" {
		return RefundProviderResult{}, fmt.Errorf("%w: code=%s message=%s", ErrProviderRejected, response.Code, response.Message)
	}
	return RefundProviderResult{WechatRefundID: response.RefundID, Status: response.Status}, nil
}

func (p *WechatProvider) ParseRefundNotification(req *http.Request, body []byte) (RefundNotificationResult, error) {
	plain, err := p.verifyAndDecrypt(req, body)
	if err != nil {
		return RefundNotificationResult{}, err
	}
	var value struct {
		OutRefundNo string `json:"out_refund_no"`
		RefundID    string `json:"refund_id"`
		Status      string `json:"refund_status"`
	}
	if err := json.Unmarshal(plain, &value); err != nil {
		return RefundNotificationResult{}, err
	}
	if value.OutRefundNo == "" || value.RefundID == "" || value.Status == "" {
		return RefundNotificationResult{}, ErrInvalidNotification
	}
	success := value.Status == "SUCCESS"
	reason := ""
	if !success {
		reason = "wechat refund status: " + value.Status
	}
	return RefundNotificationResult{
		OutRefundNo: value.OutRefundNo, WechatRefundID: value.RefundID,
		Success: success, Reason: reason,
	}, nil
}

func (p *WechatProvider) verifyAndDecrypt(req *http.Request, body []byte) ([]byte, error) {
	timestamp := req.Header.Get("Wechatpay-Timestamp")
	nonce := req.Header.Get("Wechatpay-Nonce")
	signature := req.Header.Get("Wechatpay-Signature")
	if timestamp == "" || nonce == "" || signature == "" {
		return nil, ErrInvalidNotification
	}
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || absDuration(p.now().Sub(time.Unix(unixTime, 0))) > 5*time.Minute {
		return nil, ErrInvalidNotification
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, ErrInvalidNotification
	}
	signed := []byte(timestamp + "\n" + nonce + "\n" + string(body) + "\n")
	digest := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(p.platformKey, crypto.SHA256, digest[:], rawSignature); err != nil {
		return nil, ErrInvalidNotification
	}
	var envelope struct {
		Resource struct {
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
			Ciphertext     string `json:"ciphertext"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	return decryptResource(
		p.config.APIv3Key, envelope.Resource.AssociatedData,
		envelope.Resource.Nonce, envelope.Resource.Ciphertext,
	)
}

func (p *WechatProvider) doJSON(ctx context.Context, method, path string, body []byte, target any) error {
	endpoint := strings.TrimRight(p.config.APIBase, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := p.authorize(req, body); err != nil {
		return err
	}
	response, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: http=%d body=%s", ErrProviderRejected, response.StatusCode, truncateProviderText(string(raw), 512))
	}
	return json.Unmarshal(raw, target)
}

func (p *WechatProvider) authorize(req *http.Request, body []byte) error {
	nonce, err := p.nonce()
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(p.now().Unix(), 10)
	message := req.Method + "\n" + req.URL.RequestURI() + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := signRSA(p.privateKey, []byte(message))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		p.config.MchID, nonce, signature, timestamp, p.config.MerchantSerialNo,
	))
	req.Header.Set("Accept", "application/json")
	return nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, ErrProviderNotConfigured
	}
	if value, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return value, nil
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := value.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrProviderNotConfigured
	}
	return key, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
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
	value, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := value.(*rsa.PublicKey)
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

func decryptResource(apiKey, associatedData, nonce, ciphertext string) ([]byte, error) {
	if len(apiKey) != 32 {
		return nil, ErrProviderNotConfigured
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
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
	return gcm.Open(nil, []byte(nonce), raw, []byte(associatedData))
}

func randomNonce() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", raw[:]), nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func truncateProviderText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
