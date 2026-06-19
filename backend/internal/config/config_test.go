package config

import "testing"

func validConfig() Config {
	return Config{
		AppEnv: "prod", JWTSecret: "production-secret-that-is-longer-than-thirty-two-characters",
		WechatAppID: "wx-app", WechatAppSecret: "wx-secret",
		TencentIMSDKAppID: "1400000000", TencentIMSecret: "im-secret", TencentIMAdmin: "administrator",
		MeiliHost: "http://meilisearch:7700", MeiliMasterKey: "meili-secret-that-is-long-enough", MeiliIndex: "products",
		COSSecretID: "cos-id", COSSecretKey: "cos-key", COSBucket: "campus-1234567890", COSRegion: "ap-guangzhou", COSAppID: "1234567890",
		MediaPublicBaseURL: "https://api.example.com", WechatCallbackToken: "wechat-callback-token-strong",
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

func TestProductionConfigurationAcceptsExplicitCredentials(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}
