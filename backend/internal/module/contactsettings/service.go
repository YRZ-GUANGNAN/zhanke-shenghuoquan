package contactsettings

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	return s.repo.Get(ctx)
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (Settings, error) {
	if err := input.Validate(); err != nil {
		return Settings{}, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	settings, err := s.repo.Update(ctx, input, s.now().UTC())
	if errors.Is(err, ErrConflict) {
		return Settings{}, ErrConflict
	}
	return settings, err
}
