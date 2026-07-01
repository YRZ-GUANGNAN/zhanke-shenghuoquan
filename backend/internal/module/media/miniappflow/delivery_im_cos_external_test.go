package miniappflow

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/deliveryim"
	"campus-circle/backend/internal/module/delivery"
	coremedia "campus-circle/backend/internal/module/media"
	"github.com/joho/godotenv"
)

func TestD5DeliveryIMUploadReviewBindAndFixedVersionResolveAgainstRealCOS(t *testing.T) {
	if os.Getenv("DELIVERY_D5_RUN_REAL_COS") != "1" {
		t.Skip("set DELIVERY_D5_RUN_REAL_COS=1 for real D-5 COS acceptance")
	}
	_ = godotenv.Load("../../../../.env")
	db := openModule10MySQL(t)
	secretID := module10Env("DELIVERY_D5_COS_SECRET_ID", "MEDIA_MODULE10_COS_SECRET_ID", "COS_SECRET_ID")
	secretKey := module10Env("DELIVERY_D5_COS_SECRET_KEY", "MEDIA_MODULE10_COS_SECRET_KEY", "COS_SECRET_KEY")
	bucket := module10Env("DELIVERY_D5_COS_BUCKET", "MEDIA_MODULE10_COS_BUCKET", "COS_BUCKET")
	region := module10Env("DELIVERY_D5_COS_REGION", "MEDIA_MODULE10_COS_REGION", "COS_REGION")
	appID := module10Env("DELIVERY_D5_COS_APP_ID", "MEDIA_MODULE10_COS_APP_ID", "COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Fatal("real D-5 COS credentials are incomplete")
	}
	bucket = resolveModule10Bucket(t, secretID, secretKey, bucket, region, appID)

	const ownerID uint64 = 99_005_010
	cleanupModule10Owner(t, db, ownerID)
	t.Cleanup(func() { cleanupModule10Owner(t, db, ownerID) })
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	authorizer, err := coremedia.NewTencentUploadAuthorizer(coremedia.TencentUploadAuthorizerConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region,
		AppID: appID, STSTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := coremedia.NewTencentCompletionStore(coremedia.TencentCompletionStoreConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region,
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := coremedia.NewRepository(db)
	tickets, err := coremedia.NewUploadTicketService(
		repository, authorizer, coremedia.UploadTicketServiceConfig{Environment: "dev"},
	)
	if err != nil {
		t.Fatal(err)
	}
	completion, err := coremedia.NewCompletionService(
		repository, store, coremedia.CompletionServiceConfig{Environment: "dev"},
	)
	if err != nil {
		t.Fatal(err)
	}
	projections, err := coremedia.NewReviewProjectionService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coremedia.NewMediaConsumer(repository, projections, coremedia.MediaConsumerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	mediaService := coremedia.NewService(repository)
	application, err := NewService(
		tickets, repository, completion, mediaService, projections,
		repository, consumer, NewStatusRepository(db), nil,
		WithLocalMockAutoApproval(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := module10PNG(t)
	digest := md5.Sum(payload)
	session, err := application.CreateUpload(ctx, CreateUploadInput{
		OwnerID: ownerID, Purpose: coremedia.PurposeDeliveryIM,
		SizeBytes: uint64(len(payload)), ContentMD5: base64.StdEncoding.EncodeToString(digest[:]),
		ContentType: "image/png", IdempotencyKey: fmt.Sprintf("d5-real-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("real D-5 STS authorization: %v", err)
	}
	bundle, err := repository.FindUploadBundle(ctx, session.TicketID, ownerID)
	if err != nil || bundle.Asset.StagingKey == nil {
		t.Fatalf("load D-5 staging identity: %v", err)
	}
	stagingKey := *bundle.Asset.StagingKey
	finalKey := ""
	cosClient := module10COSClient(t, secretID, secretKey, bucket, region)
	t.Cleanup(func() {
		cleanupModule10COSKey(t, cosClient, stagingKey)
		if finalKey != "" {
			cleanupModule10COSKey(t, cosClient, finalKey)
		}
	})
	putModule10Object(t, ctx, session, payload)
	status, err := application.CompleteUpload(ctx, ownerID, session.TicketID)
	if err != nil {
		t.Fatalf("real D-5 completion/mock review: %v", err)
	}
	if status.State != StateApproved || !status.CanUse {
		t.Fatalf("D-5 local review did not approve: %#v", status)
	}
	asset, err := repository.FindAsset(ctx, session.AssetID)
	if err != nil || asset.FinalKey == nil || asset.FinalVersionID == nil {
		t.Fatalf("D-5 fixed final identity missing: asset=%#v err=%v", asset, err)
	}
	finalKey = *asset.FinalKey

	gateway, err := coremedia.NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	privateDisplay, err := coremedia.NewMediaDisplayService(
		gateway, repository, store, delivery.PrivateConversationImageURLTTL,
	)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := deliveryim.NewMediaAdapter(mediaService, gateway, privateDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.ValidateOwnedApprovedConversationAsset(ctx, ownerID, asset.AssetPublicID); err != nil {
		t.Fatalf("D-5 owner/purpose/review validation: %v", err)
	}
	conversationPublicID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.PublishConversationImage(ctx, asset.AssetPublicID, conversationPublicID, 0); err != nil {
		t.Fatalf("D-5 private media reference publish: %v", err)
	}
	signedURL, err := adapter.ResolveConversationImage(
		ctx, conversationPublicID, 0, delivery.PrivateConversationImageURLTTL,
	)
	if err != nil {
		t.Fatalf("D-5 fixed-version private resolve: %v", err)
	}
	if !strings.Contains(signedURL, "q-signature=") || !strings.Contains(signedURL, "versionId=") {
		t.Fatalf("D-5 URL lacks fixed-version COS signature: %s", redactResolveURL(signedURL))
	}
	if got := fetchResolvedURL(t, ctx, signedURL); string(got) != string(payload) {
		t.Fatal("D-5 signed GET payload mismatch")
	}
	if err := adapter.ValidateOwnedApprovedConversationAsset(ctx, ownerID+1, asset.AssetPublicID); err == nil {
		t.Fatal("D-5 cross-owner conversation image validation unexpectedly succeeded")
	}
	t.Logf(
		"real_sts=PASS cos_put=PASS completion=PASS purpose=delivery_im local_mock_review=PASS fixed_final_version=%s signed_get_ttl=60 private_reference=PASS owner_fence=PASS",
		redactModule10(*asset.FinalVersionID),
	)
}
