package media

import (
	"errors"
	"time"
)

var (
	ErrQuotaExceeded              = errors.New("media upload quota exceeded")
	ErrIdempotencyConflict        = errors.New("media upload idempotency conflict")
	ErrTicketExpired              = errors.New("media upload ticket expired")
	ErrTicketForbidden            = errors.New("media upload ticket does not belong to owner")
	ErrTicketInvariant            = errors.New("media upload ticket invariant violated")
	ErrAuthorizationAlreadyIssued = errors.New("media upload authorization was already issued")
	ErrInvalidUploadDeclaration   = errors.New("invalid media upload declaration")
	ErrQuotaLockTimeout           = errors.New("media quota lock timeout")
)

type QuotaPolicy struct {
	ActiveCount uint32
	ActiveBytes uint64
	MinuteCount uint32
	MinuteBytes uint64
	HourCount   uint32
	HourBytes   uint64
	DayCount    uint32
	DayBytes    uint64
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		ActiveCount: 20, ActiveBytes: 200 << 20,
		MinuteCount: 12, MinuteBytes: 100 << 20,
		HourCount: 60, HourBytes: 500 << 20,
		DayCount: 200, DayBytes: 1 << 30,
	}
}

func (p QuotaPolicy) valid() bool {
	return p.ActiveCount > 0 && p.ActiveBytes > 0 && p.MinuteCount > 0 && p.MinuteBytes > 0 &&
		p.HourCount > 0 && p.HourBytes > 0 && p.DayCount > 0 && p.DayBytes > 0
}

type CreateUploadTicketInput struct {
	OwnerID             uint64
	Purpose             Purpose
	ExpectedSize        uint64
	ExpectedMD5         string
	ExpectedContentType string
	IdempotencyKey      string
}

type UploadBundle struct {
	Asset       Asset
	Ticket      UploadTicket
	Reservation QuotaReservation
}

type CreateUploadBundleParams struct {
	Bundle UploadBundle
	Policy QuotaPolicy
	Now    time.Time
}

type RequiredUploadHeaders struct {
	ContentType     string
	ContentMD5      string
	ContentLength   uint64
	ForbidOverwrite string
}

type UploadAuthorization struct {
	TicketID        string
	AssetPublicID   string
	UploadURL       string
	RequiredHeaders RequiredUploadHeaders
	ExpiresAt       time.Time
}

type AuthorizationRequest struct {
	TicketID    string
	ObjectKey   string
	Size        uint64
	MD5         string
	ContentType string
	TTL         time.Duration
}
