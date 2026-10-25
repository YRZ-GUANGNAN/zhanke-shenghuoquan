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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
)

var (
	ErrProviderNotConfigured     = errors.New("delivery payment provider is not configured")
	ErrProviderRejected          = errors.New("delivery payment provider rejected request")
	ErrUntrustedProviderResponse = errors.New("untrusted delivery payment provider response")
	ErrInvalidNotification       = errors.New("invalid delivery payment notification")
)

const maxProviderBodyBytes = 1 << 20

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
	PayerOpenID         string
	AmountCents         uint64
	PaidAt              time.Time
}

type PaymentQueryRequest struct {
	OutTradeNo  string
	AmountCents uint64
	PayerOpenID string
}

type PaymentQueryResult struct {
	OutTradeNo          string
	WechatTransactionID string
	PayerOpenID         string
	State               string
	AmountCents         uint64
	PaidAt              *time.Time
	RawBody             []byte
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

type RefundQueryRequest struct {
	OutRefundNo string
	OutTradeNo  string
	AmountCents uint64
}

type RefundQueryResult struct {
	OutTradeNo     string
	OutRefundNo    string
	WechatRefundID string
	State          string
	AmountCents    uint64
	TotalCents     uint64
	SuccessAt      *time.Time
}

type RefundNotificationResult struct {
	OutTradeNo     string
	OutRefundNo    string
	WechatRefundID string
	AmountCents    uint64
	Status         string
	Success        bool
	Reason         string
}

type Provider interface {
	Provider() delivery.PaymentProvider
	CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error)
	ClientParams(string) (*JSAPIParameter, error)
	ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error)
	QueryPayment(context.Context, PaymentQueryRequest) (PaymentQueryResult, error)
	ClosePayment(context.Context, string) error
	CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error)
	ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error)
	QueryRefund(context.Context, RefundQueryRequest) (RefundQueryResult, error)
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
			PlatformKeyID: cfg.WechatPayPlatformKeyID,
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
func (MockProvider) QueryPayment(context.Context, PaymentQueryRequest) (PaymentQueryResult, error) {
	return PaymentQueryResult{}, ErrProviderNotConfigured
}
func (MockProvider) ClosePayment(context.Context, string) error { return ErrProviderNotConfigured }
func (MockProvider) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, ErrProviderNotConfigured
}
func (MockProvider) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, ErrProviderNotConfigured
}
func (MockProvider) QueryRefund(context.Context, RefundQueryRequest) (RefundQueryResult, error) {
	return RefundQueryResult{}, ErrProviderNotConfigured
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
func (UnconfiguredProvider) QueryPayment(context.Context, PaymentQueryRequest) (PaymentQueryResult, error) {
	return PaymentQueryResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ClosePayment(context.Context, string) error {
	return ErrProviderNotConfigured
}
func (UnconfiguredProvider) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) QueryRefund(context.Context, RefundQueryRequest) (RefundQueryResult, error) {
	return RefundQueryResult{}, ErrProviderNotConfigured
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
	PlatformKeyID         string
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
	cfg.PlatformKeyID = strings.TrimSpace(cfg.PlatformKeyID)
	if cfg.PlatformKeyID == "" {
		return nil, ErrProviderNotConfigured
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
		httpClient: &http.Client{
			Timeout:       8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}, nonce: randomNonce, now: time.Now,
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

func (p *WechatProvider) QueryPayment(ctx context.Context, input PaymentQueryRequest) (PaymentQueryResult, error) {
	input.OutTradeNo = strings.TrimSpace(input.OutTradeNo)
	input.PayerOpenID = strings.TrimSpace(input.PayerOpenID)
	if input.OutTradeNo == "" || input.AmountCents == 0 || input.PayerOpenID == "" ||
		p.config.AppID == "" || p.config.MchID == "" {
		return PaymentQueryResult{}, ErrProviderNotConfigured
	}
	var response struct {
		AppID               string `json:"appid"`
		MchID               string `json:"mchid"`
		OutTradeNo          string `json:"out_trade_no"`
		WechatTransactionID string `json:"transaction_id"`
		TradeType           string `json:"trade_type"`
		TradeState          string `json:"trade_state"`
		SuccessTime         string `json:"success_time"`
		Payer               struct {
			OpenID string `json:"openid"`
		} `json:"payer"`
		Amount struct {
			Total    uint64 `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(input.OutTradeNo) +
		"?mchid=" + url.QueryEscape(p.config.MchID)
	raw, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return PaymentQueryResult{}, err
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return PaymentQueryResult{}, ErrUntrustedProviderResponse
	}
	if response.AppID != p.config.AppID || response.MchID != p.config.MchID ||
		response.OutTradeNo != input.OutTradeNo || !supportedPaymentState(response.TradeState) ||
		(response.TradeType != "" && response.TradeType != "JSAPI") ||
		(response.Amount.Total != 0 && response.Amount.Total != input.AmountCents) ||
		(response.Amount.Currency != "" && response.Amount.Currency != "CNY") ||
		(response.Payer.OpenID != "" && response.Payer.OpenID != input.PayerOpenID) {
		return PaymentQueryResult{}, ErrUntrustedProviderResponse
	}
	result := PaymentQueryResult{
		OutTradeNo: response.OutTradeNo, WechatTransactionID: response.WechatTransactionID,
		PayerOpenID: response.Payer.OpenID, State: response.TradeState,
		AmountCents: response.Amount.Total, RawBody: append([]byte(nil), raw...),
	}
	if response.TradeState == "SUCCESS" {
		paidAt, err := time.Parse(time.RFC3339, response.SuccessTime)
		if err != nil || response.WechatTransactionID == "" || response.TradeType != "JSAPI" ||
			response.Payer.OpenID != input.PayerOpenID || response.Amount.Total != input.AmountCents ||
			response.Amount.Currency != "CNY" {
			return PaymentQueryResult{}, ErrUntrustedProviderResponse
		}
		paidAt = paidAt.UTC()
		result.PaidAt = &paidAt
	}
	return result, nil
}

func (p *WechatProvider) ClosePayment(ctx context.Context, outTradeNo string) error {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" || p.config.MchID == "" {
		return ErrProviderNotConfigured
	}
	body, err := json.Marshal(map[string]string{"mchid": p.config.MchID})
	if err != nil {
		return err
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(outTradeNo) + "/close"
	raw, err := p.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) != 0 {
		return ErrUntrustedProviderResponse
	}
	return nil
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
	plain, err := p.verifyAndDecrypt(req, body, map[string]struct{}{"TRANSACTION.SUCCESS": {}}, "transaction")
	if err != nil {
		return PaymentNotificationResult{}, err
	}
	var transaction struct {
		AppID         string `json:"appid"`
		MchID         string `json:"mchid"`
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeType     string `json:"trade_type"`
		TradeState    string `json:"trade_state"`
		SuccessTime   string `json:"success_time"`
		Payer         struct {
			OpenID string `json:"openid"`
		} `json:"payer"`
		Amount struct {
			Total    uint64 `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &transaction); err != nil {
		return PaymentNotificationResult{}, ErrInvalidNotification
	}
	if transaction.AppID != p.config.AppID || transaction.MchID != p.config.MchID ||
		transaction.TradeType != "JSAPI" || transaction.TradeState != "SUCCESS" ||
		transaction.OutTradeNo == "" || transaction.TransactionID == "" ||
		transaction.Payer.OpenID == "" || transaction.Amount.Total == 0 || transaction.Amount.Currency != "CNY" {
		return PaymentNotificationResult{}, ErrInvalidNotification
	}
	paidAt, err := time.Parse(time.RFC3339, transaction.SuccessTime)
	if err != nil || paidAt.IsZero() {
		return PaymentNotificationResult{}, ErrInvalidNotification
	}
	return PaymentNotificationResult{
		OutTradeNo: transaction.OutTradeNo, WechatTransactionID: transaction.TransactionID,
		PayerOpenID: transaction.Payer.OpenID, AmountCents: transaction.Amount.Total, PaidAt: paidAt.UTC(),
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
		RefundID    string `json:"refund_id"`
		OutTradeNo  string `json:"out_trade_no"`
		OutRefundNo string `json:"out_refund_no"`
		Status      string `json:"status"`
		Code        string `json:"code"`
		Message     string `json:"message"`
		Amount      struct {
			Refund   uint64 `json:"refund"`
			Total    uint64 `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "/v3/refund/domestic/refunds", body, &response); err != nil {
		return RefundProviderResult{}, err
	}
	if response.RefundID == "" || response.OutTradeNo != input.OutTradeNo ||
		response.OutRefundNo != input.OutRefundNo || response.Amount.Refund != input.AmountCents ||
		response.Amount.Total != input.AmountCents || response.Amount.Currency != "CNY" ||
		!supportedRefundStatus(response.Status) {
		return RefundProviderResult{}, fmt.Errorf("%w: code=%s message=%s", ErrProviderRejected, response.Code, response.Message)
	}
	return RefundProviderResult{WechatRefundID: response.RefundID, Status: response.Status}, nil
}

func (p *WechatProvider) QueryRefund(ctx context.Context, input RefundQueryRequest) (RefundQueryResult, error) {
	input.OutRefundNo = strings.TrimSpace(input.OutRefundNo)
	input.OutTradeNo = strings.TrimSpace(input.OutTradeNo)
	if input.OutRefundNo == "" || input.OutTradeNo == "" || input.AmountCents == 0 {
		return RefundQueryResult{}, ErrProviderNotConfigured
	}
	var response struct {
		RefundID    string `json:"refund_id"`
		OutTradeNo  string `json:"out_trade_no"`
		OutRefundNo string `json:"out_refund_no"`
		Status      string `json:"status"`
		SuccessTime string `json:"success_time"`
		Amount      struct {
			Refund   uint64 `json:"refund"`
			Total    uint64 `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	path := "/v3/refund/domestic/refunds/" + url.PathEscape(input.OutRefundNo)
	if err := p.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RefundQueryResult{}, err
	}
	if response.RefundID == "" || response.OutTradeNo != input.OutTradeNo ||
		response.OutRefundNo != input.OutRefundNo || !supportedRefundStatus(response.Status) ||
		response.Amount.Refund != input.AmountCents || response.Amount.Total != input.AmountCents ||
		response.Amount.Currency != "CNY" {
		return RefundQueryResult{}, ErrUntrustedProviderResponse
	}
	result := RefundQueryResult{
		OutTradeNo: response.OutTradeNo, OutRefundNo: response.OutRefundNo,
		WechatRefundID: response.RefundID, State: response.Status,
		AmountCents: response.Amount.Refund, TotalCents: response.Amount.Total,
	}
	if response.Status == "SUCCESS" {
		successAt, err := time.Parse(time.RFC3339, response.SuccessTime)
		if err != nil {
			return RefundQueryResult{}, ErrUntrustedProviderResponse
		}
		successAt = successAt.UTC()
		result.SuccessAt = &successAt
	}
	return result, nil
}

func (p *WechatProvider) ParseRefundNotification(req *http.Request, body []byte) (RefundNotificationResult, error) {
	plain, err := p.verifyAndDecrypt(req, body, map[string]struct{}{
		"REFUND.SUCCESS": {}, "REFUND.ABNORMAL": {}, "REFUND.CLOSED": {},
	}, "refund")
	if err != nil {
		return RefundNotificationResult{}, err
	}
	var value struct {
		MchID       string `json:"mchid"`
		OutTradeNo  string `json:"out_trade_no"`
		OutRefundNo string `json:"out_refund_no"`
		RefundID    string `json:"refund_id"`
		Status      string `json:"refund_status"`
		Amount      struct {
			Refund   uint64 `json:"refund"`
			Total    uint64 `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &value); err != nil {
		return RefundNotificationResult{}, ErrInvalidNotification
	}
	if value.MchID != p.config.MchID || value.OutTradeNo == "" || value.OutRefundNo == "" ||
		value.RefundID == "" || value.Status == "" || value.Amount.Refund == 0 ||
		value.Amount.Total == 0 || value.Amount.Refund > value.Amount.Total || value.Amount.Currency != "CNY" {
		return RefundNotificationResult{}, ErrInvalidNotification
	}
	success := value.Status == "SUCCESS"
	reason := ""
	if !success {
		reason = "wechat refund status: " + value.Status
	}
	return RefundNotificationResult{
		OutTradeNo: value.OutTradeNo, OutRefundNo: value.OutRefundNo,
		WechatRefundID: value.RefundID, AmountCents: value.Amount.Refund,
		Status: value.Status, Success: success, Reason: reason,
	}, nil
}

func (p *WechatProvider) verifyAndDecrypt(req *http.Request, body []byte, allowedEvents map[string]struct{}, originalType string) ([]byte, error) {
	if len(body) == 0 || len(body) > maxProviderBodyBytes || p.verifyWechatSignature(req.Header, body) != nil {
		return nil, ErrInvalidNotification
	}
	var envelope struct {
		EventType    string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		Resource     struct {
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
			Ciphertext     string `json:"ciphertext"`
			Algorithm      string `json:"algorithm"`
			OriginalType   string `json:"original_type"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, ErrInvalidNotification
	}
	if _, ok := allowedEvents[envelope.EventType]; !ok || envelope.ResourceType != "encrypt-resource" ||
		envelope.Resource.Algorithm != "AEAD_AES_256_GCM" || envelope.Resource.OriginalType != originalType {
		return nil, ErrInvalidNotification
	}
	plain, err := decryptResource(
		p.config.APIv3Key, envelope.Resource.AssociatedData,
		envelope.Resource.Nonce, envelope.Resource.Ciphertext,
	)
	if err != nil {
		return nil, ErrInvalidNotification
	}
	return plain, nil
}

func (p *WechatProvider) doJSON(ctx context.Context, method, path string, body []byte, target any) error {
	raw, err := p.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return ErrUntrustedProviderResponse
	}
	return nil
}

func (p *WechatProvider) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	endpoint := strings.TrimRight(p.config.APIBase, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := p.authorize(req, body); err != nil {
		return nil, err
	}
	response, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxProviderBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxProviderBodyBytes {
		return nil, ErrUntrustedProviderResponse
	}
	if err := p.verifyWechatSignature(response.Header, raw); err != nil {
		return nil, ErrUntrustedProviderResponse
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: http=%d", ErrProviderRejected, response.StatusCode)
	}
	return raw, nil
}

func (p *WechatProvider) verifyWechatSignature(header http.Header, body []byte) error {
	timestamp, ok := singleHeader(header, "Wechatpay-Timestamp")
	if !ok {
		return ErrInvalidNotification
	}
	nonce, ok := singleHeader(header, "Wechatpay-Nonce")
	if !ok {
		return ErrInvalidNotification
	}
	signature, ok := singleHeader(header, "Wechatpay-Signature")
	if !ok {
		return ErrInvalidNotification
	}
	serial, ok := singleHeader(header, "Wechatpay-Serial")
	if !ok || serial != p.config.PlatformKeyID {
		return ErrInvalidNotification
	}
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || absDuration(p.now().Sub(time.Unix(unixTime, 0))) > 5*time.Minute {
		return ErrInvalidNotification
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return ErrInvalidNotification
	}
	digest := sha256.Sum256([]byte(timestamp + "\n" + nonce + "\n" + string(body) + "\n"))
	if err := rsa.VerifyPKCS1v15(p.platformKey, crypto.SHA256, digest[:], rawSignature); err != nil {
		return ErrInvalidNotification
	}
	return nil
}

func singleHeader(header http.Header, name string) (string, bool) {
	values := header.Values(name)
	if len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	return value, value != ""
}

func supportedRefundStatus(value string) bool {
	switch value {
	case "SUCCESS", "CLOSED", "PROCESSING", "ABNORMAL":
		return true
	default:
		return false
	}
}

func supportedPaymentState(value string) bool {
	switch value {
	case "SUCCESS", "REFUND", "NOTPAY", "CLOSED", "REVOKED", "USERPAYING", "PAYERROR":
		return true
	default:
		return false
	}
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
	if len(nonce) != gcm.NonceSize() || len(raw) < gcm.Overhead() {
		return nil, ErrInvalidNotification
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
