package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrExternalSignature     = errors.New("external provider signature invalid")
	ErrExternalBinding       = errors.New("external shop binding unavailable")
	ErrExternalAdapter       = errors.New("external order adapter unavailable")
	ErrExternalEventConflict = errors.New("external event identity conflict")
	ErrExternalLease         = errors.New("external event lease lost")
)

// ResolveExternalBinding must verify current shop authorization, merchant
// eligibility and the delivery feature policy from trusted server state.
// ErrExternalBinding means revoked/disabled; other errors may be retried.
// bindingKey is an opaque server-side configuration key, never a merchant ID
// accepted from a provider payload. Resolver and adapters must be concurrency-safe.
type ExternalBindingResolver interface {
	ResolveExternalBinding(context.Context, string) (ExternalShopBinding, error)
}

type ExternalIntakeConfig struct {
	Now         func() time.Time
	Lease       time.Duration
	MaxAttempts uint32
}

type ExternalReceipt struct {
	ID            uint64
	Status        string
	OrderPublicID string
	Reason        string
	Duplicate     bool
}

func intakeReceipt(row ExternalInbox, duplicate bool) ExternalReceipt {
	return ExternalReceipt{ID: row.ID, Status: row.Status, OrderPublicID: row.OrderPublicID, Reason: row.Reason, Duplicate: duplicate}
}

// Receive acknowledges durability only. Raw headers, signatures and original
// payload are deliberately not persisted. A delivery is not paid or dispatched.
func (s *ExternalIntake) Receive(ctx context.Context, bindingKey string, platform SourcePlatform, envelope ExternalOrderEnvelope) (ExternalReceipt, error) {
	adapter := s.adapters[platform]
	if adapter == nil {
		return ExternalReceipt{}, ErrExternalAdapter
	}
	if bindingKey == "" || strings.TrimSpace(bindingKey) != bindingKey || !validSourceText(bindingKey, 64) || len(envelope.Payload) == 0 || len(envelope.Payload) > 1024*1024 {
		return ExternalReceipt{}, ErrInvalidRecord
	}
	binding, err := s.bindings.ResolveExternalBinding(ctx, bindingKey)
	if err != nil {
		return ExternalReceipt{}, err
	}
	if binding.Platform != platform {
		return ExternalReceipt{}, ErrExternalBinding
	}
	now := s.now().UTC()
	envelope.ReceivedAt = now // provider input cannot choose the trusted receive time
	event, err := adapter.VerifyAndNormalize(ctx, envelope, binding)
	if err != nil {
		return ExternalReceipt{}, err
	}
	event, err = NormalizeExternalOrderEvent(platform, binding, event)
	if err != nil {
		return ExternalReceipt{}, err
	}
	event.OccurredAt = event.OccurredAt.UTC()
	if event.OccurredAt.Year() < 2000 || event.OccurredAt.After(now.Add(5*time.Minute)) {
		return ExternalReceipt{}, ErrInvalidRecord
	}
	event.Pickup = trimExternalContact(event.Pickup)
	event.Recipient = trimExternalContact(event.Recipient)
	event.Remark = strings.TrimSpace(event.Remark)
	if !validExternalContent(event) {
		return ExternalReceipt{}, ErrInvalidRecord
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return ExternalReceipt{}, err
	}
	digest := sha256.Sum256(payload)
	requestID, err := NewULID()
	if err != nil {
		return ExternalReceipt{}, err
	}
	row := ExternalInbox{BindingKey: bindingKey, MerchantUserID: binding.MerchantUserID,
		Platform: platform, ShopID: *event.Source.SourceShopID, SourceOrderID: *event.Source.SourceOrderID,
		EventID: event.EventID, EventType: event.EventType, OccurredAt: event.OccurredAt,
		Payload: payload, Fingerprint: hex.EncodeToString(digest[:]), RequestID: requestID,
		Status: "pending", CreatedAt: now, UpdatedAt: now}
	duplicate, err := s.insert(ctx, &row)
	return intakeReceipt(row, duplicate), err
}

func trimExternalContact(c Contact) Contact {
	return Contact{Name: strings.TrimSpace(c.Name), Phone: strings.TrimSpace(c.Phone), Address: strings.TrimSpace(c.Address)}
}

func validExternalContent(e ExternalOrderEvent) bool {
	for _, c := range []Contact{e.Pickup, e.Recipient} {
		if !validSourceText(c.Name, 64) || !validSourceText(c.Phone, 32) || !validSourceText(c.Address, 256) {
			return false
		}
		if e.EventType == ExternalOrderCreated && (c.Name == "" || c.Phone == "" || c.Address == "") {
			return false
		}
	}
	return validSourceText(e.Remark, 512)
}

// ProcessNext is safe to run in multiple workers. Expired leases are reclaimed.
// Binding revocation/changes stop intake; transient resolver/DB errors back off.
func (s *ExternalIntake) ProcessNext(ctx context.Context) (ExternalReceipt, bool, error) {
	row, found, err := s.claim(ctx)
	if err != nil || !found {
		return ExternalReceipt{}, found, err
	}
	if row.Attempts > s.maxAttempts {
		err := s.finish(ctx, &row, "deadletter", "lease_exhausted")
		return intakeReceipt(row, false), true, err
	}
	binding, err := s.bindings.ResolveExternalBinding(ctx, row.BindingKey)
	if errors.Is(err, ErrExternalBinding) || (err == nil && (binding.MerchantUserID != row.MerchantUserID || binding.Platform != row.Platform || binding.SourceShopID != row.ShopID)) {
		err = s.finish(ctx, &row, "needs_review", "binding_unavailable")
	} else if err == nil {
		err = s.process(ctx, &row)
	}
	if err != nil && !errors.Is(err, ErrExternalLease) {
		if failErr := s.fail(ctx, &row); failErr != nil {
			return intakeReceipt(row, false), true, failErr
		}
	}
	return intakeReceipt(row, false), true, err
}

// Run consumes until context cancellation. Receipts remain queryable even when
// a process stops. No environment switch or webhook is registered implicitly.
func (s *ExternalIntake) Run(ctx context.Context) error {
	for {
		_, found, err := s.ProcessNext(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if found && err == nil {
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
