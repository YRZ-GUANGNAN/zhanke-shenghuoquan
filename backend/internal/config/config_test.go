package config

import (
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{
		AppEnv: "prod", HTTPAddr: "127.0.0.1:8080", JWTSecret: "production-secret-that-is-longer-than-thirty-two-characters",
		MySQLDSN:            "campus-prod:strong-password@tcp(mysql:3306)/campus_circle?charset=utf8mb4&parseTime=True&loc=Local",
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

func TestRealWechatConfigurationDisablesMockFallback(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("WECHAT_APP_ID", "wx0dacef976daa8e55")
	t.Setenv("WECHAT_APP_SECRET", "test-secret-not-for-production")
	t.Setenv("ALLOW_MOCK_WECHAT_LOGIN", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowMockWechat {
		t.Fatal("real WeChat configuration unexpectedly enabled mock fallback")
	}
	if cfg.WechatAppID != "wx0dacef976daa8e55" {
		t.Fatal("real WeChat app id was not loaded")
	}
}

func TestRealWechatConfigurationDefaultsMockFallbackToDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("WECHAT_APP_ID", "wx0dacef976daa8e55")
	t.Setenv("WECHAT_APP_SECRET", "test-secret-not-for-production")
	t.Setenv("ALLOW_MOCK_WECHAT_LOGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowMockWechat {
		t.Fatal("mock WeChat login must default to disabled")
	}
}

func TestRealWechatConfigurationRejectsMockWithCredentialsInDev(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("WECHAT_APP_ID", "wx0dacef976daa8e55")
	t.Setenv("WECHAT_APP_SECRET", "test-secret-not-for-production")
	t.Setenv("ALLOW_MOCK_WECHAT_LOGIN", "true")

	if _, err := Load(); err == nil {
		t.Fatal("dev configuration accepted real WeChat credentials with mock login enabled")
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
	t.Setenv("ENABLE_DELIVERY_IM_READ_RECEIPT", "")
	legacy, err := envBool("ENABLE_LEGACY_TAKEOUT", false)
	if err != nil || legacy {
		t.Fatal("legacy takeout default must be fail-closed")
	}
	delivery, err := envBool("ENABLE_DELIVERY", true)
	if err != nil || !delivery {
		t.Fatal("delivery placeholder default must be enabled")
	}
	mockFee, err := envBool("ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION", false)
	if err != nil || mockFee {
		t.Fatal("mock delivery fee confirmation must be fail-closed")
	}
	deliveryPayment, err := envBool("ENABLE_DELIVERY_PAYMENT", false)
	if err != nil || deliveryPayment {
		t.Fatal("delivery payment gate must be fail-closed")
	}
	mockDeliveryPayment, err := envBool("ALLOW_MOCK_DELIVERY_PAYMENT", false)
	if err != nil || mockDeliveryPayment {
		t.Fatal("mock delivery payment must be fail-closed")
	}
	readReceipt, err := envBool("ENABLE_DELIVERY_IM_READ_RECEIPT", false)
	if err != nil || readReceipt {
		t.Fatal("delivery IM read receipt must be fail-closed")
	}
}

func TestBooleanEnvironmentRejectsAmbiguousValues(t *testing.T) {
	for _, value := range []string{"yes", "enabled", "TRUE ", "t"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ENABLE_DELIVERY", value)
			if _, err := envBool("ENABLE_DELIVERY", true); err == nil {
				t.Fatalf("ambiguous boolean %q was accepted", value)
			}
		})
	}

	for _, value := range []string{"true", "false", "1", "0"} {
		t.Run("accepted-"+value, func(t *testing.T) {
			t.Setenv("ENABLE_DELIVERY", value)
			if _, err := envBool("ENABLE_DELIVERY", false); err != nil {
				t.Fatalf("supported boolean %q was rejected: %v", value, err)
			}
		})
	}
}

func TestProductionLoadRequiresExplicitNetworkConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("MYSQL_DSN", "")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "HTTP_ADDR") {
		t.Fatalf("production load without explicit HTTP_ADDR returned %v", err)
	}
}

func TestProductionLoadRequiresExplicitDatabaseConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("MYSQL_DSN", "")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MYSQL_DSN") {
		t.Fatalf("production load without explicit MYSQL_DSN returned %v", err)
	}
}

func TestCORSOriginsAreParsedAndValidated(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", " https://admin.example.com,https://admin.example.com, http://localhost:5173 ")
	origins := envList("CORS_ALLOW_ORIGINS")
	if len(origins) != 2 || origins[0] != "https://admin.example.com" || origins[1] != "http://localhost:5173" {
		t.Fatalf("unexpected origins: %#v", origins)
	}

	cfg := validConfig()
	cfg.CORSAllowedOrigins = []string{"https://admin.example.com/path"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid origin") {
		t.Fatalf("invalid CORS origin returned %v", err)
	}

	cfg.CORSAllowedOrigins = []string{"http://admin.example.com"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("insecure production CORS origin returned %v", err)
	}
}

func TestProductionValidationRejectsDefaultDatabaseDSN(t *testing.T) {
	cfg := validConfig()
	cfg.MySQLDSN = defaultMySQLDSN
	if err := cfg.Validate(); err == nil {
		t.Fatal("production accepted the development database DSN")
	}
}

func TestProductionValidationRequiresNativeProxyHTTPAddress(t *testing.T) {
	for _, address := range []string{":8080", "0.0.0.0:8080", "[::]:8080", "[::1]:8080", "127.0.0.2:8080", "localhost:8080", "api.example.com:8080", "127.0.0.1:", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:not-a-port"} {
		t.Run(address, func(t *testing.T) {
			cfg := validConfig()
			cfg.HTTPAddr = address
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
				t.Fatalf("unsafe production HTTP_ADDR %q returned %v", address, err)
			}
		})
	}
	for _, address := range []string{"127.0.0.1:8080", "127.0.0.1:18080"} {
		t.Run("allowed-"+address, func(t *testing.T) {
			cfg := validConfig()
			cfg.HTTPAddr = address
			if err := cfg.Validate(); err != nil {
				t.Fatalf("loopback production HTTP_ADDR %q was rejected: %v", address, err)
			}
		})
	}
}

func TestConfigurationRejectsUnknownEnvironmentName(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "production"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown APP_ENV was treated as a development profile")
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
