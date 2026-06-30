package deliverypayment

import (
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
	"net/http/httptest"
	"os"
	"path/filepath"
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
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return now }
	plain := []byte(`{"out_trade_no":"D3TEST","transaction_id":"WXTX1","trade_state":"SUCCESS","success_time":"2027-01-15T08:00:00+08:00","amount":{"total":150}}`)
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

func encryptedNotificationBody(t *testing.T, apiKey string, plain []byte) []byte {
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
		"resource": map[string]string{
			"associated_data": associated,
			"nonce":           string(nonce),
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
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
