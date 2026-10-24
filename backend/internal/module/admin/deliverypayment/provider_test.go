package deliverypayment

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
)

func TestProviderSelectionKeepsDeliveryMockIsolated(t *testing.T) {
	provider, err := NewProviderFromConfig(config.Config{
		AppEnv: "dev", EnableDeliveryPayment: true, AllowMockDeliveryPayment: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Provider() != delivery.PaymentMock {
		t.Fatalf("provider=%s want mock", provider.Provider())
	}

	provider, err = NewProviderFromConfig(config.Config{AppEnv: "dev", EnableDeliveryPayment: true})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Provider() != delivery.PaymentWechat {
		t.Fatalf("unconfigured provider mode=%s", provider.Provider())
	}
	if _, err := provider.CreatePrepay(nil, PrepayRequest{}); !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("unconfigured provider returned %v", err)
	}
}

func TestWechatProviderBuildsSignedJSAPIParameters(t *testing.T) {
	privatePath, publicPath, _ := writeProviderKeys(t)
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001",
		APIv3Key:         "12345678901234567890123456789012",
		MerchantSerialNo: "serial", PrivateKeyPath: privatePath,
		PaymentNotifyURL:      "https://api.example.com/payment-notify",
		RefundNotifyURL:       "https://api.example.com/refund-notify",
		PlatformPublicKeyPath: publicPath,
		PlatformKeyID:         "platform-key-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	provider.nonce = func() (string, error) { return "nonce-test", nil }
	params, err := provider.ClientParams("wx-prepay-id")
	if err != nil {
		t.Fatal(err)
	}
	if params.TimeStamp != "1800000000" || params.NonceStr != "nonce-test" ||
		params.Package != "prepay_id=wx-prepay-id" || params.SignType != "RSA" ||
		params.PaySign == "" {
		t.Fatalf("unexpected JSAPI parameters: %+v", params)
	}
}

func TestWechatProviderVerifiesAndDecryptsPaymentNotification(t *testing.T) {
	privatePath, publicPath, signingKey := writeProviderKeys(t)
	const apiKey = "12345678901234567890123456789012"
	now := time.Unix(1_800_000_000, 0).UTC()
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001", APIv3Key: apiKey,
		MerchantSerialNo: "serial", PrivateKeyPath: privatePath,
		PaymentNotifyURL:      "https://api.example.com/payment-notify",
		RefundNotifyURL:       "https://api.example.com/refund-notify",
		PlatformPublicKeyPath: publicPath,
		PlatformKeyID:         "platform-key-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return now }
	plain := []byte(`{"appid":"wx-test","mchid":"1900000001","out_trade_no":"D3TEST","transaction_id":"WXTX1","trade_type":"JSAPI","trade_state":"SUCCESS","success_time":"2027-01-15T08:00:00+08:00","amount":{"total":150,"currency":"CNY"}}`)
	body := encryptedNotificationBody(t, apiKey, plain)
	request := httptest.NewRequest("POST", "/notify", nil)
	timestamp := fmt.Sprintf("%d", now.Unix())
	nonce := "callback-nonce"
	signature, err := signRSA(signingKey, []byte(timestamp+"\n"+nonce+"\n"+string(body)+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Wechatpay-Timestamp", timestamp)
	request.Header.Set("Wechatpay-Nonce", nonce)
	request.Header.Set("Wechatpay-Signature", signature)
	request.Header.Set("Wechatpay-Serial", "platform-key-id")

	result, err := provider.ParsePaymentNotification(request, body)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutTradeNo != "D3TEST" || result.WechatTransactionID != "WXTX1" ||
		result.AmountCents != 150 {
		t.Fatalf("notification=%+v", result)
	}

	request.Header.Set("Wechatpay-Timestamp", fmt.Sprintf("%d", now.Add(-6*time.Minute).Unix()))
	if _, err := provider.ParsePaymentNotification(request, body); !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("stale callback returned %v", err)
	}
}

func TestWechatProviderRejectsUntrustedPaymentNotifications(t *testing.T) {
	privatePath, publicPath, signingKey := writeProviderKeys(t)
	const apiKey = "12345678901234567890123456789012"
	now := time.Unix(1_800_000_000, 0).UTC()
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001", APIv3Key: apiKey,
		MerchantSerialNo: "merchant-serial", PrivateKeyPath: privatePath,
		PaymentNotifyURL: "https://api.example.com/payment-notify", RefundNotifyURL: "https://api.example.com/refund-notify",
		PlatformPublicKeyPath: publicPath, PlatformKeyID: "trusted-platform-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return now }

	valid := `{"appid":"wx-test","mchid":"1900000001","out_trade_no":"D3TEST","transaction_id":"WXTX1","trade_type":"JSAPI","trade_state":"SUCCESS","success_time":"2027-01-15T08:00:00+08:00","amount":{"total":150,"currency":"CNY"}}`
	tests := []struct {
		name, plain, eventType, algorithm, serial string
	}{
		{name: "wrong serial", plain: valid, eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "unknown-key"},
		{name: "wrong appid", plain: strings.Replace(valid, `"wx-test"`, `"wx-other"`, 1), eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "wrong merchant", plain: strings.Replace(valid, `"1900000001"`, `"1900000002"`, 1), eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "wrong currency", plain: strings.Replace(valid, `"CNY"`, `"USD"`, 1), eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "wrong trade type", plain: strings.Replace(valid, `"JSAPI"`, `"NATIVE"`, 1), eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "invalid success time", plain: strings.Replace(valid, `"2027-01-15T08:00:00+08:00"`, `"not-a-time"`, 1), eventType: "TRANSACTION.SUCCESS", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "wrong event", plain: valid, eventType: "TRANSACTION.CLOSED", algorithm: "AEAD_AES_256_GCM", serial: "trusted-platform-key"},
		{name: "wrong algorithm", plain: valid, eventType: "TRANSACTION.SUCCESS", algorithm: "UNSUPPORTED", serial: "trusted-platform-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := encryptedNotificationEnvelope(t, apiKey, []byte(tt.plain), tt.eventType, "transaction", tt.algorithm)
			request := signedWechatRequest(t, body, signingKey, now, tt.serial)
			if _, err := provider.ParsePaymentNotification(request, body); !errors.Is(err, ErrInvalidNotification) {
				t.Fatalf("untrusted callback returned %v", err)
			}
		})
	}
}

func TestWechatProviderRequiresSignedSuccessfulResponses(t *testing.T) {
	privatePath, publicPath, signingKey := writeProviderKeys(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	responseBody := []byte(`{"prepay_id":"wx-prepay-id"}`)
	signed := false
	responseSerial := "trusted-platform-key"
	signedBody := responseBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if signed {
			setWechatSignatureHeaders(t, w.Header(), signedBody, signingKey, now, responseSerial)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001", APIv3Key: "12345678901234567890123456789012",
		MerchantSerialNo: "merchant-serial", PrivateKeyPath: privatePath,
		PaymentNotifyURL: "https://api.example.com/payment-notify", RefundNotifyURL: "https://api.example.com/refund-notify",
		APIBase: server.URL, PlatformPublicKeyPath: publicPath, PlatformKeyID: "trusted-platform-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return now }
	provider.httpClient = server.Client()
	request := PrepayRequest{Description: "配送服务费", OutTradeNo: "D3TEST", AmountCents: 150, OpenID: "openid"}
	if _, err := provider.CreatePrepay(context.Background(), request); !errors.Is(err, ErrUntrustedProviderResponse) {
		t.Fatalf("unsigned 200 response returned %v", err)
	}
	signed = true
	responseSerial = "wrong-platform-key"
	if _, err := provider.CreatePrepay(context.Background(), request); !errors.Is(err, ErrUntrustedProviderResponse) {
		t.Fatalf("response signed by an unknown key ID returned %v", err)
	}
	responseSerial = "trusted-platform-key"
	signedBody = []byte(`{"prepay_id":"different"}`)
	if _, err := provider.CreatePrepay(context.Background(), request); !errors.Is(err, ErrUntrustedProviderResponse) {
		t.Fatalf("tampered signed response returned %v", err)
	}
	signedBody = responseBody
	result, err := provider.CreatePrepay(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.PrepayID != "wx-prepay-id" {
		t.Fatalf("prepay=%+v", result)
	}
}

func TestWechatProviderValidatesRefundResponseAndNotification(t *testing.T) {
	privatePath, publicPath, signingKey := writeProviderKeys(t)
	const apiKey = "12345678901234567890123456789012"
	now := time.Unix(1_800_000_000, 0).UTC()
	responseBody := []byte(`{"refund_id":"WXREFUND1","out_trade_no":"D3TEST","out_refund_no":"D3RTEST","status":"PROCESSING","amount":{"refund":150,"total":150,"currency":"CNY"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		setWechatSignatureHeaders(t, w.Header(), responseBody, signingKey, now, "trusted-platform-key")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001", APIv3Key: apiKey,
		MerchantSerialNo: "merchant-serial", PrivateKeyPath: privatePath,
		PaymentNotifyURL: "https://api.example.com/payment-notify", RefundNotifyURL: "https://api.example.com/refund-notify",
		APIBase: server.URL, PlatformPublicKeyPath: publicPath, PlatformKeyID: "trusted-platform-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return now }
	provider.httpClient = server.Client()
	refundRequest := RefundRequest{OutTradeNo: "D3TEST", OutRefundNo: "D3RTEST", AmountCents: 150, Reason: "配送取消"}
	refund, err := provider.CreateRefund(context.Background(), refundRequest)
	if err != nil {
		t.Fatal(err)
	}
	if refund.WechatRefundID != "WXREFUND1" || refund.Status != "PROCESSING" {
		t.Fatalf("refund=%+v", refund)
	}

	responseBody = []byte(`{"refund_id":"WXREFUND1","out_trade_no":"D3OTHER","out_refund_no":"D3RTEST","status":"PROCESSING","amount":{"refund":150,"total":150,"currency":"CNY"}}`)
	if _, err := provider.CreateRefund(context.Background(), refundRequest); !errors.Is(err, ErrProviderRejected) {
		t.Fatalf("mismatched signed refund response returned %v", err)
	}

	plain := []byte(`{"mchid":"1900000001","out_trade_no":"D3TEST","transaction_id":"WXTX1","out_refund_no":"D3RTEST","refund_id":"WXREFUND1","refund_status":"SUCCESS","amount":{"refund":150,"total":150,"currency":"CNY"}}`)
	body := encryptedNotificationEnvelope(t, apiKey, plain, "REFUND.SUCCESS", "refund", "AEAD_AES_256_GCM")
	request := signedWechatRequest(t, body, signingKey, now, "trusted-platform-key")
	notification, err := provider.ParseRefundNotification(request, body)
	if err != nil {
		t.Fatal(err)
	}
	if notification.OutTradeNo != "D3TEST" || notification.OutRefundNo != "D3RTEST" ||
		notification.WechatRefundID != "WXREFUND1" || notification.AmountCents != 150 || !notification.Success {
		t.Fatalf("refund notification=%+v", notification)
	}
}

func TestDecryptResourceRejectsMalformedNonceWithoutPanic(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("malformed nonce panicked: %v", recovered)
		}
	}()
	if _, err := decryptResource("12345678901234567890123456789012", "delivery-d3", "short", base64.StdEncoding.EncodeToString(make([]byte, 32))); err == nil {
		t.Fatal("malformed nonce was accepted")
	}
}

func encryptedNotificationBody(t *testing.T, apiKey string, plain []byte) []byte {
	return encryptedNotificationEnvelope(t, apiKey, plain, "TRANSACTION.SUCCESS", "transaction", "AEAD_AES_256_GCM")
}

func encryptedNotificationEnvelope(t *testing.T, apiKey string, plain []byte, eventType, originalType, algorithm string) []byte {
	t.Helper()
	block, err := aes.NewCipher([]byte(apiKey))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("123456789012")
	associated := "delivery-d3"
	ciphertext := gcm.Seal(nil, nonce, plain, []byte(associated))
	body, err := json.Marshal(map[string]any{
		"event_type": eventType, "resource_type": "encrypt-resource",
		"resource": map[string]string{
			"associated_data": associated,
			"nonce":           string(nonce),
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"algorithm":       algorithm,
			"original_type":   originalType,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func signedWechatRequest(t *testing.T, body []byte, key *rsa.PrivateKey, now time.Time, serial string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/notify", nil)
	setWechatSignatureHeaders(t, request.Header, body, key, now, serial)
	return request
}

func setWechatSignatureHeaders(t *testing.T, header http.Header, body []byte, key *rsa.PrivateKey, now time.Time, serial string) {
	t.Helper()
	timestamp := fmt.Sprintf("%d", now.Unix())
	nonce := "wechatpay-response-nonce"
	signature, err := signRSA(key, []byte(timestamp+"\n"+nonce+"\n"+string(body)+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	header.Set("Wechatpay-Timestamp", timestamp)
	header.Set("Wechatpay-Nonce", nonce)
	header.Set("Wechatpay-Signature", signature)
	header.Set("Wechatpay-Serial", serial)
}

func writeProviderKeys(t *testing.T) (string, string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateRaw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicRaw, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(t.TempDir(), "merchant.pem")
	publicPath := filepath.Join(t.TempDir(), "platform.pem")
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateRaw}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicRaw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return privatePath, publicPath, key
}
