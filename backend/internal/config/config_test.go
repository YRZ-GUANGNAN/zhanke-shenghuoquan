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
		{name: "mock delivery fee enabled", mutate: func(c *Config) { c.AllowMockDeliveryFeeConfirmation = true }},
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
	t.Setenv("ENABLE_DELIVERY_PAYMENT", "")
	t.Setenv("ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION", "")
	t.Setenv("ALLOW_MOCK_DELIVERY_PAYMENT", "")
	if envBool("ENABLE_LEGACY_TAKEOUT", false) {
		t.Fatal("legacy takeout default must be fail-closed")
	}
	if !envBool("ENABLE_DELIVERY", true) {
		t.Fatal("delivery placeholder default must be enabled")
	}
	if envBool("ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION", false) {
		t.Fatal("mock delivery fee confirmation must be fail-closed")
	}
	if envBool("ENABLE_DELIVERY_PAYMENT", false) {
		t.Fatal("delivery payment gate must be fail-closed")
	}
	if envBool("ALLOW_MOCK_DELIVERY_PAYMENT", false) {
		t.Fatal("mock delivery payment must be fail-closed")
	}
}

func TestLocalMockDeliveryFeeConfirmationIsIndependent(t *testing.T) {
	cfg := Config{
		AppEnv: "dev", JWTSecret: "local-secret-that-is-longer-than-thirty-two-characters",
		AllowMockWechat: true, EnableDelivery: true, AllowMockDeliveryFeeConfirmation: true,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local delivery mock bridge should be allowed: %v", err)
	}
	if cfg.AllowMockPayment {
		t.Fatal("delivery mock bridge unexpectedly enabled legacy mock payment")
	}
	cfg.AppEnv = "prod"
	cfg.WechatAppID = "wx-app"
	cfg.WechatAppSecret = "wx-secret"
	cfg.TencentIMSDKAppID = "1400000000"
	cfg.TencentIMSecret = "im-secret"
	cfg.TencentIMAdmin = "administrator"
	cfg.MeiliHost = "http://meili:7700"
	cfg.MeiliMasterKey = "meili-secret-that-is-long-enough"
	cfg.COSSecretID = "cos-id"
	cfg.COSSecretKey = "cos-key"
	cfg.COSBucket = "bucket"
	cfg.COSRegion = "ap-guangzhou"
	cfg.COSAppID = "123"
	cfg.MediaPublicBaseURL = "https://api.example.com"
	cfg.WechatCallbackToken = "callback-token-long-enough"
	if err := cfg.Validate(); err == nil {
		t.Fatal("production accepted mock delivery fee confirmation")
	}
}

func TestDeliveryPaymentGateAndMockIsolation(t *testing.T) {
	local := Config{
		AppEnv: "dev", JWTSecret: "local-secret-that-is-longer-than-thirty-two-characters",
		AllowMockWechat: true, EnableDelivery: true, EnableDeliveryPayment: true,
		AllowMockDeliveryPayment: true,
	}
	if err := local.Validate(); err != nil {
		t.Fatalf("isolated local delivery payment mock should be allowed: %v", err)
	}

	bothMocks := local
	bothMocks.AllowMockDeliveryFeeConfirmation = true
	if err := bothMocks.Validate(); err == nil {
		t.Fatal("D-2 fee confirmation mock and D-3 payment mock were enabled together")
	}

	mockWithReal := local
	mockWithReal.WechatPayMchID = "1900000001"
	if err := mockWithReal.Validate(); err == nil {
		t.Fatal("delivery payment mock accepted a partial real credential")
	}

	partialReal := local
	partialReal.AllowMockDeliveryPayment = false
	partialReal.WechatPayMchID = "1900000001"
	partialReal.WechatPayAPIv3Key = "12345678901234567890123456789012"
	partialReal.WechatPayMerchantSerialNo = "serial"
	partialReal.WechatPayPrivateKeyPath = "merchant.pem"
	partialReal.DeliveryWechatPayNotifyURL = "https://api.example.com/payment-notify"
	if err := partialReal.Validate(); err == nil {
		t.Fatal("enabled delivery payment accepted a partial real configuration")
	}

	productionMock := validConfig()
	productionMock.EnableDeliveryPayment = true
	productionMock.AllowMockDeliveryPayment = true
	productionMock.DeliveryWechatPayNotifyURL = ""
	productionMock.DeliveryWechatRefundNotifyURL = ""
	if err := productionMock.Validate(); err == nil {
		t.Fatal("production accepted delivery payment mock")
	}
}

func TestDeliveryPaymentProductionCredentialGate(t *testing.T) {
	disabled := validConfig()
	disabled.EnableLegacyTakeout = false
	disabled.EnableDeliveryPayment = false
	disabled.WechatPayMchID = ""
	disabled.WechatPayAPIv3Key = ""
	disabled.WechatPayMerchantSerialNo = ""
	disabled.WechatPayPrivateKeyPath = ""
	disabled.WechatPayPlatformKeyPath = ""
	disabled.WechatPayNotifyURL = ""
	disabled.DeliveryWechatPayNotifyURL = ""
	disabled.DeliveryWechatRefundNotifyURL = ""
	if err := disabled.Validate(); err != nil {
		t.Fatalf("closed delivery payment gate blocked production startup: %v", err)
	}

	partial := validConfig()
	partial.EnableLegacyTakeout = false
	partial.EnableDeliveryPayment = true
	partial.WechatPayNotifyURL = ""
	partial.DeliveryWechatPayNotifyURL = "https://api.example.com/api/v1/delivery/payments/wechat/notify"
	partial.DeliveryWechatRefundNotifyURL = ""
	if err := partial.Validate(); err == nil {
		t.Fatal("enabled delivery payment accepted partial callback configuration")
	}

	complete := validConfig()
	complete.EnableDeliveryPayment = true
	complete.WechatPayPlatformKeyPath = "testdata/wechatpay_platform.pem"
	complete.DeliveryWechatPayNotifyURL = "https://api.example.com/api/v1/delivery/payments/wechat/notify"
	complete.DeliveryWechatRefundNotifyURL = "https://api.example.com/api/v1/delivery/payments/wechat/refund-notify"
	if err := complete.Validate(); err != nil {
		t.Fatalf("complete production delivery payment configuration was rejected: %v", err)
	}
}
