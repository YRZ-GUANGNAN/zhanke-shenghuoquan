package media

import (
	"context"
	"strings"
	"time"
)

type MediaDisplayService struct {
	gateway    displayGateway
	repository MediaDisplayReadRepository
	signer     DisplayURLSigner
	ttl        time.Duration
}

func NewMediaDisplayService(gateway displayGateway, repository MediaDisplayReadRepository, signer DisplayURLSigner, ttl time.Duration) (*MediaDisplayService, error) {
	if gateway == nil || repository == nil || signer == nil || ttl <= 0 {
		return nil, ErrInvalidRecord
	}
	return &MediaDisplayService{gateway: gateway, repository: repository, signer: signer, ttl: ttl}, nil
}

func (s *MediaDisplayService) ContentURL(ctx context.Context, referenceType Purpose, referenceID string, slot uint16) (string, error) {
	referenceID = strings.TrimSpace(referenceID)
	if !referenceType.Valid() || referenceID == "" {
		return "", ErrInvalidRecord
	}
	if _, err := s.gateway.BuildDisplayView(ctx, referenceType, referenceID, slot); err != nil {
		return "", err
	}
	object, err := s.repository.FindVisibleDisplayObjectByReference(ctx, referenceType, referenceID, slot)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(object.FinalKey) == "" || strings.TrimSpace(object.FinalVersionID) == "" {
		return "", ErrMediaNotVisible
	}
	return s.signer.SignedGetURL(ctx, object.FinalKey, object.FinalVersionID, s.ttl)
}
