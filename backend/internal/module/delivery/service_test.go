package delivery

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingRepository struct {
	created    *Order
	transition transitionCall
	found      Order
	err        error
}

type transitionCall struct {
	publicID           string
	expectedRowVersion uint64
	from, to           Status
	riderUserID        *uint64
	at                 time.Time
}

func (r *recordingRepository) Create(_ context.Context, value *Order) error {
	copy := *value
	r.created = &copy
	return r.err
}

func (r *recordingRepository) Find(_ context.Context, _ string) (Order, error) {
	return r.found, r.err
}

func (r *recordingRepository) Transition(
	_ context.Context,
	publicID string,
	expectedRowVersion uint64,
	from, to Status,
	riderUserID *uint64,
	at time.Time,
) (Order, error) {
	r.transition = transitionCall{
		publicID: publicID, expectedRowVersion: expectedRowVersion,
		from: from, to: to, riderUserID: riderUserID, at: at,
	}
	return r.found, r.err
}

func TestServiceCreatesSnapshotAndDelegatesTransitions(t *testing.T) {
	repository := &recordingRepository{}
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	service, err := NewService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		MerchantUserID:   991001,
		Pickup:           Contact{Name: " Merchant ", Phone: " 13800000001 ", Address: " Gate 1 "},
		Recipient:        Contact{Name: " Student ", Phone: " 13800000002 ", Address: " Dorm 1 "},
		Remark:           " test ",
		DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created == nil || order.Status != StatusPendingFeePayment || order.RowVersion != 1 ||
		order.PickupContactName != "Merchant" || order.RecipientName != "Student" ||
		order.CreatedAt.Location() != time.UTC {
		t.Fatalf("created order=%#v repository=%#v", order, repository.created)
	}

	repository.found = order
	if _, err := service.AssignRider(context.Background(), order.DeliveryOrderPublicID, 2, 7001); err != nil {
		t.Fatal(err)
	}
	if repository.transition.from != StatusReadyForClaim || repository.transition.to != StatusClaimed ||
		repository.transition.riderUserID == nil || *repository.transition.riderUserID != 7001 ||
		repository.transition.expectedRowVersion != 2 || repository.transition.at.Location() != time.UTC {
		t.Fatalf("transition=%#v", repository.transition)
	}
}

func TestServiceValidationFailsClosed(t *testing.T) {
	repository := &recordingRepository{}
	service, _ := NewService(repository, time.Now)
	validID := "01J00000000000000000000991"
	cases := []struct {
		name string
		call func() error
		want error
	}{
		{"invalid ULID", func() error {
			_, err := service.MarkFeePaid(context.Background(), "bad", 1)
			return err
		}, ErrInvalidULID},
		{"zero version", func() error {
			_, err := service.MarkFeePaid(context.Background(), validID, 0)
			return err
		}, ErrInvalidRecord},
		{"zero rider", func() error {
			_, err := service.AssignRider(context.Background(), validID, 1, 0)
			return err
		}, ErrRiderRequired},
		{"invalid release source", func() error {
			_, err := service.ReleaseToPool(context.Background(), validID, StatusPickedUp, 1, 1)
			return err
		}, ErrInvalidTransition},
		{"terminal cancel", func() error {
			_, err := service.Cancel(context.Background(), validID, StatusDelivered, 1)
			return err
		}, ErrInvalidTransition},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, test.want) {
				t.Fatalf("got %v want %v", err, test.want)
			}
		})
	}
	if repository.created != nil || repository.transition.publicID != "" {
		t.Fatal("invalid input reached repository")
	}
}
