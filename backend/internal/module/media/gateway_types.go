package media

import (
	"errors"
	"time"
)

// VisibilityReason is intentionally storage-agnostic. It must never reveal a
// bucket, object key, version ID, upload ticket, or other internal detail.
type VisibilityReason string

const (
	VisibilityAllowed           VisibilityReason = "visible"
	VisibilityNotReady          VisibilityReason = "not_ready"
	VisibilityNotApproved       VisibilityReason = "not_approved"
	VisibilityReferenceInactive VisibilityReason = "reference_inactive"
	VisibilityParentHidden      VisibilityReason = "parent_hidden"
	VisibilityRevoked           VisibilityReason = "revoked"
	VisibilityPurposeMismatch   VisibilityReason = "purpose_mismatch"
)

type MediaVisibilityView struct {
	MediaID string           `json:"mediaId"`
	Visible bool             `json:"visible"`
	Reason  VisibilityReason `json:"reason"`
}

type MediaMetadataView struct {
	MediaID      string           `json:"mediaId"`
	Purpose      Purpose          `json:"purpose"`
	MIMEType     string           `json:"mimeType"`
	ByteSize     uint64           `json:"byteSize"`
	Width        uint32           `json:"width"`
	Height       uint32           `json:"height"`
	ReviewStatus ProjectionStatus `json:"reviewStatus"`
}

type MediaReferenceView struct {
	ReferenceType Purpose `json:"referenceType"`
	ReferenceID   string  `json:"referenceId"`
	Slot          uint16  `json:"slot"`
}

type MediaDisplayView struct {
	Media     MediaMetadataView  `json:"media"`
	Reference MediaReferenceView `json:"reference"`
}

// ReviewProjectionView exposes projection state only. It contains no review
// task ID, decision payload, owner, storage identity, or upload information.
type ReviewProjectionView struct {
	MediaID                string           `json:"mediaId"`
	Status                 ProjectionStatus `json:"status"`
	AppliedDecisionVersion uint64           `json:"appliedDecisionVersion"`
	DecidedAt              *time.Time       `json:"decidedAt,omitempty"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

// GatewaySnapshot is an internal read model populated by an explicit SQL
// allowlist. Storage keys and version IDs never enter this structure.
type GatewaySnapshot struct {
	AssetPublicID          string
	Purpose                Purpose
	StorageStage           StorageStage
	TechnicalStatus        TechnicalStatus
	VerifiedMIME           string
	ByteSize               uint64
	Width                  uint32
	Height                 uint32
	FinalIdentityReady     bool
	DeleteRequested        bool
	ProjectionStatus       ProjectionStatus
	AppliedDecisionVersion uint64
	ProjectionDecidedAt    *time.Time
	ProjectionUpdatedAt    *time.Time
	HasReference           bool
	ReferenceType          Purpose
	ReferenceID            string
	ReferenceSlot          uint16
	ReferenceStatus        ReferenceStatus
	ParentVisibility       ParentVisibility
	RevocationFence        string
}

var (
	ErrMediaNotVisible             = errors.New("media is not visible")
	ErrReviewProjectionUnavailable = errors.New("media review projection unavailable")
)
