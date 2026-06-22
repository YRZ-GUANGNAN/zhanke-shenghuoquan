package takeout

import (
	"context"
	"testing"
	"time"
)

type merchantRepositoryStub struct {
	created         *Merchant
	transitionCalls int
	lastOwner       *uint64
}

func (r *merchantRepositoryStub) CreateMerchant(_ context.Context, value *Merchant) error {
	copy := *value
	r.created = &copy
	value.ID = 71
	return nil
}

func (*merchantRepositoryStub) FindMerchant(context.Context, string) (Merchant, error) {
	return Merchant{ID: 71}, nil
}

func (r *merchantRepositoryStub) TransitionMerchantStatus(_ context.Context, publicID string, ownerID *uint64, version uint64, from, to MerchantStatus, at time.Time) (Merchant, error) {
	r.transitionCalls++
	r.lastOwner = ownerID
	return Merchant{ID: 71, MerchantPublicID: publicID, Status: to, RowVersion: version + 1, UpdatedAt: at}, nil
}

func TestCreateMerchantSetsFrozenInitialStateAndUTC(t *testing.T) {
	repository := &merchantRepositoryStub{}
	zone := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 6, 22, 16, 0, 0, 0, zone)
	service, err := NewMerchantService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	logo, _ := NewULID()
	value, err := service.CreateMerchant(context.Background(), CreateMerchantInput{
		OwnerUserID: 7, Name: "  Campus Kitchen  ", ContactPhone: " 13800000000 ",
		Location: " Building A ", BusinessHours: " 08:00-20:00 ", LogoAssetPublicID: &logo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.ID != 71 || !ValidULID(value.MerchantPublicID) || repository.created == nil {
		t.Fatalf("merchant was not persisted: %#v", value)
	}
	if value.Status != MerchantPendingReview || value.RowVersion != 1 || value.OwnerUserID != 7 || value.Name != "Campus Kitchen" {
		t.Fatalf("unexpected initial state: %#v", value)
	}
	if value.CreatedAt.Location() != time.UTC || !value.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("timestamp is not UTC: %v", value.CreatedAt)
	}
}

func TestServiceRejectsInvalidInputBeforeRepository(t *testing.T) {
	repository := &merchantRepositoryStub{}
	service, _ := NewMerchantService(repository, time.Now)
	if _, err := service.CreateMerchant(context.Background(), CreateMerchantInput{Name: "missing owner"}); err != ErrInvalidRecord {
		t.Fatalf("invalid create returned %v", err)
	}
	publicID, _ := NewULID()
	if _, err := service.Suspend(context.Background(), publicID, MerchantPendingReview, 1); err != ErrInvalidTransition {
		t.Fatalf("illegal transition returned %v", err)
	}
	if _, err := service.Rest(context.Background(), publicID, 0, 1); err != ErrInvalidRecord {
		t.Fatalf("zero owner returned %v", err)
	}
	if _, err := service.FindMerchant(context.Background(), "not-a-ulid"); err != ErrInvalidULID {
		t.Fatalf("invalid lookup id returned %v", err)
	}
	if _, err := service.Approve(context.Background(), "not-a-ulid", 1); err != ErrInvalidULID {
		t.Fatalf("invalid transition id returned %v", err)
	}
	if repository.transitionCalls != 0 {
		t.Fatal("repository called for invalid input")
	}
}

func TestServiceDelegatesOwnerAndAdminTransitions(t *testing.T) {
	repository := &merchantRepositoryStub{}
	service, _ := NewMerchantService(repository, time.Now)
	publicID, _ := NewULID()
	if _, err := service.Rest(context.Background(), publicID, 88, 3); err != nil {
		t.Fatal(err)
	}
	if repository.lastOwner == nil || *repository.lastOwner != 88 {
		t.Fatal("owner transition omitted owner fence")
	}
	if _, err := service.Suspend(context.Background(), publicID, MerchantResting, 4); err != nil {
		t.Fatal(err)
	}
	if repository.lastOwner != nil {
		t.Fatal("admin transition unexpectedly carried owner fence")
	}
}

func TestNewMerchantServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewMerchantService(nil, time.Now); err != ErrInvalidRecord {
		t.Fatalf("nil repository returned %v", err)
	}
}
