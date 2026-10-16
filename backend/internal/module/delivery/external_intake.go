package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
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

type ExternalIntake struct {
	db          *gorm.DB
	bindings    ExternalBindingResolver
	fees        FeePolicyProvider
	adapters    map[SourcePlatform]ExternalOrderAdapter
	now         func() time.Time
	lease       time.Duration
	maxAttempts uint32
}

// NewExternalIntake builds the durable workflow, not an HTTP webhook. Official
// provider signature verification and an authorized binding resolver are required
// before Receive can accept any event. There are no default/fake providers.
func NewExternalIntake(db *gorm.DB, bindings ExternalBindingResolver, fees FeePolicyProvider, adapters []ExternalOrderAdapter, cfg ExternalIntakeConfig) (*ExternalIntake, error) {
	if db == nil || bindings == nil || fees == nil {
		return nil, ErrInvalidRecord
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Lease == 0 {
		cfg.Lease = 30 * time.Second
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 8
	}
	if cfg.Lease < time.Second || cfg.Lease > 10*time.Minute || cfg.MaxAttempts > 32 {
		return nil, ErrInvalidRecord
	}
	registry := make(map[SourcePlatform]ExternalOrderAdapter)
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, ErrInvalidRecord
		}
		platform := adapter.Platform()
		if platform != SourceTaobao && platform != SourceJD && platform != SourceMeituan {
			return nil, ErrInvalidRecord
		}
		if registry[platform] != nil {
			return nil, ErrInvalidRecord
		}
		registry[platform] = adapter
	}
	return &ExternalIntake{db: db, bindings: bindings, fees: fees, adapters: registry, now: cfg.Now, lease: cfg.Lease, maxAttempts: cfg.MaxAttempts}, nil
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

// Receipt is merchant-scoped. Delivery callers must still perform their normal
// authentication/authorization before exposing this through an API.
func (s *ExternalIntake) Receipt(ctx context.Context, merchantID, id uint64) (ExternalReceipt, error) {
	var row ExternalInbox
	if merchantID == 0 || id == 0 {
		return ExternalReceipt{}, ErrInvalidRecord
	}
	err := s.db.WithContext(ctx).Where("id = ? AND merchant_user_id = ?", id, merchantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ExternalReceipt{}, ErrNotFound
	}
	return intakeReceipt(row, false), err
}

// Retry only requeues exhausted transient failures; needs_review must not be
// blindly replayed. Eligibility is checked again by ProcessNext.
func (s *ExternalIntake) Retry(ctx context.Context, merchantID, id uint64) error {
	if merchantID == 0 || id == 0 {
		return ErrInvalidRecord
	}
	result := s.db.WithContext(ctx).Model(&ExternalInbox{}).Where("id = ? AND merchant_user_id = ? AND status = ?", id, merchantID, "deadletter").
		Updates(map[string]any{"status": "pending", "attempts": 0, "next_attempt_at": nil, "reason": "", "updated_at": s.now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInvalidTransition
	}
	return nil
}
