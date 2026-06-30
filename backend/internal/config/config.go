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

	WechatPayMchID            string
	WechatPayAPIv3Key         string
	WechatPayMerchantSerialNo string
	WechatPayPrivateKeyPath   string
	WechatPayNotifyURL        string
	WechatPayAPIBase          string
	WechatPayPlatformKeyPath  string
	AllowMockPayment          bool

	EnableLegacyTakeout              bool
	EnableDelivery                   bool
	EnableDeliveryPayment            bool
	AllowMockDeliveryFeeConfirmation bool
	AllowMockDeliveryPayment         bool
	DeliveryWechatPayNotifyURL       string
	DeliveryWechatRefundNotifyURL    string
	WeComRiderWebhookURL             string
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

		WechatPayMchID:            env("WECHAT_PAY_MCH_ID", ""),
		WechatPayAPIv3Key:         env("WECHAT_PAY_API_V3_KEY", ""),
		WechatPayMerchantSerialNo: env("WECHAT_PAY_MERCHANT_SERIAL_NO", ""),
		WechatPayPrivateKeyPath:   env("WECHAT_PAY_PRIVATE_KEY_PATH", ""),
		WechatPayNotifyURL:        env("WECHAT_PAY_NOTIFY_URL", ""),
		WechatPayAPIBase:          env("WECHAT_PAY_API_BASE", "https://api.mch.weixin.qq.com"),
		WechatPayPlatformKeyPath:  env("WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH", ""),
		AllowMockPayment:          envBool("ALLOW_MOCK_PAYMENT", false),

		EnableLegacyTakeout:              envBool("ENABLE_LEGACY_TAKEOUT", false),
		EnableDelivery:                   envBool("ENABLE_DELIVERY", true),
		EnableDeliveryPayment:            envBool("ENABLE_DELIVERY_PAYMENT", false),
		AllowMockDeliveryFeeConfirmation: envBool("ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION", false),
		AllowMockDeliveryPayment:         envBool("ALLOW_MOCK_DELIVERY_PAYMENT", false),
		DeliveryWechatPayNotifyURL:       env("DELIVERY_WECHAT_PAY_NOTIFY_URL", ""),
		DeliveryWechatRefundNotifyURL:    env("DELIVERY_WECHAT_REFUND_NOTIFY_URL", ""),
		WeComRiderWebhookURL:             env("WECOM_RIDER_WEBHOOK_URL", ""),
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
	if c.WechatPayAnyCoreCredentialSet() && !c.WechatPayCoreConfigured() {
		return errors.New("WeChat Pay core credentials must be configured together")
	}
	if c.WechatPayNotifyURL != "" && !c.WechatPayCoreConfigured() {
		return errors.New("legacy WeChat Pay notify URL requires complete core credentials")
	}
	if c.WechatPayPlatformKeyPath != "" && !c.WechatPayCoreConfigured() {
		return errors.New("WeChat Pay platform public key requires complete core credentials")
	}
	if (c.DeliveryWechatPayNotifyURL != "" || c.DeliveryWechatRefundNotifyURL != "") && !c.WechatPayCoreConfigured() {
		return errors.New("delivery WeChat Pay notify URLs require complete core credentials")
	}
	if c.AllowMockPayment && c.WechatPayAnyCredentialSet() {
		return errors.New("mock payment cannot be enabled with real WeChat Pay credentials")
	}
	if c.AllowMockDeliveryPayment && c.DeliveryWechatPayAnyCredentialSet() {
		return errors.New("mock delivery payment cannot be enabled with real WeChat Pay credentials")
	}
	if c.EnableDeliveryPayment && c.AllowMockDeliveryPayment && c.AllowMockDeliveryFeeConfirmation {
		return errors.New("delivery payment mock and legacy fee confirmation mock cannot be enabled together")
	}
	if c.EnableDeliveryPayment && !c.AllowMockDeliveryPayment &&
		c.DeliveryWechatPayAnyCredentialSet() && !c.DeliveryWechatPayConfigured() {
		return errors.New("enabled delivery payment requires either no real credentials or a complete real configuration")
	}
	if c.AppEnv == "prod" {
		if c.WechatAppID == "" || c.WechatAppSecret == "" {
			return errors.New("production requires WECHAT_APP_ID and WECHAT_APP_SECRET")
		}
		if c.AllowMockWechat {
			return errors.New("mock WeChat login is forbidden in production")
		}
		if c.AllowMockPayment {
			return errors.New("mock payment is forbidden in production")
		}
		if c.AllowMockDeliveryFeeConfirmation {
			return errors.New("mock delivery fee confirmation is forbidden in production")
		}
		if c.AllowMockDeliveryPayment {
			return errors.New("mock delivery payment is forbidden in production")
		}
		if c.EnableLegacyTakeout && !c.WechatPayConfigured() {
			return errors.New("production requires complete WeChat Pay credentials")
		}
		if c.EnableDeliveryPayment && !c.DeliveryWechatPayConfigured() {
			return errors.New("production delivery payment requires complete WeChat Pay credentials and notify URLs")
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

func (c Config) WechatPayConfigured() bool {
	return c.WechatPayCoreConfigured() && c.WechatPayNotifyURL != ""
}

func (c Config) WechatPayCoreConfigured() bool {
	return c.WechatPayMchID != "" && c.WechatPayAPIv3Key != "" && c.WechatPayMerchantSerialNo != "" &&
		c.WechatPayPrivateKeyPath != ""
}

func (c Config) DeliveryWechatPayConfigured() bool {
	return c.WechatPayCoreConfigured() && c.WechatPayPlatformKeyPath != "" &&
		c.DeliveryWechatPayNotifyURL != "" && c.DeliveryWechatRefundNotifyURL != ""
}

func (c Config) WechatPayAnyCredentialSet() bool {
	return c.WechatPayMchID != "" || c.WechatPayAPIv3Key != "" || c.WechatPayMerchantSerialNo != "" ||
		c.WechatPayPrivateKeyPath != "" || c.WechatPayNotifyURL != "" || c.WechatPayPlatformKeyPath != ""
}

func (c Config) WechatPayAnyCoreCredentialSet() bool {
	return c.WechatPayMchID != "" || c.WechatPayAPIv3Key != "" || c.WechatPayMerchantSerialNo != "" ||
		c.WechatPayPrivateKeyPath != ""
}

func (c Config) DeliveryWechatPayAnyCredentialSet() bool {
	return c.WechatPayAnyCoreCredentialSet() || c.WechatPayPlatformKeyPath != "" ||
		c.DeliveryWechatPayNotifyURL != "" || c.DeliveryWechatRefundNotifyURL != ""
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
