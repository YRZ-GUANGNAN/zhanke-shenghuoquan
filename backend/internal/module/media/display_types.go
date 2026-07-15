package media

import (
	"context"
	"time"
)

const MediaDisplayURLTTL = 5 * time.Minute

type DisplayObjectIdentity struct {
	MediaID        string
	FinalKey       string
	FinalVersionID string
}

type MediaDisplayReadRepository interface {
	FindVisibleDisplayObjectByReference(context.Context, Purpose, string, uint16) (DisplayObjectIdentity, error)
	FindVisibleDisplayObjectByAsset(context.Context, string) (DisplayObjectIdentity, error)
}

type DisplayURLSigner interface {
	SignedGetURL(context.Context, string, string, time.Duration) (string, error)
}

type displayGateway interface {
	BuildDisplayView(context.Context, Purpose, string, uint16) (MediaDisplayView, error)
	GetVisibleMetadata(context.Context, string) (MediaMetadataView, error)
}
