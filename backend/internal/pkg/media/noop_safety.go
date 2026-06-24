package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

type LocalNoopSafety struct{}

func NewLocalNoopSafety() *LocalNoopSafety {
	return &LocalNoopSafety{}
}

func (c *LocalNoopSafety) Submit(ctx context.Context, openID, mediaURL string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if openID == "" || mediaURL == "" {
		return "", errors.New("openid and media url are required")
	}
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "local-noop-" + hex.EncodeToString(value), nil
}

var _ SafetyChecker = (*LocalNoopSafety)(nil)
