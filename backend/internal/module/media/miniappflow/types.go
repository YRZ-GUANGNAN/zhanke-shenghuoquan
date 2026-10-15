package miniappflow

import (
	"errors"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type ClientState string

const (
	StateWaitingUpload ClientState = "waiting_upload"
	StateProcessing    ClientState = "processing"
	StateReviewPending ClientState = "review_pending"
	StateApproved      ClientState = "approved"
	StateReady         ClientState = "ready"
	StateRejected      ClientState = "rejected"
	StateReviewFailed  ClientState = "review_failed"
	StateExpired       ClientState = "expired"
	StateUploadFailed  ClientState = "upload_failed"
	StateDeleted       ClientState = "deleted"
)

type RecoveryAction string

const (
	RecoveryNone      RecoveryAction = "none"
	RecoveryPoll      RecoveryAction = "poll"
	RecoveryNewUpload RecoveryAction = "new_upload"
	RecoveryComplete  RecoveryAction = "complete_upload"
)

type CreateUploadInput struct {
	OwnerID        uint64
	Purpose        coremedia.Purpose
	SizeBytes      uint64
	ContentMD5     string
	ContentType    string
	IdempotencyKey string
}

type RequiredHeaders struct {
	ContentType     string `json:"contentType"`
	ContentMD5      string `json:"contentMD5"`
	ContentLength   uint64 `json:"contentLength"`
	ForbidOverwrite string `json:"forbidOverwrite"`
}

type UploadSession struct {
	TicketID  string          `json:"ticketId"`
	AssetID   string          `json:"assetId"`
	UploadURL string          `json:"uploadUrl"`
	Headers   RequiredHeaders `json:"requiredHeaders"`
	ExpiresAt time.Time       `json:"expiresAt"`
	State     ClientState     `json:"state"`
	Message   string          `json:"message"`
	Recovery  RecoveryAction  `json:"recovery"`
}

type AssetStatus struct {
	AssetID         string            `json:"assetId"`
	TicketID        string            `json:"ticketId"`
	Purpose         coremedia.Purpose `json:"purpose"`
	State           ClientState       `json:"state"`
	TechnicalStatus string            `json:"technicalStatus"`
	ReviewStatus    string            `json:"reviewStatus"`
	TicketStatus    string            `json:"ticketStatus"`
	Message         string            `json:"message"`
	Recovery        RecoveryAction    `json:"recovery"`
	CanUse          bool              `json:"canUse"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

type History struct {
	Items    []AssetStatus `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int64         `json:"total"`
}

type ResolvedDisplayURL struct {
	URL              string `json:"url"`
	ExpiresInSeconds int64  `json:"expiresInSeconds"`
}

type StatusFact struct {
	AssetID         uint64
	AssetPublicID   string
	OwnerID         uint64
	Purpose         coremedia.Purpose
	StorageStage    coremedia.StorageStage
	TechnicalStatus coremedia.TechnicalStatus
	AssetReview     coremedia.ReviewStatus
	DeleteRequested bool
	TicketStatus    coremedia.TicketStatus
	TicketID        string
	Projection      *coremedia.ProjectionStatus
	UpdatedAt       time.Time
}

var (
	ErrInvalidRequest        = errors.New("invalid miniapp media request")
	ErrAuthorizationConsumed = errors.New("upload authorization cannot be reissued; create a new upload")
	ErrUploadNotStarted      = errors.New("COS upload has not been authorized")
	ErrUploadExpired         = errors.New("upload ticket expired; create a new upload")
	ErrUploadCancelled       = errors.New("upload ticket cancelled; create a new upload")
	ErrSubmissionPending     = errors.New("review submission is pending and may be retried")
)
