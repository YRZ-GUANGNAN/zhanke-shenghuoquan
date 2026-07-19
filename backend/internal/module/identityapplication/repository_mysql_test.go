package identityapplication

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/takeout"
	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	mysqlAssetA = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	mysqlAssetB = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	mysqlAssetC = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	mysqlAssetD = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
)

type failAfterTransactionalMediaCommit struct {
	ApplicationMedia
	err error
}

func TestGormRepositoryFindGrantAgainstMySQL(t *testing.T) {
	db := requireIdentityApplicationMySQL(t)
	now := time.Now().UTC()
	for _, grant := range []IdentityGrant{
		{UserID: 901, Role: RoleMerchant, Status: GrantActive, GrantedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now},
		{UserID: 902, Role: RoleRider, Status: GrantSuspended, GrantedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&grant).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(db)
	active, found, err := repository.FindGrant(context.Background(), 901, RoleMerchant)
	if err != nil || !found || active.Status != GrantActive {
		t.Fatalf("active=%+v found=%v err=%v", active, found, err)
	}
	suspended, found, err := repository.FindGrant(context.Background(), 902, RoleRider)
	if err != nil || !found || suspended.Status != GrantSuspended {
		t.Fatalf("suspended=%+v found=%v err=%v", suspended, found, err)
	}
	missing, found, err := repository.FindGrant(context.Background(), 903, RoleMerchant)
	if err != nil || found || missing.ID != 0 {
		t.Fatalf("missing=%+v found=%v err=%v", missing, found, err)
	}
}

func (m *failAfterTransactionalMediaCommit) EnsureCommittedInTransaction(ctx context.Context, tx *gorm.DB, ownerID uint64, reservation MediaReservation, applicationPublicID string) error {
	if err := m.ApplicationMedia.EnsureCommittedInTransaction(ctx, tx, ownerID, reservation, applicationPublicID); err != nil {
		return err
	}
	return m.err
}

func TestGormRepositoryMediaBindingAtomicityAgainstMySQL(t *testing.T) {
	db := requireIdentityApplicationMySQL(t)
	now := time.Now().UTC()
	for index, fixture := range []struct {
		ownerID uint64
		assetID string
		projID  string
	}{
		{ownerID: 71, assetID: mysqlAssetA, projID: "01ARZ3NDEKTSV4RRFFQ69G5FB2"},
		{ownerID: 71, assetID: mysqlAssetB, projID: "01ARZ3NDEKTSV4RRFFQ69G5FB3"},
		{ownerID: 71, assetID: mysqlAssetC, projID: "01ARZ3NDEKTSV4RRFFQ69G5FB4"},
		{ownerID: 72, assetID: mysqlAssetD, projID: "01ARZ3NDEKTSV4RRFFQ69G5FB5"},
	} {
		seedIdentityProofAsset(t, db, fixture.ownerID, fixture.assetID, fixture.projID, now.Add(time.Duration(index)*time.Millisecond))
	}
	bindingService, err := coremedia.NewReferenceBindingService(coremedia.NewReferenceBindingRepository(db), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	applicationMedia, err := NewApplicationMedia(bindingService, coremedia.NewRepository(db), &proofSignerStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	service := NewService(repository, applicationMedia, func() time.Time { return now })
	input := SubmitInput{RealName: "原子用户", Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetA, mysqlAssetB}}
	first, err := service.Submit(context.Background(), 71, RoleRider, input)
	if err != nil {
		t.Fatal(err)
	}
	assertExactLiveMySQLProofs(t, db, first.ApplicationPublicID, []string{mysqlAssetA, mysqlAssetB})

	markMySQLApplicationRejected(t, db, first.ID, 1, 2, now.Add(time.Minute))
	now = now.Add(2 * time.Minute)
	input.AssetPublicIDs = []string{mysqlAssetB, mysqlAssetC}
	input.ExpectedRowVersion = 2
	second, err := service.Submit(context.Background(), 71, RoleRider, input)
	if err != nil {
		t.Fatal(err)
	}
	assertExactLiveMySQLProofs(t, db, first.ApplicationPublicID, []string{mysqlAssetB, mysqlAssetC})
	assertMySQLProofRevoked(t, db, first.ApplicationPublicID, mysqlAssetA)

	markMySQLApplicationRejected(t, db, first.ID, second.RowVersion, 4, now.Add(time.Minute))
	now = now.Add(2 * time.Minute)
	input.AssetPublicIDs = []string{mysqlAssetC}
	input.ExpectedRowVersion = 4
	if _, err := service.Submit(context.Background(), 71, RoleRider, input); err != nil {
		t.Fatal(err)
	}
	assertExactLiveMySQLProofs(t, db, first.ApplicationPublicID, []string{mysqlAssetC})
	assertMySQLProofRevoked(t, db, first.ApplicationPublicID, mysqlAssetA)
	assertMySQLProofRevoked(t, db, first.ApplicationPublicID, mysqlAssetB)
	assertMySQLCount(t, db, 3, &ApplicationEvent{}, "application_id = ?", first.ID)

	beforeApplications := mysqlModelCount(t, db, &Application{})
	beforeAssets := mysqlModelCount(t, db, &ApplicationAsset{})
	beforeEvents := mysqlModelCount(t, db, &ApplicationEvent{})
	failingMedia := &failAfterTransactionalMediaCommit{ApplicationMedia: applicationMedia, err: coremedia.ErrBindingMismatch}
	failingService := NewService(repository, failingMedia, func() time.Time { return now })
	failedInput := SubmitInput{RealName: "回滚用户", Phone: "13900139000", AssetPublicIDs: []string{mysqlAssetD}}
	if _, err := failingService.Submit(context.Background(), 72, RoleRider, failedInput); !errors.Is(err, ErrProofMediaConflict) {
		t.Fatalf("transactional media failure err=%v", err)
	}
	if got := mysqlModelCount(t, db, &Application{}); got != beforeApplications {
		t.Fatalf("failed commit left application rows: got=%d want=%d", got, beforeApplications)
	}
	if got := mysqlModelCount(t, db, &ApplicationAsset{}); got != beforeAssets {
		t.Fatalf("failed commit left application asset rows: got=%d want=%d", got, beforeAssets)
	}
	if got := mysqlModelCount(t, db, &ApplicationEvent{}); got != beforeEvents {
		t.Fatalf("failed commit left application event rows: got=%d want=%d", got, beforeEvents)
	}
	var failedReferences int64
	if err := db.Model(&coremedia.AssetReference{}).Joins("JOIN media_assets a ON a.id = media_asset_references.asset_id").
		Where("a.owner_id = ?", 72).Count(&failedReferences).Error; err != nil {
		t.Fatal(err)
	}
	if failedReferences != 0 {
		t.Fatalf("failed transaction left media references: %d", failedReferences)
	}
	var failedReservation struct {
		Token     string     `gorm:"column:reservation_token"`
		ExpiresAt *time.Time `gorm:"column:reservation_expires_at"`
	}
	if err := db.Table("media_assets").Select("reservation_token, reservation_expires_at").
		Where("asset_public_id = ?", mysqlAssetD).Take(&failedReservation).Error; err != nil {
		t.Fatal(err)
	}
	if failedReservation.Token != "" || failedReservation.ExpiresAt != nil {
		t.Fatalf("failed transaction reservation was not cancelled: %+v", failedReservation)
	}
}

func seedIdentityProofAsset(t *testing.T, db *gorm.DB, ownerID uint64, assetPublicID, projectionID string, now time.Time) {
	t.Helper()
	finalKey := "final/identity/" + assetPublicID + ".webp"
	finalVersion := "version-" + assetPublicID
	asset := coremedia.Asset{
		AssetPublicID: assetPublicID, OwnerID: ownerID, Purpose: coremedia.PurposeIdentityApplication,
		StorageStage: coremedia.StorageStageFinal, ReviewStatus: coremedia.ReviewApproved,
		TechnicalStatus: coremedia.TechnicalFinalReady, VerifiedMIME: "image/webp", ByteSize: 1024,
		Width: 640, Height: 640, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		AppliedDecisionVersion: 1, ProjectedDecision: coremedia.ReviewApproved,
		IdempotencyKey: "identity-proof-" + assetPublicID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	projection := coremedia.ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: coremedia.ProjectionApproved,
		AppliedDecisionVersion: 1, FailureCode: "", RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
}

func markMySQLApplicationRejected(t *testing.T, db *gorm.DB, applicationID, expectedVersion, nextVersion uint64, now time.Time) {
	t.Helper()
	result := db.Model(&Application{}).Where("id = ? AND status = ? AND row_version = ?", applicationID, StatusPending, expectedVersion).
		Updates(map[string]any{"status": StatusRejected, "review_reason": "补充证明", "row_version": nextVersion, "updated_at": now})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("mark rejected rows=%d err=%v", result.RowsAffected, result.Error)
	}
}

func assertExactLiveMySQLProofs(t *testing.T, db *gorm.DB, applicationPublicID string, wantAssetPublicIDs []string) {
	t.Helper()
	type liveReference struct {
		AssetPublicID string                     `gorm:"column:asset_public_id"`
		Slot          uint16                     `gorm:"column:slot"`
		Status        coremedia.ReferenceStatus  `gorm:"column:status"`
		Visibility    coremedia.ParentVisibility `gorm:"column:parent_visibility"`
		Fence         *string                    `gorm:"column:revocation_fence"`
	}
	var references []liveReference
	if err := db.Table("media_asset_references AS r").
		Select("a.asset_public_id, r.slot, r.status, r.parent_visibility, r.revocation_fence").
		Joins("JOIN media_assets AS a ON a.id = r.asset_id").
		Where("r.reference_type = ? AND r.reference_id = ? AND r.status IN ?", coremedia.PurposeIdentityApplication,
			applicationPublicID, []coremedia.ReferenceStatus{coremedia.ReferencePending, coremedia.ReferenceActive}).
		Order("r.slot ASC, r.id ASC").Scan(&references).Error; err != nil {
		t.Fatal(err)
	}
	if len(references) != len(wantAssetPublicIDs) {
		t.Fatalf("live references=%+v want=%v", references, wantAssetPublicIDs)
	}
	for index, reference := range references {
		if reference.AssetPublicID != wantAssetPublicIDs[index] || reference.Slot != uint16(index) ||
			reference.Visibility != coremedia.ParentHidden || reference.Fence != nil ||
			(reference.Status != coremedia.ReferencePending && reference.Status != coremedia.ReferenceActive) {
			t.Fatalf("live reference[%d]=%+v want asset=%s slot=%d", index, reference, wantAssetPublicIDs[index], index)
		}
	}
}

func assertMySQLProofRevoked(t *testing.T, db *gorm.DB, applicationPublicID, assetPublicID string) {
	t.Helper()
	var references []coremedia.AssetReference
	if err := db.Model(&coremedia.AssetReference{}).Joins("JOIN media_assets a ON a.id = media_asset_references.asset_id").
		Where("media_asset_references.reference_type = ? AND media_asset_references.reference_id = ? AND a.asset_public_id = ?",
			coremedia.PurposeIdentityApplication, applicationPublicID, assetPublicID).Find(&references).Error; err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 || references[0].Status != coremedia.ReferenceRevoked || references[0].RevocationFence == nil ||
		references[0].ParentVisibility != coremedia.ParentHidden {
		t.Fatalf("asset %s references=%+v", assetPublicID, references)
	}
}

func mysqlModelCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestGormRepositoryMySQLContract(t *testing.T) {
	db := requireIdentityApplicationMySQL(t)
	repository := NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 123000000, time.UTC)
	firstSubmittedAt := now
	service := NewService(repository, &submissionMediaStub{}, func() time.Time { return now })

	first, err := service.Submit(ctx, 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetA},
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if first.Status != StatusPending || first.RowVersion != 1 || first.UserID != "41" {
		t.Fatalf("first=%+v", first)
	}
	assertMySQLCount(t, db, 1, &Application{}, "user_id = ? AND role = ?", 41, RoleRider)
	assertMySQLCount(t, db, 1, &ApplicationAsset{}, "application_id = ?", first.ID)
	assertMySQLCount(t, db, 1, &ApplicationEvent{}, "application_id = ? AND event_type = ?", first.ID, EventSubmitted)
	var submittedEvent ApplicationEvent
	if err := db.Where("application_id = ?", first.ID).Take(&submittedEvent).Error; err != nil {
		t.Fatal(err)
	}
	if submittedEvent.ActorUserID != 41 || submittedEvent.Status != StatusPending || len(submittedEvent.SnapshotJSON) == 0 {
		t.Fatalf("submitted event=%+v", submittedEvent)
	}

	reviewerID := uint64(7)
	reviewedAt := now.Add(30 * time.Minute)
	result := db.Model(&Application{}).Where("id = ? AND row_version = ?", first.ID, 1).Updates(map[string]any{
		"status":           StatusRejected,
		"review_reason":    "资料不清晰",
		"reviewer_user_id": reviewerID,
		"reviewed_at":      reviewedAt,
		"row_version":      2,
		"updated_at":       reviewedAt,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("mark rejected: rows=%d err=%v", result.RowsAffected, result.Error)
	}

	now = now.Add(time.Hour)
	resubmitted, err := service.Submit(ctx, 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetB, mysqlAssetC}, ExpectedRowVersion: 2,
	})
	if err != nil {
		t.Fatalf("resubmit rejected: %v", err)
	}
	if resubmitted.RowVersion != 3 || resubmitted.Status != StatusPending || resubmitted.ReviewerUserID != nil ||
		resubmitted.ReviewedAt != nil || len(resubmitted.AssetPublicIDs) != 2 {
		t.Fatalf("resubmitted=%+v", resubmitted)
	}
	assertMySQLCount(t, db, 2, &ApplicationAsset{}, "application_id = ?", first.ID)
	assertMySQLCount(t, db, 2, &ApplicationEvent{}, "application_id = ?", first.ID)

	staleBase := now.Add(15 * time.Minute)
	result = db.Model(&Application{}).Where("id = ? AND row_version = ?", first.ID, 3).Updates(map[string]any{
		"status":           StatusRejected,
		"review_reason":    "仍需补充",
		"reviewer_user_id": reviewerID,
		"reviewed_at":      staleBase,
		"row_version":      4,
		"updated_at":       staleBase,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("prepare stale CAS: rows=%d err=%v", result.RowsAffected, result.Error)
	}
	stale, found, err := repository.FindCurrent(ctx, 41, RoleRider)
	if err != nil || !found {
		t.Fatalf("load stale candidate: found=%v err=%v", found, err)
	}
	stale.Status = StatusPending
	stale.RowVersion = 4
	stale.SubmittedAt = staleBase.Add(time.Hour)
	stale.UpdatedAt = stale.SubmittedAt
	if err := repository.SaveSubmission(ctx, &stale, []string{mysqlAssetD}, EventResubmitted, 3, func(context.Context, *gorm.DB) error { return nil }); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale CAS err=%v", err)
	}
	unchanged, found, err := repository.FindCurrent(ctx, 41, RoleRider)
	if err != nil || !found || unchanged.Status != StatusRejected || unchanged.RowVersion != 4 || unchanged.ReviewReason != "仍需补充" {
		t.Fatalf("stale CAS changed application: found=%v value=%+v err=%v", found, unchanged, err)
	}
	if len(unchanged.Assets) != 2 || unchanged.Assets[0].AssetPublicID != mysqlAssetB || unchanged.Assets[1].AssetPublicID != mysqlAssetC {
		t.Fatalf("stale CAS changed assets: %+v", unchanged.Assets)
	}
	assertMySQLCount(t, db, 2, &ApplicationEvent{}, "application_id = ?", first.ID)

	rollbackTime := now.Add(2 * time.Hour)
	rollbackApplication := Application{
		ApplicationPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FB0",
		UserID:              99,
		Role:                RoleRider,
		Status:              StatusPending,
		RealName:            "回滚用户",
		Phone:               "13900139000",
		SubmittedAt:         rollbackTime,
		RowVersion:          1,
		CreatedAt:           rollbackTime,
		UpdatedAt:           rollbackTime,
	}
	if err := repository.SaveSubmission(ctx, &rollbackApplication, []string{mysqlAssetA, mysqlAssetA}, EventSubmitted, 0, func(context.Context, *gorm.DB) error { return nil }); err == nil {
		t.Fatal("duplicate asset transaction unexpectedly succeeded")
	}
	assertMySQLCount(t, db, 0, &Application{}, "user_id = ?", 99)
	assertMySQLCount(t, db, 0, &ApplicationAsset{}, "application_id = ?", rollbackApplication.ID)
	assertMySQLCount(t, db, 0, &ApplicationEvent{}, "actor_user_id = ?", 99)

	now = now.Add(3 * time.Hour)
	submitMySQLApplication(t, service, 42, RoleRider, "骑手甲")
	now = now.Add(time.Minute)
	approved := submitMySQLApplication(t, service, 43, RoleRider, "骑手乙")
	if err := db.Model(&Application{}).Where("id = ?", approved.ID).Updates(map[string]any{
		"status": StatusApproved, "row_version": 2, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	submitMySQLApplication(t, service, 44, RoleMerchant, "商家甲")
	now = now.Add(time.Minute)
	latestPendingRider := submitMySQLApplication(t, service, 45, RoleRider, "骑手丙")

	pageOne, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleRider, Status: StatusPending, Page: 1, PageSize: 1})
	if err != nil || total != 2 || len(pageOne) != 1 || pageOne[0].ID != latestPendingRider.ID || len(pageOne[0].Assets) != 1 {
		t.Fatalf("rider pending page one: total=%d values=%+v err=%v", total, pageOne, err)
	}
	pageTwo, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleRider, Status: StatusPending, Page: 2, PageSize: 1})
	if err != nil || total != 2 || len(pageTwo) != 1 || pageTwo[0].UserID != 42 {
		t.Fatalf("rider pending page two: total=%d values=%+v err=%v", total, pageTwo, err)
	}
	merchants, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleMerchant, Status: StatusPending, Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(merchants) != 1 || merchants[0].UserID != 44 {
		t.Fatalf("merchant pending filter: total=%d values=%+v err=%v", total, merchants, err)
	}

	// Make physical row order disagree with sort_order, and give the two real
	// submission events the same timestamp. The extra earlier event proves that
	// FindDetail applies created_at first and id as the deterministic tie-breaker.
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 0).Update("sort_order", 9).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 1).Update("sort_order", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 9).Update("sort_order", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationEvent{}).Where("application_id = ?", first.ID).Update("created_at", firstSubmittedAt).Error; err != nil {
		t.Fatal(err)
	}
	earlyEvent := ApplicationEvent{
		EventPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FB1",
		ApplicationID: first.ID,
		ActorUserID:   7,
		EventType:     EventRejected,
		Status:        StatusRejected,
		Reason:        "排序哨兵",
		SnapshotJSON:  json.RawMessage(`{}`),
		CreatedAt:     firstSubmittedAt.Add(-time.Hour),
	}
	if err := db.Create(&earlyEvent).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := repository.FindDetail(ctx, first.ApplicationPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.UserID != "41" || detail.ReviewerUserID == nil || *detail.ReviewerUserID != "7" {
		t.Fatalf("detail string ids=%+v", detail.ApplicationView)
	}
	if len(detail.AssetPublicIDs) != 2 || detail.AssetPublicIDs[0] != mysqlAssetC || detail.AssetPublicIDs[1] != mysqlAssetB {
		t.Fatalf("detail assets not sorted by sort_order: %+v", detail.AssetPublicIDs)
	}
	if len(detail.Events) != 3 || detail.Events[0].EventType != EventRejected || detail.Events[0].ActorUserID != "7" ||
		detail.Events[1].EventType != EventSubmitted || detail.Events[2].EventType != EventResubmitted ||
		detail.Events[1].ActorUserID != "41" || !detail.Events[1].CreatedAt.Equal(detail.Events[2].CreatedAt) {
		t.Fatalf("detail events not sorted by created_at/id: %+v", detail.Events)
	}
}

func TestGormRepositoryDecisionAtomicityAgainstMySQL(t *testing.T) {
	db := requireIdentityApplicationMySQL(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	submissionService := NewService(repository, &submissionMediaStub{}, func() time.Time { return now })

	riderApplication := submitMySQLApplication(t, submissionService, 81, RoleRider, "真实骑手")
	provisioner := ApprovalProvisionerFunc(func(ctx context.Context, tx *gorm.DB, approved ApprovedIdentity) error {
		riders, err := takeout.NewRiderIdentityService(takeout.NewRepository(tx), func() time.Time { return now })
		if err != nil {
			return err
		}
		_, err = riders.RegisterApproved(ctx, approved.UserID, approved.RealName, approved.Phone)
		return err
	})
	decisionService := NewService(repository, &submissionMediaStub{}, func() time.Time { return now }, provisioner)
	if _, err := decisionService.Approve(ctx, 81, riderApplication.ApplicationPublicID, riderApplication.RowVersion); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("self review err=%v", err)
	}
	approved, err := decisionService.Approve(ctx, 7, riderApplication.ApplicationPublicID, riderApplication.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != StatusApproved || approved.RowVersion != 2 || approved.ReviewerUserID == nil || *approved.ReviewerUserID != "7" {
		t.Fatalf("approved=%+v", approved)
	}
	assertMySQLCount(t, db, 1, &IdentityGrant{}, "user_id = ? AND role = ? AND status = ? AND source_application_id = ?", 81, RoleRider, GrantActive, riderApplication.ID)
	assertMySQLCount(t, db, 1, &takeout.Rider{}, "rider_user_id = ? AND status = ?", 81, takeout.RiderActive)
	assertMySQLCount(t, db, 1, &ApplicationEvent{}, "application_id = ? AND event_type = ? AND actor_user_id = ?", riderApplication.ID, EventApproved, 7)
	if _, err := decisionService.Approve(ctx, 8, riderApplication.ApplicationPublicID, riderApplication.RowVersion); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("repeated approval err=%v", err)
	}

	now = now.Add(time.Minute)
	failingApplication := submitMySQLApplication(t, submissionService, 82, RoleRider, "回滚骑手")
	failingService := NewService(repository, &submissionMediaStub{}, func() time.Time { return now }, ApprovalProvisionerFunc(
		func(ctx context.Context, tx *gorm.DB, approved ApprovedIdentity) error {
			riders, err := takeout.NewRiderIdentityService(takeout.NewRepository(tx), func() time.Time { return now })
			if err != nil {
				return err
			}
			if _, err := riders.RegisterApproved(ctx, approved.UserID, approved.RealName, approved.Phone); err != nil {
				return err
			}
			return errors.New("force decision rollback")
		},
	))
	if _, err := failingService.Approve(ctx, 7, failingApplication.ApplicationPublicID, failingApplication.RowVersion); err == nil {
		t.Fatal("failing provisioner was accepted")
	}
	current, found, err := repository.FindCurrent(ctx, 82, RoleRider)
	if err != nil || !found || current.Status != StatusPending || current.RowVersion != 1 {
		t.Fatalf("failed decision changed application: found=%v value=%+v err=%v", found, current, err)
	}
	assertMySQLCount(t, db, 0, &IdentityGrant{}, "user_id = ? AND role = ?", 82, RoleRider)
	assertMySQLCount(t, db, 0, &takeout.Rider{}, "rider_user_id = ?", 82)
	assertMySQLCount(t, db, 0, &ApplicationEvent{}, "application_id = ? AND event_type = ?", failingApplication.ID, EventApproved)

	now = now.Add(time.Minute)
	merchantApplication := submitMySQLApplication(t, submissionService, 83, RoleMerchant, "商家负责人")
	merchantApproved, err := decisionService.Approve(ctx, 7, merchantApplication.ApplicationPublicID, merchantApplication.RowVersion)
	if err != nil || merchantApproved.Status != StatusApproved {
		t.Fatalf("merchant approval=%+v err=%v", merchantApproved, err)
	}
	assertMySQLCount(t, db, 1, &IdentityGrant{}, "user_id = ? AND role = ? AND source_application_id = ?", 83, RoleMerchant, merchantApplication.ID)
	assertMySQLCount(t, db, 0, &takeout.Rider{}, "rider_user_id = ?", 83)

	now = now.Add(time.Minute)
	rejectedApplication := submitMySQLApplication(t, submissionService, 84, RoleMerchant, "拒绝商家")
	rejected, err := decisionService.Reject(ctx, 7, rejectedApplication.ApplicationPublicID, rejectedApplication.RowVersion, " 证明模糊 ")
	if err != nil || rejected.Status != StatusRejected || rejected.ReviewReason != "证明模糊" || rejected.RowVersion != 2 {
		t.Fatalf("rejected=%+v err=%v", rejected, err)
	}
	assertMySQLCount(t, db, 0, &IdentityGrant{}, "user_id = ? AND role = ?", 84, RoleMerchant)
	assertMySQLCount(t, db, 1, &ApplicationEvent{}, "application_id = ? AND event_type = ? AND actor_user_id = ?", rejectedApplication.ID, EventRejected, 7)
}

func submitMySQLApplication(t *testing.T, service *Service, userID uint64, role Role, name string) ApplicationView {
	t.Helper()
	input := SubmitInput{RealName: name, Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetA}}
	if role == RoleMerchant {
		input.MerchantName = "测试商家"
		input.MerchantAddress = "测试地址"
	}
	value, err := service.Submit(context.Background(), userID, role, input)
	if err != nil {
		t.Fatalf("submit user %d role %s: %v", userID, role, err)
	}
	return value
}

func assertMySQLCount(t *testing.T, db *gorm.DB, want int64, model any, query string, args ...any) {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("count=%d want=%d model=%T query=%s", count, want, model, query)
	}
}

func requireIdentityApplicationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("PENDING: set TEST_MYSQL_DSN to run identity application repository tests in an independent temporary MySQL database")
	}
	serverConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse TEST_MYSQL_DSN: %v", err)
	}
	serverConfig.DBName = ""
	serverConfig.MultiStatements = false
	serverConfig.ParseTime = true
	serverConfig.Loc = time.UTC
	serverDB, err := sql.Open("mysql", serverConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL server connection: %v", err)
	}
	if err := serverDB.Ping(); err != nil {
		_ = serverDB.Close()
		t.Fatalf("connect TEST_MYSQL_DSN server: %v", err)
	}
	var version string
	if err := serverDB.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		_ = serverDB.Close()
		t.Fatalf("read MySQL version: %v", err)
	}
	if !strings.HasPrefix(version, "8.") {
		_ = serverDB.Close()
		t.Skipf("PENDING: identity application repository contract requires MySQL 8, got %s", version)
	}

	databaseName := fmt.Sprintf("identity_application_repo_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := serverDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = serverDB.Close()
		t.Skipf("PENDING: cannot create independent temporary database; shared campus_circle fallback is forbidden: %v", err)
	}
	databaseConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatal(err)
	}
	databaseConfig.DBName = databaseName
	databaseConfig.MultiStatements = false
	databaseConfig.ParseTime = true
	databaseConfig.Loc = time.UTC
	databaseSQL, err := sql.Open("mysql", databaseConfig.FormatDSN())
	if err != nil {
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatal(err)
	}
	if err := databaseSQL.Ping(); err != nil {
		_ = databaseSQL.Close()
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatalf("connect temporary database: %v", err)
	}
	t.Cleanup(func() {
		if err := databaseSQL.Close(); err != nil {
			t.Errorf("close temporary identity application database: %v", err)
		}
		if _, err := serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop temporary identity application database: %v", err)
		}
		if err := serverDB.Close(); err != nil {
			t.Errorf("close MySQL server connection: %v", err)
		}
	})

	for _, statement := range identityApplicationRepositoryDDL() {
		if _, err := databaseSQL.Exec(statement); err != nil {
			t.Fatalf("create identity application test schema: %v\n%s", err, statement)
		}
	}
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: databaseSQL}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open GORM temporary database: %v", err)
	}
	if err := db.AutoMigrate(&IdentityGrant{}, &takeout.Rider{}, &coremedia.Asset{}, &coremedia.ReviewProjection{}, &coremedia.AssetReference{}); err != nil {
		t.Fatalf("create media binding test schema: %v", err)
	}
	if err := db.Exec(`ALTER TABLE media_assets
		ADD COLUMN public_id CHAR(32) NULL,
		ADD COLUMN reservation_token CHAR(32) NOT NULL DEFAULT '',
		ADD COLUMN reservation_expires_at DATETIME(3) NULL,
		ADD INDEX idx_media_reservation (reservation_token)`).Error; err != nil {
		t.Fatalf("complete media binding test schema: %v", err)
	}
	return db
}

func identityApplicationRepositoryDDL() []string {
	return []string{
		`CREATE TABLE identity_applications (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			application_public_id CHAR(26) NOT NULL,
			user_id BIGINT UNSIGNED NOT NULL,
			role VARCHAR(16) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			real_name VARCHAR(64) NOT NULL,
			phone VARCHAR(32) NOT NULL,
			merchant_name VARCHAR(120) NOT NULL DEFAULT '',
			merchant_address VARCHAR(240) NOT NULL DEFAULT '',
			review_reason VARCHAR(500) NOT NULL DEFAULT '',
			reviewer_user_id BIGINT UNSIGNED NULL,
			reviewed_at DATETIME(3) NULL,
			submitted_at DATETIME(3) NOT NULL,
			row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL,
			updated_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_public_id (application_public_id),
			UNIQUE KEY uq_identity_application_user_role (user_id, role),
			KEY idx_identity_application_status_submitted (status, submitted_at, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE identity_application_assets (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			application_id BIGINT UNSIGNED NOT NULL,
			asset_public_id CHAR(26) NOT NULL,
			sort_order SMALLINT UNSIGNED NOT NULL,
			created_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_asset_slot (application_id, sort_order),
			UNIQUE KEY uq_identity_application_asset (application_id, asset_public_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE identity_application_events (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			event_public_id CHAR(26) NOT NULL,
			application_id BIGINT UNSIGNED NOT NULL,
			actor_user_id BIGINT UNSIGNED NOT NULL,
			event_type VARCHAR(16) NOT NULL,
			status VARCHAR(16) NOT NULL,
			reason VARCHAR(500) NOT NULL DEFAULT '',
			snapshot_json JSON NOT NULL,
			created_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_event_public_id (event_public_id),
			KEY idx_identity_application_event_application (application_id, created_at, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
}
