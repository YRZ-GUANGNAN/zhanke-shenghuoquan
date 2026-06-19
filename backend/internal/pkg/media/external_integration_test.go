package media

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMySQLMediaIdempotencyAndReservationConcurrency(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated MySQL integration DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Asset{}, &AssetAudit{}); err != nil {
		t.Fatal(err)
	}
	type indexRow struct {
		Name    string
		Columns string
	}
	var indexes []indexRow
	if err := db.Raw(`SELECT index_name AS name, GROUP_CONCAT(column_name ORDER BY seq_in_index) AS columns
		FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'media_assets'
		GROUP BY index_name`).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	found := make(map[string]string, len(indexes))
	for _, index := range indexes {
		found[index.Name] = index.Columns
	}
	wantIndexes := map[string]string{
		"idx_media_owner_idempotency": "owner_user_id,idempotency_key",
		"idx_media_review_pending":    "status,review_available_at,id",
		"idx_media_delete_pending":    "delete_requested_at,storage_deleted_at,id",
		"idx_media_reference":         "reference_type,reference_id,reference_sort",
	}
	for name, columns := range wantIndexes {
		if found[name] != columns {
			t.Fatalf("index %s=%q want=%q; all=%v", name, found[name], columns, found)
		}
	}
	repo := NewRepository(db)
	stamp := time.Now().UnixNano()
	owner := uint(stamp%1000000 + 2000000)
	key := fmt.Sprintf("p04-%d", stamp)
	var wg sync.WaitGroup
	var created atomic.Int32
	ids := make(chan uint64, 20)
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			value := Asset{PublicID: fmt.Sprintf("%032x", stamp+int64(index)), OwnerUserID: owner, Purpose: PurposePostImage, ObjectKey: fmt.Sprintf("media/test/%d-%d.jpg", stamp, index), DeclaredMIME: "image/jpeg", DeclaredSize: 10, Status: StatusPendingUpload, IdempotencyKey: key, UploadExpiresAt: time.Now().Add(time.Minute), ReviewAvailableAt: time.Now()}
			wasCreated, e := repo.CreateOrFind(context.Background(), &value)
			if wasCreated {
				created.Add(1)
			}
			if e == nil {
				ids <- value.ID
			}
			errs <- e
		}(i)
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	unique := map[uint64]bool{}
	for id := range ids {
		unique[id] = true
	}
	if created.Load() != 1 || len(unique) != 1 {
		t.Fatalf("created=%d ids=%v", created.Load(), unique)
	}
	var asset Asset
	for id := range unique {
		if err := db.First(&asset, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	defer db.Delete(&Asset{}, asset.ID)
	defer db.Where("asset_id = ?", asset.ID).Delete(&AssetAudit{})
	var createdAudits int64
	if err := db.Model(&AssetAudit{}).Where("asset_id = ? AND action = ?", asset.ID, "upload_created").Count(&createdAudits).Error; err != nil {
		t.Fatal(err)
	}
	if createdAudits != 1 {
		t.Fatalf("concurrent create audit count=%d want=1", createdAudits)
	}
	if err := db.Model(&Asset{}).Where("id = ?", asset.ID).Updates(map[string]any{"status": StatusApproved}).Error; err != nil {
		t.Fatal(err)
	}
	asset.Status = StatusApproved
	var reservations atomic.Int32
	reserveErrs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, e := repo.Reserve(context.Background(), owner, PurposePostImage, []string{asset.PublicID}, fmt.Sprintf("%032x", index+1), time.Now().Add(time.Minute))
			if e == nil {
				reservations.Add(1)
			}
			reserveErrs <- e
		}(i)
	}
	wg.Wait()
	close(reserveErrs)
	for e := range reserveErrs {
		if e != nil && e != ErrAlreadyBound {
			t.Fatal(e)
		}
	}
	if reservations.Load() != 1 {
		t.Fatalf("concurrent reservations=%d", reservations.Load())
	}
	if err := db.First(&asset, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	referenceID := uint64(stamp)
	superseded := Asset{PublicID: fmt.Sprintf("%032x", stamp+100), OwnerUserID: owner, Purpose: PurposePostImage, ObjectKey: fmt.Sprintf("media/test/%d-superseded.jpg", stamp), DeclaredMIME: "image/jpeg", DeclaredSize: 10, Status: StatusApproved, ReferenceType: "post", ReferenceID: referenceID, IdempotencyKey: key + "-superseded", UploadExpiresAt: time.Now().Add(time.Hour), ReviewAvailableAt: time.Now()}
	if err := db.Create(&superseded).Error; err != nil {
		t.Fatal(err)
	}
	defer db.Delete(&Asset{}, superseded.ID)
	defer db.Where("asset_id = ?", superseded.ID).Delete(&AssetAudit{})
	if err := repo.Commit(context.Background(), owner, asset.ReservationToken, "post", referenceID, []string{asset.PublicID}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&superseded, superseded.ID).Error; err != nil {
		t.Fatal(err)
	}
	if superseded.DeleteRequestedAt == nil || superseded.Status != StatusDeleted {
		t.Fatalf("superseded reference was not deleted atomically: %+v", superseded)
	}
	var referenceAudits int64
	if err := db.Model(&AssetAudit{}).Where("asset_id IN ? AND action IN ?", []uint64{asset.ID, superseded.ID}, []string{"reference_bound", "reference_replaced"}).Count(&referenceAudits).Error; err != nil {
		t.Fatal(err)
	}
	if referenceAudits != 2 {
		t.Fatalf("reference audit count=%d want=2", referenceAudits)
	}
	now := time.Now()
	garbageCases := []Asset{
		{PublicID: fmt.Sprintf("%032x", stamp+101), OwnerUserID: owner, Purpose: PurposePostImage, ObjectKey: fmt.Sprintf("media/test/%d-pending.jpg", stamp), DeclaredMIME: "image/jpeg", DeclaredSize: 10, Status: StatusPendingUpload, IdempotencyKey: key + "-pending", UploadExpiresAt: now.Add(-time.Minute), ReviewAvailableAt: now, UpdatedAt: now},
		{PublicID: fmt.Sprintf("%032x", stamp+102), OwnerUserID: owner, Purpose: PurposePostImage, ObjectKey: fmt.Sprintf("media/test/%d-unsubmitted.jpg", stamp), DeclaredMIME: "image/jpeg", DeclaredSize: 10, Status: StatusUploaded, IdempotencyKey: key + "-unsubmitted", UploadExpiresAt: now.Add(time.Hour), ReviewAvailableAt: now, UpdatedAt: now.Add(-25 * time.Hour)},
		{PublicID: fmt.Sprintf("%032x", stamp+103), OwnerUserID: owner, Purpose: PurposePostImage, ObjectKey: fmt.Sprintf("media/test/%d-rejected.jpg", stamp), DeclaredMIME: "image/jpeg", DeclaredSize: 10, Status: StatusRejected, IdempotencyKey: key + "-rejected", UploadExpiresAt: now.Add(time.Hour), ReviewAvailableAt: now, UpdatedAt: now.Add(-25 * time.Hour)},
	}
	for index := range garbageCases {
		if err := db.Create(&garbageCases[index]).Error; err != nil {
			t.Fatal(err)
		}
		defer db.Delete(&Asset{}, garbageCases[index].ID)
		defer db.Where("asset_id = ?", garbageCases[index].ID).Delete(&AssetAudit{})
	}
	garbage, err := repo.ExpiredGarbage(context.Background(), 100, now)
	if err != nil {
		t.Fatal(err)
	}
	foundGarbage := map[uint64]bool{}
	for _, value := range garbage {
		foundGarbage[value.ID] = true
	}
	for _, value := range garbageCases {
		if !foundGarbage[value.ID] {
			t.Fatalf("garbage asset %d (%s) was not selected", value.ID, value.Status)
		}
	}
}

func TestTencentCOSScopedSTSAndObjectLifecycle(t *testing.T) {
	secretID, secretKey, bucket, region, appID := os.Getenv("TEST_COS_SECRET_ID"), os.Getenv("TEST_COS_SECRET_KEY"), os.Getenv("TEST_COS_BUCKET"), os.Getenv("TEST_COS_REGION"), os.Getenv("TEST_COS_APP_ID")
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real Tencent COS test credentials are not configured")
	}
	adapter, err := NewTencentCOS(secretID, secretKey, bucket, region, appID)
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("media/p04-acceptance/%d.png", time.Now().UnixNano())
	credential, err := adapter.Issue(context.Background(), key, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	bucketURL, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	temporary := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{Transport: &cos.AuthorizationTransport{SecretID: credential.SecretID, SecretKey: credential.SecretKey, SessionToken: credential.SessionToken}, Timeout: 15 * time.Second})
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	if _, err := temporary.Object.Put(context.Background(), key, bytes.NewReader(png), &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "image/png"}}); err != nil {
		t.Fatal(err)
	}
	defer adapter.Delete(context.Background(), key)
	if _, err := temporary.Object.Put(context.Background(), key+"-forbidden", bytes.NewReader(png), &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "image/png"}}); err == nil {
		t.Fatal("scoped STS wrote an unauthorized object key")
	}
	info, err := adapter.Stat(context.Background(), key)
	if err != nil || info.Size != int64(len(png)) || info.ContentType != "image/png" {
		t.Fatalf("stat=%+v err=%v", info, err)
	}
	prefix, err := adapter.ReadPrefix(context.Background(), key, 8)
	if err != nil || !bytes.Equal(prefix, png[:8]) {
		t.Fatalf("prefix=%v err=%v", prefix, err)
	}
	signed, err := adapter.SignedGetURL(context.Background(), key, time.Minute)
	if err != nil || signed == "" {
		t.Fatalf("signed url err=%v", err)
	}
	if err := adapter.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
}
