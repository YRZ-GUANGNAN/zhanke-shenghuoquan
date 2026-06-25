package takeoutpayment

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/takeout"
)

func TestProviderSelectionKeepsMockPaymentIsolated(t *testing.T) {
	provider, err := NewProviderFromConfig(config.Config{AppEnv: "dev", AllowMockPayment: true})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Provider() != takeout.PaymentProviderMock {
		t.Fatalf("provider=%s want mock", provider.Provider())
	}

	provider, err = NewProviderFromConfig(config.Config{AppEnv: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Provider() != takeout.PaymentProviderWechat {
		t.Fatalf("unconfigured provider mode=%s", provider.Provider())
	}
	if _, err := provider.CreatePrepay(nil, PrepayRequest{}); err == nil {
		t.Fatal("unconfigured provider accepted prepay")
	}
}

func TestWechatProviderBuildsJSAPIParameters(t *testing.T) {
	privateKeyPath := writeTestPrivateKey(t)
	provider, err := NewWechatProvider(WechatProviderConfig{
		AppID: "wx-test", MchID: "1900000001", APIv3Key: "12345678901234567890123456789012",
		MerchantSerialNo: "serial", PrivateKeyPath: privateKeyPath, NotifyURL: "https://api.example.com/notify",
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
		params.Package != "prepay_id=wx-prepay-id" || params.SignType != "RSA" || params.PaySign == "" {
		t.Fatalf("unexpected params=%+v", params)
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "apiclient_key.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
