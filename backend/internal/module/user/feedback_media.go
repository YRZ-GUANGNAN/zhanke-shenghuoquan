package user

import (
	"context"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type CoreFeedbackMedia struct {
	bindings *coremedia.ReferenceBindingService
	now      func() time.Time
}

func NewCoreFeedbackMedia(bindings *coremedia.ReferenceBindingService) (*CoreFeedbackMedia, error) {
	if bindings == nil {
		return nil, ErrFeedbackUnavailable
	}
	return &CoreFeedbackMedia{bindings: bindings, now: time.Now}, nil
}

func (m *CoreFeedbackMedia) ReserveFeedback(ctx context.Context, userID uint, assetIDs []string) (FeedbackMediaReservation, error) {
	if len(assetIDs) == 0 {
		return FeedbackMediaReservation{}, nil
	}
	tokenID, err := coremedia.NewULID()
	if err != nil {
		return FeedbackMediaReservation{}, err
	}
	token := "m10_" + tokenID
	value, err := m.bindings.Reserve(ctx, coremedia.BindingReserveInput{
		OwnerID: uint64(userID), Purpose: coremedia.PurposeFeedback,
		OrderedAssetPublicIDs: append([]string(nil), assetIDs...), Token: token, ExpiresAt: m.now().UTC().Add(15 * time.Minute),
	})
	if err != nil {
		return FeedbackMediaReservation{}, err
	}
	if len(value.References) != len(assetIDs) {
		_ = m.bindings.Cancel(ctx, uint64(userID), token)
		return FeedbackMediaReservation{}, coremedia.ErrBindingMismatch
	}
	return FeedbackMediaReservation{Token: token, AssetIDs: append([]string(nil), assetIDs...)}, nil
}

func (m *CoreFeedbackMedia) CommitFeedback(ctx context.Context, userID uint, value FeedbackMediaReservation, feedbackID string) error {
	return m.bindings.Commit(ctx, coremedia.BindingCommitInput{
		OwnerID: uint64(userID), Token: value.Token, ReferenceType: coremedia.PurposeFeedback,
		ReferenceID: feedbackID, OrderedAssetPublicIDs: append([]string(nil), value.AssetIDs...),
	})
}

func (m *CoreFeedbackMedia) CancelFeedback(ctx context.Context, userID uint, value FeedbackMediaReservation) error {
	if value.Token == "" {
		return nil
	}
	return m.bindings.Cancel(ctx, uint64(userID), value.Token)
}

func newFeedbackPublicID() (string, error) { return coremedia.NewULID() }
