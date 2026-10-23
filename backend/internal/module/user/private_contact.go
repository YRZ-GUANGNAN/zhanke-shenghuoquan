package user

import (
	"context"
	"errors"
)

var ErrPrivateContactUnavailable = errors.New("当前无法与该用户私信")

type privateContactRepository interface {
	HasBlockBetween(context.Context, uint, uint) (bool, error)
}

// ValidatePrivateContact returns no identity or block-direction details.
func (s *Service) ValidatePrivateContact(ctx context.Context, viewerID, peerID uint) error {
	if viewerID == 0 || peerID == 0 || viewerID == peerID {
		return ErrPrivateContactUnavailable
	}
	for _, id := range []uint{viewerID, peerID} {
		value, err := s.repo.FindByID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			return ErrPrivateContactUnavailable
		}
		if err != nil {
			return err
		}
		if value.ID != id || value.Status != StatusActive {
			return ErrPrivateContactUnavailable
		}
	}
	repo, ok := s.repo.(privateContactRepository)
	if !ok {
		return ErrSettingsUnavailable
	}
	blocked, err := repo.HasBlockBetween(ctx, viewerID, peerID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrPrivateContactUnavailable
	}
	return nil
}
