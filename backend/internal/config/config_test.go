package config

import "testing"

func validConfig() Config {
	return Config{
		AppEnv: "prod", JWTSecret: "production-secret-that-is-longer-than-thirty-two-characters",
		EnableLegacyTakeout: true, EnableDelivery: true,
		WechatAppID: "wx-app", WechatAppSecret: "wx-secret",
		TencentIMSDKAppID: "1400000000", TencentIMSecret: "im-secret", TencentIMAdmin: "administrator",
		MeiliHost: "http://meilisearch:7700", MeiliMasterKey: "meili-secret-that-is-long-enough", MeiliIndex: "products",
		COSSecretID: "cos-id", COSSecretKey: "cos-key", COSBucket: "campus-1234567890", COSRegion: "ap-guangzhou", COSAppID: "1234567890",
		MediaPublicBaseURL: "https://api.example.com", WechatCallbackToken: "wechat-callback-token-strong",
		WechatPayMchID: "1900000001", WechatPayAPIv3Key: "pay-api-v3-key-32-bytes-long",
		WechatPayMerchantSerialNo: "PAY-SERIAL-NO", WechatPayPrivateKeyPath: "testdata/apiclient_key.pem",
		WechatPayNotifyURL: "https://api.example.com/api/v1/takeout/payments/wechat/notify",
	}
}

func TestProductionConfigurationRejectsUnsafeAuthentication(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "default jwt", mutate: func(c *Config) { c.JWTSecret = "dev-secret-change-me" }},
		{name: "short jwt", mutate: func(c *Config) { c.JWTSecret = "short" }},
		{name: "missing wechat secret", mutate: func(c *Config) { c.WechatAppSecret = "" }},
		{name: "mock enabled", mutate: func(c *Config) { c.AllowMockWechat = true }},
		{name: "missing im", mutate: func(c *Config) { c.TencentIMSecret = "" }},
		{name: "missing meili", mutate: func(c *Config) { c.MeiliHost = "" }},
		{name: "missing cos", mutate: func(c *Config) { c.COSBucket = "" }},
		{name: "unsafe media base", mutate: func(c *Config) { c.MediaPublicBaseURL = "http://api.example.com" }},
		{name: "short callback token", mutate: func(c *Config) { c.WechatCallbackToken = "short" }},
		{name: "mock payment enabled", mutate: func(c *Config) { c.AllowMockPayment = true }},
		{name: "missing payment credential", mutate: func(c *Config) { c.WechatPayMchID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("unsafe production configuration was accepted")
			}
		})
	}
}

func TestLocalMockPaymentIsolation(t *testing.T) {
	cfg := Config{
		AppEnv: "dev", JWTSecret: "local-secret-that-is-longer-than-thirty-two-characters",
		WechatAppID: "", WechatAppSecret: "", AllowMockWechat: true, AllowMockPayment: true,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local mock payment should be allowed without real pay credentials: %v", err)
	}

	cfg.WechatPayMchID = "1900000001"
	if err := cfg.Validate(); err == nil {
		t.Fatal("mock payment with partial real credentials was accepted")
	}
}

func TestProductionConfigurationAcceptsExplicitCredentials(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyTakeoutDisabledDoesNotRequireWechatPayCredentials(t *testing.T) {
	cfg := validConfig()
	cfg.EnableLegacyTakeout = false
	cfg.WechatPayMchID = ""
	cfg.WechatPayAPIv3Key = ""
	cfg.WechatPayMerchantSerialNo = ""
	cfg.WechatPayPrivateKeyPath = ""
	cfg.WechatPayNotifyURL = ""
	cfg.WechatPayPlatformKeyPath = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled legacy takeout must not require legacy payment credentials: %v", err)
	}
}

func TestLegacyTakeoutEnabledStillRequiresWechatPayCredentials(t *testing.T) {
	cfg := validConfig()
	cfg.WechatPayMchID = ""
	cfg.WechatPayAPIv3Key = ""
	cfg.WechatPayMerchantSerialNo = ""
	cfg.WechatPayPrivateKeyPath = ""
	cfg.WechatPayNotifyURL = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("enabled legacy takeout accepted missing production payment credentials")
	}
}

func TestProductionMockPaymentRemainsForbiddenWhenLegacyTakeoutDisabled(t *testing.T) {
	cfg := validConfig()
	cfg.EnableLegacyTakeout = false
	cfg.AllowMockPayment = true
	cfg.WechatPayMchID = ""
	cfg.WechatPayAPIv3Key = ""
	cfg.WechatPayMerchantSerialNo = ""
	cfg.WechatPayPrivateKeyPath = ""
	cfg.WechatPayNotifyURL = ""
	cfg.WechatPayPlatformKeyPath = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("production mock payment was accepted while legacy takeout was disabled")
	}
}

func TestFeatureGateDefaults(t *testing.T) {
	t.Setenv("ENABLE_LEGACY_TAKEOUT", "")
	t.Setenv("ENABLE_DELIVERY", "")
	if envBool("ENABLE_LEGACY_TAKEOUT", false) {
		t.Fatal("legacy takeout default must be fail-closed")
	}
	if !envBool("ENABLE_DELIVERY", true) {
		t.Fatal("delivery placeholder default must be enabled")
	}
}
