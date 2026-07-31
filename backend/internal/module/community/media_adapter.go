package community

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

const communityMediaReservationTTL = 15 * time.Minute

type communityReferenceBindingGateway interface {
	Reserve(context.Context, coremedia.BindingReserveInput) (coremedia.BindingReservation, error)
	Commit(context.Context, coremedia.BindingCommitInput) error
	EnsureCommitted(context.Context, coremedia.BindingCommitInput) error
	Cancel(context.Context, uint64, string) error
	Revoke(context.Context, coremedia.Purpose, string, []uint64) error
}

type legacyCommunityMediaGateway interface {
	legacymedia.Gateway
	EnsureCommitted(context.Context, uint, string, string, uint64, []string) error
}

// HybridMediaGateway binds Module 10 uploads for new posts while preserving
// the legacy media path for historical assets.
type HybridMediaGateway struct {
	core       communityReferenceBindingGateway
	legacy     legacyCommunityMediaGateway
	publicBase string
}

func NewHybridMediaGateway(core communityReferenceBindingGateway, legacy legacyCommunityMediaGateway, publicBase string) (*HybridMediaGateway, error) {
	if core == nil || legacy == nil {
		return nil, legacymedia.ErrInvalidAsset
	}
	return &HybridMediaGateway{
		core: core, legacy: legacy, publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/"),
	}, nil
}

func (g *HybridMediaGateway) Reserve(ctx context.Context, ownerID uint, purpose legacymedia.Purpose, publicIDs []string) (legacymedia.Reservation, error) {
	if ownerID == 0 || purpose != legacymedia.PurposePostImage {
		return legacymedia.Reservation{}, legacymedia.ErrInvalidAsset
	}
	if len(publicIDs) == 0 {
		return legacymedia.Reservation{}, nil
	}
	operationID, err := coremedia.NewULID()
	if err != nil {
		return legacymedia.Reservation{}, err
	}
	input := coremedia.BindingReserveInput{
		OwnerID:               uint64(ownerID),
		Purpose:               coremedia.PurposePost,
		OrderedAssetPublicIDs: append([]string(nil), publicIDs...),
		Token:                 "m10_" + operationID,
		ExpiresAt:             time.Now().UTC().Add(communityMediaReservationTTL),
	}
	reservation, err := g.core.Reserve(ctx, input)
	if errors.Is(err, coremedia.ErrBindingLegacySet) {
		return g.legacy.Reserve(ctx, ownerID, purpose, publicIDs)
	}
	if err != nil {
		return legacymedia.Reservation{}, mapCommunityCoreMediaError(err)
	}
	if reservation.Token != input.Token || len(reservation.References) != len(publicIDs) {
		return legacymedia.Reservation{}, legacymedia.ErrInvalidAsset
	}
	result := legacymedia.Reservation{
		Token: reservation.Token, References: make([]legacymedia.Reference, 0, len(reservation.References)),
	}
	for _, reference := range reservation.References {
		if reference.InternalID == 0 || reference.AssetPublicID == "" {
			return legacymedia.Reservation{}, legacymedia.ErrInvalidAsset
		}
		result.References = append(result.References, legacymedia.Reference{
			InternalID: reference.InternalID,
			PublicID:   reference.AssetPublicID,
			URL:        g.publicBase + "/api/v1/media/assets/" + reference.AssetPublicID + "/content",
		})
	}
	return result, nil
}

func (g *HybridMediaGateway) Commit(ctx context.Context, ownerID uint, reservation legacymedia.Reservation, referenceType string, referenceID uint64) error {
	if !communityCoreMediaToken(reservation.Token) {
		return g.legacy.Commit(ctx, ownerID, reservation, referenceType, referenceID)
	}
	input, err := communityBindingCommitInput(ownerID, reservation.Token, referenceType, referenceID, reservationPublicIDs(reservation))
	if err != nil {
		return err
	}
	return mapCommunityCoreMediaError(g.core.Commit(ctx, input))
}

func (g *HybridMediaGateway) EnsureCommitted(ctx context.Context, ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) error {
	if !communityCoreMediaToken(token) {
		return g.legacy.EnsureCommitted(ctx, ownerID, token, referenceType, referenceID, orderedIDs)
	}
	input, err := communityBindingCommitInput(ownerID, token, referenceType, referenceID, orderedIDs)
	if err != nil {
		return err
	}
	return mapCommunityCoreMediaError(g.core.EnsureCommitted(ctx, input))
}

func (g *HybridMediaGateway) Cancel(ctx context.Context, ownerID uint, reservation legacymedia.Reservation) error {
	if !communityCoreMediaToken(reservation.Token) {
		return g.legacy.Cancel(ctx, ownerID, reservation)
	}
	if ownerID == 0 {
		return legacymedia.ErrInvalidAsset
	}
	return mapCommunityCoreMediaError(g.core.Cancel(ctx, uint64(ownerID), reservation.Token))
}

func (g *HybridMediaGateway) DeleteReference(ctx context.Context, referenceType string, referenceID uint64, keep []uint64) error {
	if referenceType != "post" || referenceID == 0 {
		return legacymedia.ErrInvalidAsset
	}
	coreErr := g.core.Revoke(ctx, coremedia.PurposePost, strconv.FormatUint(referenceID, 10), append([]uint64(nil), keep...))
	if errors.Is(coreErr, coremedia.ErrNotFound) {
		coreErr = nil
	}
	legacyErr := g.legacy.DeleteReference(ctx, referenceType, referenceID, append([]uint64(nil), keep...))
	return errors.Join(mapCommunityCoreMediaError(coreErr), legacyErr)
}

func communityBindingCommitInput(ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) (coremedia.BindingCommitInput, error) {
	if ownerID == 0 || referenceType != "post" || referenceID == 0 || len(orderedIDs) == 0 {
		return coremedia.BindingCommitInput{}, legacymedia.ErrInvalidAsset
	}
	for _, publicID := range orderedIDs {
		if strings.TrimSpace(publicID) == "" {
			return coremedia.BindingCommitInput{}, legacymedia.ErrInvalidAsset
		}
	}
	return coremedia.BindingCommitInput{
		OwnerID:               uint64(ownerID),
		Token:                 token,
		ReferenceType:         coremedia.PurposePost,
		ReferenceID:           strconv.FormatUint(referenceID, 10),
		OrderedAssetPublicIDs: append([]string(nil), orderedIDs...),
	}, nil
}

func reservationPublicIDs(reservation legacymedia.Reservation) []string {
	result := make([]string, 0, len(reservation.References))
	for _, reference := range reservation.References {
		result = append(result, reference.PublicID)
	}
	return result
}

func communityCoreMediaToken(token string) bool {
	return strings.HasPrefix(token, "m10_")
}

func mapCommunityCoreMediaError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, coremedia.ErrBindingConflict):
		return fmt.Errorf("%w: %v", legacymedia.ErrAlreadyBound, err)
	case errors.Is(err, coremedia.ErrBindingInvalidAsset),
		errors.Is(err, coremedia.ErrBindingMismatch),
		errors.Is(err, coremedia.ErrBindingLegacySet),
		errors.Is(err, coremedia.ErrInvalidULID),
		errors.Is(err, coremedia.ErrInvalidRecord),
		errors.Is(err, coremedia.ErrNotFound):
		return fmt.Errorf("%w: %v", legacymedia.ErrInvalidAsset, err)
	default:
		return err
	}
}

var (
	_ legacymedia.Gateway = (*HybridMediaGateway)(nil)
	_ MediaEnsurer        = (*HybridMediaGateway)(nil)
)
