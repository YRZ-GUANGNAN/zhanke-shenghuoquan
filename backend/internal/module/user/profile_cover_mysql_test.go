package user

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type failCoverPublish struct {
	*coremedia.ReferenceBindingService
}

func (s failCoverPublish) Publish(ctx context.Context, p coremedia.Purpose, id string) error {
	if err := s.ReferenceBindingService.Publish(ctx, p, id); err != nil {
		return err
	}
	return errors.New("isolated failure after media publication")
}

func TestProfileCoverTemporaryMySQL(t *testing.T) {
	dsn := os.Getenv("PROFILE_COVER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated profile-cover test engine")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil || cfg.Addr != "127.0.0.1:33308" || !strings.HasPrefix(cfg.DBName, "campus_cover_test_") {
		t.Fatal("only an isolated local cover test database is permitted")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&count).Error; err != nil || count != 0 {
		t.Fatal("test database must start empty")
	}
	exec := func(sql string) {
		t.Helper()
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("test schema: %v", err)
		}
	}
	if err := db.AutoMigrate(&User{}, &coremedia.Asset{}, &coremedia.AssetReference{}, &coremedia.ReviewProjection{}); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the pre-042 boundary, then apply the actual checked-in migration.
	exec("ALTER TABLE users DROP COLUMN cover, DROP COLUMN cover_asset_id")
	exec("ALTER TABLE media_assets ADD COLUMN public_id VARCHAR(32) NULL, ADD COLUMN reservation_token VARCHAR(32) NOT NULL DEFAULT '', ADD COLUMN reservation_expires_at DATETIME(3) NULL, ADD CONSTRAINT chk_media_asset_purpose CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'))")
	exec("ALTER TABLE media_asset_references ADD CONSTRAINT chk_media_reference_type CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'))")
	exec("ALTER TABLE media_review_projections ADD CONSTRAINT chk_media_review_projection_status CHECK (status IN ('pending','submitted','reviewing','approved','rejected','review_failed'))")
	seedUser := User{ID: 7, OpenID: "isolated-profile-cover", Nickname: "原昵称", Avatar: "existing-avatar", School: "原学校", Campus: "原校区", Status: StatusActive}
	if err := db.Omit("Cover", "CoverAssetID").Create(&seedUser).Error; err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../../migrations/042_user_profile_cover.sql")
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	for _, line := range strings.Split(string(source), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		if strings.TrimSpace(statement) != "" {
			exec(statement)
		}
	}
	policyMigration, err := os.ReadFile("../../../migrations/044_media_review_policy.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(string(policyMigration))
	repo := NewRepository(db)
	loaded, err := repo.FindByID(context.Background(), 7)
	if err != nil || loaded.Cover != "" || loaded.Avatar != seedUser.Avatar {
		t.Fatalf("migration changed existing profile: %+v %v", loaded, err)
	}
	bindings, _ := coremedia.NewReferenceBindingService(coremedia.NewReferenceBindingRepository(db), nil)
	service, _ := NewProfileCoverService(repo, bindings, "https://media.example")
	gateway, _ := coremedia.NewMediaGateway(coremedia.NewRepository(db))
	ctx := context.Background()
	newAsset := func(owner uint64, purpose coremedia.Purpose, status coremedia.ProjectionStatus) coremedia.Asset {
		t.Helper()
		id, _ := coremedia.NewULID()
		projectionID, _ := coremedia.NewULID()
		key, version := "isolated/"+id, "isolated-version"
		value := coremedia.Asset{AssetPublicID: id, OwnerID: owner, Purpose: purpose, StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady, ReviewStatus: coremedia.ReviewApproved, FinalKey: &key, FinalVersionID: &version, VerifiedMIME: "image/jpeg", ByteSize: 512, Width: 1600, Height: 600, IdempotencyKey: id, RowVersion: 1}
		if err := db.Create(&value).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&coremedia.ReviewProjection{ProjectionID: projectionID, AssetID: value.ID, Status: status, RowVersion: 1}).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	visible := func(id string) bool {
		t.Helper()
		v, err := gateway.CheckVisibility(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return v.Visible
	}
	first := newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionApproved)
	if _, err := service.Update(ctx, 7, first.AssetPublicID); err != nil {
		t.Fatal(err)
	}
	loaded, err = NewRepository(db).FindByID(ctx, 7)
	if err != nil || loaded.CoverAssetID == nil || *loaded.CoverAssetID != first.ID || !visible(first.AssetPublicID) {
		t.Fatal("cover was not persisted and published")
	}
	if _, err := service.Update(ctx, 7, first.AssetPublicID); err != nil {
		t.Fatalf("exact replay: %v", err)
	}

	for _, invalid := range []coremedia.Asset{
		newAsset(8, coremedia.PurposeProfileCover, coremedia.ProjectionApproved),
		newAsset(7, coremedia.PurposeAvatar, coremedia.ProjectionApproved),
		newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionPending),
		newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionRejected),
	} {
		if _, err := service.Update(ctx, 7, invalid.AssetPublicID); err == nil {
			t.Fatal("accepted foreign, wrong-purpose or unapproved asset")
		}
		loaded, _ = repo.FindByID(ctx, 7)
		if *loaded.CoverAssetID != first.ID {
			t.Fatal("invalid image changed cover")
		}
	}

	second := newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionApproved)
	failing, _ := NewProfileCoverService(repo, failCoverPublish{bindings}, "https://media.example")
	if _, err := failing.Update(ctx, 7, second.AssetPublicID); err == nil {
		t.Fatal("expected injected failure")
	}
	loaded, _ = repo.FindByID(ctx, 7)
	if *loaded.CoverAssetID != first.ID || !visible(first.AssetPublicID) || visible(second.AssetPublicID) {
		t.Fatal("media failure did not roll back both profile and references")
	}
	var reservation string
	if err := db.Raw("SELECT reservation_token FROM media_assets WHERE id = ?", second.ID).Scan(&reservation).Error; err != nil || reservation != "" {
		t.Fatal("failed transaction leaked media reservation")
	}
	exec("CREATE TRIGGER fail_cover_row BEFORE UPDATE ON users FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'isolated user write failure'")
	if _, err := service.Update(ctx, 7, second.AssetPublicID); err == nil {
		t.Fatal("expected row-write failure")
	}
	exec("DROP TRIGGER fail_cover_row")
	if !visible(first.AssetPublicID) || visible(second.AssetPublicID) {
		t.Fatal("row failure leaked new media reference")
	}
	if _, err := service.Update(ctx, 7, second.AssetPublicID); err != nil {
		t.Fatal(err)
	}
	if visible(first.AssetPublicID) || !visible(second.AssetPublicID) {
		t.Fatal("replacement did not revoke previous cover")
	}

	third, fourth := newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionApproved), newAsset(7, coremedia.PurposeProfileCover, coremedia.ProjectionApproved)
	var workers sync.WaitGroup
	results := make(chan error, 2)
	for _, asset := range []coremedia.Asset{third, fourth} {
		workers.Add(1)
		go func(id string) { defer workers.Done(); _, err := service.Update(ctx, 7, id); results <- err }(asset.AssetPublicID)
	}
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("serialized cover update: %v", err)
		}
	}
	if err := db.Model(&coremedia.AssetReference{}).Where("reference_type = ? AND reference_id = ? AND status = ? AND parent_visibility = ?", coremedia.PurposeProfileCover, "7", coremedia.ReferenceActive, coremedia.ParentVisible).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("expected exactly one active cover, got %d %v", count, err)
	}
	loaded, _ = repo.FindByID(ctx, 7)
	if loaded.Avatar != seedUser.Avatar || loaded.Nickname != seedUser.Nickname || loaded.School != seedUser.School {
		t.Fatal("cover mutated unrelated profile")
	}
	if err := repo.UpdateProfile(ctx, 7, Profile{Nickname: "资料编辑后的昵称", Avatar: seedUser.Avatar, School: seedUser.School}); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.FindByID(ctx, 7)
	if updated.Cover != loaded.Cover {
		t.Fatal("old profile endpoint overwrote cover")
	}
	if _, err := service.Update(ctx, 7, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, 7, ""); err != nil {
		t.Fatal("clear is not idempotent")
	}
	if visible(third.AssetPublicID) || visible(fourth.AssetPublicID) {
		t.Fatal("cleared cover remains visible")
	}
	updated, _ = repo.FindByID(ctx, 7)
	if updated.Cover != "" || updated.CoverAssetID != nil || updated.Nickname != "资料编辑后的昵称" {
		t.Fatal("clear damaged profile")
	}
	t.Run("direct media policy and atomic profile updates", func(t *testing.T) {
		testDirectMediaPolicyMySQL(t, db, repo, bindings, newAsset)
	})
	if err := db.Model(&User{}).Where("id = ?", 7).Update("status", StatusMuted).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, 7, ""); !errors.Is(err, ErrCoverForbidden) {
		t.Fatal("locked user status was not rechecked")
	}
	t.Log("migration, persistence, exact replay, ownership, purpose, review, transaction rollback, concurrent replacement, profile isolation and clear passed")
}

func testDirectMediaPolicyMySQL(t *testing.T, db *gorm.DB, repo *GormRepository, bindings *coremedia.ReferenceBindingService, newAsset func(uint64, coremedia.Purpose, coremedia.ProjectionStatus) coremedia.Asset) {
	ctx := context.Background()
	mediaRepo := coremedia.NewRepository(db)
	projections, _ := coremedia.NewReviewProjectionService(mediaRepo, nil)
	profile, _ := NewProfileUpdateService(repo, bindings, "https://media.example")
	cover, _ := NewProfileCoverService(repo, bindings, "https://media.example")
	directAsset := func(purpose coremedia.Purpose) coremedia.Asset {
		t.Helper()
		id, _ := coremedia.NewULID()
		key, version := "isolated/"+id, "version-"+id
		asset := coremedia.Asset{AssetPublicID: id, OwnerID: 7, Purpose: purpose, StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady, FinalKey: &key, FinalVersionID: &version, VerifiedMIME: "image/jpeg", ByteSize: 512, Width: 800, Height: 800, IdempotencyKey: id, RowVersion: 1}
		if err := db.Create(&asset).Error; err != nil {
			t.Fatal(err)
		}
		projection, created, err := projections.EnsureProjection(ctx, id)
		if err != nil || !created {
			t.Fatalf("create projection: %+v %v", projection, err)
		}
		want := coremedia.ProjectionPending
		if !purpose.ContentReviewRequired() {
			want = coremedia.ProjectionNotRequired
		}
		if projection.Status != want || projection.ReviewTaskID != nil || projection.AppliedDecisionVersion != 0 {
			t.Fatalf("policy projection: %+v", projection)
		}
		if want == coremedia.ProjectionNotRequired && projection.DecidedAt == nil {
			t.Fatal("policy decision time missing")
		}
		replay, again, err := projections.EnsureProjection(ctx, id)
		if err != nil || again || replay.ProjectionID != projection.ProjectionID || replay.Status != want {
			t.Fatalf("projection replay: %+v %v", replay, err)
		}
		return asset
	}
	for _, purpose := range []coremedia.Purpose{coremedia.PurposePost, coremedia.PurposeProduct, coremedia.PurposeDish, coremedia.PurposeMerchantLogo, coremedia.PurposeDeliveryProof, coremedia.PurposeDeliveryIM, coremedia.PurposeFeedback} {
		asset := directAsset(purpose)
		id, _ := coremedia.NewULID()
		if _, err := bindings.Reserve(ctx, coremedia.BindingReserveInput{OwnerID: 7, Purpose: purpose, Token: "m10_" + id, OrderedAssetPublicIDs: []string{asset.AssetPublicID}, ExpiresAt: time.Now().Add(time.Minute)}); err == nil {
			t.Fatal("pending cloud image accepted: " + string(purpose))
		}
	}
	first, second, header := directAsset(coremedia.PurposeAvatar), directAsset(coremedia.PurposeAvatar), directAsset(coremedia.PurposeProfileCover)
	if _, err := cover.Update(ctx, 7, header.AssetPublicID); err != nil {
		t.Fatal(err)
	}
	input := ProfileUpdateInput{Nickname: "直接保存", School: "校园", Campus: "东校区", AvatarAssetID: first.AssetPublicID}
	if _, err := profile.Update(ctx, 7, input); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.FindByID(ctx, 7)
	if err != nil || loaded.AvatarAssetID == nil || *loaded.AvatarAssetID != first.ID || loaded.CoverAssetID == nil || *loaded.CoverAssetID != header.ID {
		t.Fatalf("profile persistence: %+v %v", loaded, err)
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByAsset(ctx, first.AssetPublicID); err != nil {
		t.Fatal("new avatar is not publicly resolvable", err)
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByReference(ctx, coremedia.PurposeAvatar, "7", 0); err != nil {
		t.Fatal("avatar reference is not publicly resolvable", err)
	}
	if _, err := profile.Update(ctx, 7, input); err != nil {
		t.Fatal("avatar exact replay", err)
	}
	for _, invalid := range []coremedia.Asset{newAsset(8, coremedia.PurposeAvatar, coremedia.ProjectionNotRequired), header, newAsset(7, coremedia.PurposeAvatar, coremedia.ProjectionRejected), newAsset(7, coremedia.PurposeAvatar, coremedia.ProjectionPending)} {
		input.AvatarAssetID = invalid.AssetPublicID
		if _, err := profile.Update(ctx, 7, input); err == nil {
			t.Fatal("foreign, wrong-purpose or blocked avatar accepted")
		}
		if p, _, err := projections.EnsureProjection(ctx, invalid.AssetPublicID); err != nil || (invalid.OwnerID == 7 && invalid.Purpose == coremedia.PurposeAvatar && p.Status == coremedia.ProjectionNotRequired) {
			t.Fatal("old review state was reset", err)
		}
	}
	input.AvatarAssetID, input.Nickname = second.AssetPublicID, "不应保存"
	failing, _ := NewProfileUpdateService(repo, failCoverPublish{bindings}, "https://media.example")
	if _, err := failing.Update(ctx, 7, input); err == nil {
		t.Fatal("expected publish failure")
	}
	loaded, _ = repo.FindByID(ctx, 7)
	if *loaded.AvatarAssetID != first.ID || loaded.Nickname != "直接保存" {
		t.Fatal("failed update changed existing profile")
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByAsset(ctx, second.AssetPublicID); !errors.Is(err, coremedia.ErrNotFound) {
		t.Fatal("failed transaction published replacement")
	}
	input.Nickname = "新头像"
	if _, err := profile.Update(ctx, 7, input); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByAsset(ctx, first.AssetPublicID); !errors.Is(err, coremedia.ErrNotFound) {
		t.Fatal("replaced avatar remains public")
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByAsset(ctx, second.AssetPublicID); err != nil {
		t.Fatal(err)
	}
	input.AvatarAssetID, input.Nickname = "", "仅修改昵称"
	if _, err := profile.Update(ctx, 7, input); err != nil {
		t.Fatal(err)
	}
	loaded, _ = repo.FindByID(ctx, 7)
	if *loaded.AvatarAssetID != second.ID || *loaded.CoverAssetID != header.ID {
		t.Fatal("text edit cleared images")
	}
	proof := directAsset(coremedia.PurposeIdentityApplication)
	operationID, _ := coremedia.NewULID()
	applicationID, _ := coremedia.NewULID()
	token := "m10_" + operationID
	if _, err := bindings.Reserve(ctx, coremedia.BindingReserveInput{OwnerID: 7, Purpose: coremedia.PurposeIdentityApplication, Token: token, OrderedAssetPublicIDs: []string{proof.AssetPublicID}, ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := bindings.Commit(ctx, coremedia.BindingCommitInput{OwnerID: 7, Token: token, ReferenceType: coremedia.PurposeIdentityApplication, ReferenceID: applicationID, OrderedAssetPublicIDs: []string{proof.AssetPublicID}}); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaRepo.FindVisibleDisplayObjectByAsset(ctx, proof.AssetPublicID); !errors.Is(err, coremedia.ErrNotFound) {
		t.Fatal("private identity material became public")
	}
	snapshots, err := mediaRepo.FindGatewayAssetSnapshots(ctx, proof.AssetPublicID)
	if err != nil || len(snapshots) != 1 || snapshots[0].ParentVisibility != coremedia.ParentHidden || snapshots[0].ProjectionStatus != coremedia.ProjectionNotRequired {
		t.Fatal("identity reference lost privacy", err)
	}
	t.Log("real policy migration, new projection classification, pending content rejection, profile persistence/replacement/rollback and private identity binding passed")
}
