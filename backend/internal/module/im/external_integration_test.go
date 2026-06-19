package im

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestTencentAccountImportAgainstConfiguredApplication(t *testing.T) {
	sdkAppID, secret, admin := os.Getenv("TEST_TENCENT_IM_SDK_APP_ID"), os.Getenv("TEST_TENCENT_IM_SECRET_KEY"), os.Getenv("TEST_TENCENT_IM_ADMIN_USER")
	if sdkAppID == "" || secret == "" || admin == "" {
		t.Skip("real Tencent IM test credentials are not configured")
	}
	client, err := NewTencentRESTClient(sdkAppID, secret, admin, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, userID := range []string{"campus_5", "campus_6"} {
		if err := client.ImportAccount(ctx, userID); err != nil {
			t.Fatalf("import %s: %v", userID, err)
		}
		userSig, err := generateUserSig(sdkAppID, secret, userID, 3600)
		if err != nil {
			t.Fatalf("generate UserSig for %s: %v", userID, err)
		}
		if len(userSig) < 100 {
			t.Fatalf("UserSig for %s is unexpectedly short", userID)
		}
	}

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("real Account Import passed; MySQL DSN is not configured for UserSig persistence check")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&UserMapping{}, &Conversation{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), client, sdkAppID, secret)
	for _, userID := range []uint{5, 6} {
		credentials, err := service.Credentials(ctx, userID)
		if err != nil {
			t.Fatalf("issue real credentials for user %d: %v", userID, err)
		}
		if credentials.SDKAppID != sdkAppID || credentials.UserID == "" || len(credentials.UserSig) < 100 {
			t.Fatalf("invalid credentials for user %d", userID)
		}
	}
}
