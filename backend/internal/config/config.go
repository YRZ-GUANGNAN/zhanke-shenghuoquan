package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv    string
	HTTPAddr  string
	JWTSecret string

	MySQLDSN          string
	TencentIMSDKAppID string
	TencentIMSecret   string
	TencentIMAdmin    string
	TencentIMEndpoint string

	MeiliHost      string
	MeiliMasterKey string
	MeiliIndex     string

	COSSecretID         string
	COSSecretKey        string
	COSBucket           string
	COSRegion           string
	COSAppID            string
	MediaPublicBaseURL  string
	WechatCallbackToken string
	WechatAPIBase       string

	WechatAppID     string
	WechatAppSecret string
	AllowMockWechat bool
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		AppEnv:    env("APP_ENV", "dev"),
		HTTPAddr:  env("HTTP_ADDR", ":8080"),
		JWTSecret: env("JWT_SECRET", "dev-secret-change-me"),

		MySQLDSN:          env("MYSQL_DSN", "campus:campus@tcp(127.0.0.1:3306)/campus_circle?charset=utf8mb4&parseTime=True&loc=Local"),
		TencentIMSDKAppID: env("TENCENT_IM_SDK_APP_ID", ""),
		TencentIMSecret:   env("TENCENT_IM_SECRET_KEY", ""),
		TencentIMAdmin:    env("TENCENT_IM_ADMIN_USER", ""),
		TencentIMEndpoint: env("TENCENT_IM_REST_ENDPOINT", "https://console.tim.qq.com"),

		MeiliHost:      env("MEILI_HOST", ""),
		MeiliMasterKey: env("MEILI_MASTER_KEY", ""),
		MeiliIndex:     env("MEILI_INDEX", "products"),

		COSSecretID:         env("COS_SECRET_ID", ""),
		COSSecretKey:        env("COS_SECRET_KEY", ""),
		COSBucket:           env("COS_BUCKET", ""),
		COSRegion:           env("COS_REGION", ""),
		COSAppID:            env("COS_APP_ID", ""),
		MediaPublicBaseURL:  env("MEDIA_PUBLIC_BASE_URL", ""),
		WechatCallbackToken: env("WECHAT_CALLBACK_TOKEN", ""),
		WechatAPIBase:       env("WECHAT_API_BASE", "https://api.weixin.qq.com"),

		WechatAppID:     env("WECHAT_APP_ID", ""),
		WechatAppSecret: env("WECHAT_APP_SECRET", ""),
		AllowMockWechat: envBool("ALLOW_MOCK_WECHAT_LOGIN", false),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if len(c.JWTSecret) < 32 || c.JWTSecret == "dev-secret-change-me" {
		return errors.New("JWT_SECRET must be a non-default secret of at least 32 characters")
	}
	if (c.WechatAppID == "") != (c.WechatAppSecret == "") {
		return errors.New("WECHAT_APP_ID and WECHAT_APP_SECRET must be configured together")
	}
	if c.AppEnv == "prod" {
		if c.WechatAppID == "" || c.WechatAppSecret == "" {
			return errors.New("production requires WECHAT_APP_ID and WECHAT_APP_SECRET")
		}
		if c.AllowMockWechat {
			return errors.New("mock WeChat login is forbidden in production")
		}
		if c.TencentIMSDKAppID == "" || c.TencentIMSecret == "" || c.TencentIMAdmin == "" {
			return errors.New("production requires Tencent IM SDKAppID, secret and administrator")
		}
		if c.MeiliHost == "" || len(c.MeiliMasterKey) < 16 {
			return errors.New("production requires Meilisearch host and a non-trivial master key")
		}
		if c.COSSecretID == "" || c.COSSecretKey == "" || c.COSBucket == "" || c.COSRegion == "" || c.COSAppID == "" {
			return errors.New("production requires COS credentials, bucket, region and app id")
		}
		if !strings.HasPrefix(c.MediaPublicBaseURL, "https://") || len(c.WechatCallbackToken) < 16 {
			return errors.New("production requires HTTPS MEDIA_PUBLIC_BASE_URL and a non-trivial WECHAT_CALLBACK_TOKEN")
		}
	} else if c.WechatAppID == "" && !c.AllowMockWechat {
		return fmt.Errorf("WeChat credentials are missing; set ALLOW_MOCK_WECHAT_LOGIN=true only for local development")
	}
	return nil
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
