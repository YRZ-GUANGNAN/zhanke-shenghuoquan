package contactsettings

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubRepository struct {
	current  Settings
	conflict bool
	update   UpdateInput
}

func (s *stubRepository) Get(context.Context) (Settings, error) { return s.current, nil }

func (s *stubRepository) Update(_ context.Context, input UpdateInput, _ time.Time) (Settings, error) {
	s.update = input
	if s.conflict {
		return Settings{}, ErrConflict
	}
	return Settings{MakerWechat: input.MakerWechat, MakerIntro: input.MakerIntro, RowVersion: input.ExpectedRowVersion + 1}, nil
}

func TestUpdateValidatesInput(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo, nil)
	ctx := context.Background()

	if _, err := service.Update(ctx, UpdateInput{MakerWechat: "has space", ExpectedRowVersion: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wechat with spaces must be rejected, got %v", err)
	}
	if _, err := service.Update(ctx, UpdateInput{MakerWechat: "ok-id", MakerIntro: strings.Repeat("长", 501), ExpectedRowVersion: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized intro must be rejected, got %v", err)
	}

	view, err := service.Update(ctx, UpdateInput{MakerWechat: "  zhanke-maker  ", MakerIntro: " 第一行\n第二行 ", ExpectedRowVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	if repo.update.MakerWechat != "zhanke-maker" || repo.update.MakerIntro != "第一行\n第二行" {
		t.Fatalf("input must be trimmed before persisting: %+v", repo.update)
	}
	if view.RowVersion != 4 || view.MakerWechat != "zhanke-maker" {
		t.Fatalf("unexpected settings view: %+v", view)
	}
}

func TestUpdateMapsConflict(t *testing.T) {
	service := NewService(&stubRepository{conflict: true}, nil)
	if _, err := service.Update(context.Background(), UpdateInput{MakerWechat: "ok-id", ExpectedRowVersion: 1}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale row version must map to ErrConflict, got %v", err)
	}
}
