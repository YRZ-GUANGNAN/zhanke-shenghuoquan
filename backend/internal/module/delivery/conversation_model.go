package delivery

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrConversationNotFound     = errors.New("delivery conversation not found")
	ErrConversationForbidden    = errors.New("delivery conversation forbidden")
	ErrConversationUnavailable  = errors.New("delivery conversation unavailable")
	ErrConversationReadOnly     = errors.New("delivery conversation is read-only")
	ErrConversationConcurrent   = errors.New("concurrent delivery conversation update")
	ErrConversationLease        = errors.New("delivery conversation lease lost")
	ErrConversationImageDenied  = errors.New("delivery conversation image denied")
	ErrConversationImagePending = errors.New("delivery conversation image binding pending")
)

type ConversationStatus string

const (
	ConversationProvisioning ConversationStatus = "provisioning"
	ConversationActive       ConversationStatus = "active"
	ConversationReadOnly     ConversationStatus = "read_only"
	ConversationRevoking     ConversationStatus = "revoking"
	ConversationRevoked      ConversationStatus = "revoked"
	ConversationFailed       ConversationStatus = "failed"
)

func (s ConversationStatus) Valid() bool {
	switch s {
	case ConversationProvisioning, ConversationActive, ConversationReadOnly,
		ConversationRevoking, ConversationRevoked, ConversationFailed:
		return true
	default:
		return false
	}
}

func (s ConversationStatus) Terminal() bool { return s == ConversationRevoked }

type Conversation struct {
	ID                    uint64             `gorm:"column:id;primaryKey"`
	ConversationPublicID  string             `gorm:"column:conversation_public_id;size:26;not null;uniqueIndex:uq_delivery_im_conversation_public_id"`
	DeliveryOrderPublicID string             `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_im_order_assignment,priority:1;index:idx_delivery_im_order_status,priority:1"`
	AssignmentVersion     uint64             `gorm:"column:assignment_version;not null;uniqueIndex:uq_delivery_im_order_assignment,priority:2"`
	MerchantUserID        uint64             `gorm:"column:merchant_user_id;not null;index:idx_delivery_im_participant_status,priority:1"`
	RiderUserID           uint64             `gorm:"column:rider_user_id;not null;index:idx_delivery_im_participant_status,priority:2"`
	TencentGroupID        string             `gorm:"column:tencent_group_id;size:64;not null;uniqueIndex:uq_delivery_im_tencent_group_id"`
	Status                ConversationStatus `gorm:"column:status;size:16;not null;index:idx_delivery_im_order_status,priority:2;index:idx_delivery_im_participant_status,priority:3;index:idx_delivery_im_provision_scan,priority:1"`
	NextImageSlot         uint16             `gorm:"column:next_image_slot;not null"`
	RowVersion            uint64             `gorm:"column:row_version;not null"`
	AttemptCount          uint32             `gorm:"column:attempt_count;not null"`
	NextAttemptAt         *time.Time         `gorm:"column:next_attempt_at;index:idx_delivery_im_provision_scan,priority:2"`
	LeaseToken            *string            `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt        *time.Time         `gorm:"column:lease_expires_at;index:idx_delivery_im_provision_scan,priority:3"`
	LastError             string             `gorm:"column:last_error;size:512;not null"`
	CreatedAt             time.Time          `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time          `gorm:"column:updated_at;not null"`
	ActivatedAt           *time.Time         `gorm:"column:activated_at"`
	ReadOnlyAt            *time.Time         `gorm:"column:read_only_at"`
	RevokedAt             *time.Time         `gorm:"column:revoked_at"`
}

func (Conversation) TableName() string { return "delivery_im_conversations" }

func (c Conversation) ValidateNew() error {
	if !ValidULID(c.ConversationPublicID) || !ValidULID(c.DeliveryOrderPublicID) ||
		c.AssignmentVersion == 0 || c.MerchantUserID == 0 || c.RiderUserID == 0 ||
		strings.TrimSpace(c.TencentGroupID) == "" || len(c.TencentGroupID) > 64 ||
		c.Status != ConversationProvisioning || c.RowVersion != 1 ||
		c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type ConversationLease struct {
	Conversation Conversation
	LeaseToken   string
}

func orderAllowsConversationSend(status Status) bool {
	switch status {
	case StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering:
		return true
	default:
		return false
	}
}

func orderConversationReadOnly(status Status) bool {
	return status == StatusDelivered || status == StatusCancelled
}
