package media

import (
	"context"
	"errors"
	"time"
)

const (
	module10BindingTokenPrefix = "m10_"
	maxReferenceBindingAssets  = 9
)

var (
	ErrBindingInvalidAsset = errors.New("invalid media asset")
	ErrBindingConflict     = errors.New("media asset already bound")
	ErrBindingMismatch     = errors.New("media binding coordinates mismatch")
	ErrBindingLegacySet    = errors.New("legacy media asset set")
)

type BindingAsset struct {
	InternalID    uint64
	AssetPublicID string
	Slot          uint16
	Kept          bool
}

type BindingReservation struct {
	Token      string
	References []BindingAsset
}

type BindingReserveInput struct {
	OwnerID               uint64
	Purpose               Purpose
	OrderedAssetPublicIDs []string
	Token                 string
	ExpiresAt             time.Time
	ExistingReferenceID   string
}

type BindingCommitInput struct {
	OwnerID               uint64
	Token                 string
	ReferenceType         Purpose
	ReferenceID           string
	OrderedAssetPublicIDs []string
}

type ReferenceBindingRepository interface {
	ReserveBinding(context.Context, BindingReserveInput) ([]BindingAsset, error)
	CommitBinding(context.Context, BindingCommitInput) error
	ExactCommittedBinding(context.Context, BindingCommitInput) (bool, error)
	CancelBinding(context.Context, uint64, string) error
	PublishBinding(context.Context, Purpose, string) error
	RevokeBinding(context.Context, Purpose, string, []uint64, string) error
}
