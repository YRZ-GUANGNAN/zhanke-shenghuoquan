package delivery

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ProofType string

const (
	ProofPickup   ProofType = "pickup"
	ProofDelivery ProofType = "delivery"
)

func (v ProofType) Valid() bool {
	return v == ProofPickup || v == ProofDelivery
}

type ProofStatus string

const (
	ProofBinding    ProofStatus = "binding"
	ProofActive     ProofStatus = "active"
	ProofDeadletter ProofStatus = "deadletter"
	ProofRevoked    ProofStatus = "revoked"
)

func (v ProofStatus) Valid() bool {
	switch v {
	case ProofBinding, ProofActive, ProofDeadletter, ProofRevoked:
		return true
	default:
		return false
	}
}

func (v ProofStatus) Terminal() bool {
	return v == ProofDeadletter || v == ProofRevoked
}

type Proof struct {
	ID                    uint64      `gorm:"column:id;primaryKey"`
	ProofPublicID         string      `gorm:"column:proof_public_id;size:26;not null;uniqueIndex:uq_delivery_proof_public_id"`
	DeliveryOrderPublicID string      `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_proof_order_type_slot,priority:1;index:idx_delivery_proof_order_status,priority:1"`
	ProofType             ProofType   `gorm:"column:proof_type;size:16;not null;uniqueIndex:uq_delivery_proof_order_type_slot,priority:2"`
	Slot                  uint16      `gorm:"column:slot;not null;uniqueIndex:uq_delivery_proof_order_type_slot,priority:3"`
	AssetPublicID         string      `gorm:"column:asset_public_id;size:26;not null;uniqueIndex:uq_delivery_proof_asset"`
	UploaderUserID        uint64      `gorm:"column:uploader_user_id;not null"`
	Status                ProofStatus `gorm:"column:status;size:16;not null;index:idx_delivery_proof_order_status,priority:2;index:idx_delivery_proof_binding_scan,priority:1"`
	OperationID           string      `gorm:"column:operation_id;size:26;not null;uniqueIndex:uq_delivery_proof_operation"`
	RowVersion            uint64      `gorm:"column:row_version;not null"`
	AttemptCount          uint32      `gorm:"column:attempt_count;not null"`
	NextAttemptAt         *time.Time  `gorm:"column:next_attempt_at;index:idx_delivery_proof_binding_scan,priority:2"`
	LeaseToken            *string     `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt        *time.Time  `gorm:"column:lease_expires_at;index:idx_delivery_proof_binding_scan,priority:3"`
	CreatedAt             time.Time   `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time   `gorm:"column:updated_at;not null"`
	ActivatedAt           *time.Time  `gorm:"column:activated_at"`
	RevokedAt             *time.Time  `gorm:"column:revoked_at"`
}

func (Proof) TableName() string { return "delivery_proofs" }

func (v Proof) ValidateNew() error {
	if !ValidULID(v.ProofPublicID) || !ValidULID(v.DeliveryOrderPublicID) ||
		!ValidULID(v.AssetPublicID) || !ValidULID(v.OperationID) ||
		!v.ProofType.Valid() || v.Slot != 0 || v.UploaderUserID == 0 ||
		v.Status != ProofBinding || v.RowVersion != 1 ||
		v.AttemptCount != 0 || v.CreatedAt.IsZero() || v.UpdatedAt.IsZero() {
		return ErrInvalidProof
	}
	return nil
}

type MediaProofPort interface {
	ValidateOwnedApprovedAsset(context.Context, uint64, string) error
	PublishProofReference(context.Context, string, string, uint16) error
	ResolveProofURL(context.Context, string, uint16, time.Duration) (string, error)
}

type ProofTransitionInput struct {
	OrderPublicID      string
	RiderUserID        uint64
	ExpectedRowVersion uint64
	ProofType          ProofType
	AssetPublicID      string
}

func (v ProofTransitionInput) validate() error {
	if !ValidULID(strings.TrimSpace(v.OrderPublicID)) || v.RiderUserID == 0 ||
		v.ExpectedRowVersion == 0 || !v.ProofType.Valid() ||
		!ValidULID(strings.TrimSpace(v.AssetPublicID)) {
		return ErrInvalidProof
	}
	return nil
}

type ProofBindingLease struct {
	Proof      Proof
	LeaseToken string
}

var (
	ErrInvalidProof        = errors.New("invalid delivery proof")
	ErrProofRequired       = errors.New("delivery proof is required")
	ErrProofAssetRejected  = errors.New("delivery proof asset rejected")
	ErrProofConflict       = errors.New("delivery proof conflicts with existing fact")
	ErrProofNotActive      = errors.New("delivery proof is not active")
	ErrProofBindingPending = errors.New("delivery proof media binding pending")
	ErrProofLease          = errors.New("delivery proof lease lost")
)
