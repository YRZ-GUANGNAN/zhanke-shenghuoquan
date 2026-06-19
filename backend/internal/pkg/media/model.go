// Package media provides the shared P0-4 media infrastructure used by domains A, B and C.
// It owns upload credentials, storage verification, safety review and object lifecycle only.
package media

import "time"

type Purpose string

const (
	PurposePostImage    Purpose = "post_image"
	PurposeProductImage Purpose = "product_image"
	PurposeAvatar       Purpose = "avatar"
)

type Status string

const (
	StatusPendingUpload Status = "pending_upload"
	StatusUploaded      Status = "uploaded"
	StatusSubmitted     Status = "submitted"
	StatusReviewing     Status = "reviewing"
	StatusApproved      Status = "approved"
	StatusRejected      Status = "rejected"
	StatusDeleted       Status = "deleted"
)

type Asset struct {
	ID                   uint64     `json:"-" gorm:"primaryKey;index:idx_media_review_pending,priority:3;index:idx_media_delete_pending,priority:3"`
	PublicID             string     `json:"id" gorm:"size:32;uniqueIndex:idx_media_public_id;not null"`
	OwnerUserID          uint       `json:"-" gorm:"not null;index:idx_media_owner_created,priority:1;uniqueIndex:idx_media_owner_idempotency,priority:1"`
	Purpose              Purpose    `json:"purpose" gorm:"size:32;not null"`
	ObjectKey            string     `json:"-" gorm:"size:512;uniqueIndex:idx_media_object_key;not null"`
	DeclaredMIME         string     `json:"-" gorm:"size:64;not null"`
	VerifiedMIME         string     `json:"mimeType" gorm:"size:64;not null;default:''"`
	DeclaredSize         int64      `json:"-" gorm:"not null"`
	VerifiedSize         int64      `json:"sizeBytes" gorm:"not null;default:0"`
	Width                int        `json:"width" gorm:"not null;default:0"`
	Height               int        `json:"height" gorm:"not null;default:0"`
	Status               Status     `json:"status" gorm:"size:24;not null;index:idx_media_review_pending,priority:1"`
	ReviewTaskID         uint64     `json:"-" gorm:"index:idx_media_review_task"`
	WechatOpenID         string     `json:"-" gorm:"size:96;not null;default:''"`
	ReviewTraceID        *string    `json:"-" gorm:"size:128;uniqueIndex:idx_media_review_trace"`
	ReviewSuggest        string     `json:"reviewSuggest,omitempty" gorm:"size:24;not null;default:''"`
	ReviewLabel          int        `json:"reviewLabel,omitempty" gorm:"not null;default:0"`
	ReferenceType        string     `json:"-" gorm:"size:24;index:idx_media_reference,priority:1"`
	ReferenceID          uint64     `json:"-" gorm:"index:idx_media_reference,priority:2"`
	ReferenceSort        int        `json:"-" gorm:"index:idx_media_reference,priority:3"`
	ReservationToken     string     `json:"-" gorm:"size:32;index:idx_media_reservation"`
	ReservationExpiresAt *time.Time `json:"-"`
	IdempotencyKey       string     `json:"-" gorm:"size:96;not null;uniqueIndex:idx_media_owner_idempotency,priority:2"`
	UploadExpiresAt      time.Time  `json:"uploadExpiresAt" gorm:"not null;index:idx_media_upload_expires"`
	ReviewAttempts       int        `json:"-" gorm:"not null;default:0"`
	ReviewAvailableAt    time.Time  `json:"-" gorm:"not null;index:idx_media_review_pending,priority:2"`
	LastError            string     `json:"error,omitempty" gorm:"size:512;not null;default:''"`
	DeleteRequestedAt    *time.Time `json:"-" gorm:"index:idx_media_delete_pending,priority:1"`
	StorageDeletedAt     *time.Time `json:"-" gorm:"index:idx_media_delete_pending,priority:2"`
	CreatedAt            time.Time  `json:"createdAt" gorm:"index:idx_media_owner_created,priority:2"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func (Asset) TableName() string { return "media_assets" }

// AssetAudit is the append-only state/reference audit trail for a media asset.
// It intentionally contains no COS URL or credential material.
type AssetAudit struct {
	ID         uint64    `json:"-" gorm:"primaryKey"`
	AssetID    uint64    `json:"assetId" gorm:"not null;index:idx_media_audit_asset,priority:1"`
	FromStatus Status    `json:"fromStatus" gorm:"size:24;not null"`
	ToStatus   Status    `json:"toStatus" gorm:"size:24;not null"`
	Action     string    `json:"action" gorm:"size:48;not null;index:idx_media_audit_action,priority:1"`
	ActorType  string    `json:"actorType" gorm:"size:24;not null"`
	ActorID    uint      `json:"actorId" gorm:"not null;default:0"`
	Detail     string    `json:"detail" gorm:"size:240;not null;default:''"`
	CreatedAt  time.Time `json:"createdAt" gorm:"not null;index:idx_media_audit_asset,priority:2;index:idx_media_audit_action,priority:2"`
}

func (AssetAudit) TableName() string { return "media_asset_audits" }

type TemporaryCredentials struct {
	SecretID     string `json:"tmpSecretId"`
	SecretKey    string `json:"tmpSecretKey"`
	SessionToken string `json:"sessionToken"`
	StartTime    int64  `json:"startTime"`
	ExpiredTime  int64  `json:"expiredTime"`
}

type UploadTicket struct {
	AssetID    string               `json:"assetId"`
	Bucket     string               `json:"bucket"`
	Region     string               `json:"region"`
	Key        string               `json:"key"`
	MIMEType   string               `json:"mimeType"`
	MaxBytes   int64                `json:"maxBytes"`
	Credential TemporaryCredentials `json:"credentials"`
}

type CreateUploadInput struct {
	OwnerUserID    uint
	Purpose        Purpose
	FileName       string
	MIMEType       string
	SizeBytes      int64
	IdempotencyKey string
}

type Reference struct {
	InternalID uint64
	PublicID   string
	URL        string
}

type Reservation struct {
	Token      string
	References []Reference
}

type ObjectInfo struct {
	Size        int64
	ContentType string
}
