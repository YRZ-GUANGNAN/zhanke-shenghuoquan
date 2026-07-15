package trade

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

const tradeMediaReservationTTL = 15 * time.Minute

var (
	ErrMediaInvalid  = errors.New("invalid media asset")
	ErrMediaConflict = errors.New("media asset already bound")
)

type productMediaUpdater interface {
	ReserveForProduct(context.Context, uint, uint, legacymedia.Purpose, []string) (legacymedia.Reservation, error)
}

type productMediaPublisher interface {
	PublishProduct(context.Context, uint64) error
}

type coreReferenceBindingGateway interface {
	Reserve(context.Context, coremedia.BindingReserveInput) (coremedia.BindingReservation, error)
	Commit(context.Context, coremedia.BindingCommitInput) error
	EnsureCommitted(context.Context, coremedia.BindingCommitInput) error
	Cancel(context.Context, uint64, string) error
	Publish(context.Context, coremedia.Purpose, string) error
	Revoke(context.Context, coremedia.Purpose, string, []uint64) error
}

type legacyTradeMediaGateway interface {
	legacymedia.Gateway
	EnsureCommitted(context.Context, uint, string, string, uint64, []string) error
}

type HybridMediaGateway struct {
	core       coreReferenceBindingGateway
	legacy     legacyTradeMediaGateway
	publicBase string
}

func NewHybridMediaGateway(core coreReferenceBindingGateway, legacy legacyTradeMediaGateway, publicBase string) (*HybridMediaGateway, error) {
	if core == nil || legacy == nil {
		return nil, ErrMediaInvalid
	}
	return &HybridMediaGateway{core: core, legacy: legacy, publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/")}, nil
}

func (g *HybridMediaGateway) Reserve(ctx context.Context, ownerID uint, purpose legacymedia.Purpose, publicIDs []string) (legacymedia.Reservation, error) {
	return g.reserve(ctx, ownerID, 0, purpose, publicIDs)
}

func (g *HybridMediaGateway) ReserveForProduct(ctx context.Context, ownerID, productID uint, purpose legacymedia.Purpose, publicIDs []string) (legacymedia.Reservation, error) {
	if productID == 0 {
		return legacymedia.Reservation{}, ErrMediaInvalid
	}
	return g.reserve(ctx, ownerID, uint64(productID), purpose, publicIDs)
}

func (g *HybridMediaGateway) reserve(ctx context.Context, ownerID uint, productID uint64, purpose legacymedia.Purpose, publicIDs []string) (legacymedia.Reservation, error) {
	if ownerID == 0 || purpose != legacymedia.PurposeProductImage {
		return legacymedia.Reservation{}, ErrMediaInvalid
	}
	if len(publicIDs) == 0 {
		return legacymedia.Reservation{}, nil
	}
	operationID, err := coremedia.NewULID()
	if err != nil {
		return legacymedia.Reservation{}, err
	}
	input := coremedia.BindingReserveInput{
		OwnerID: uint64(ownerID), Purpose: coremedia.PurposeProduct,
		OrderedAssetPublicIDs: append([]string(nil), publicIDs...),
		Token:                 "m10_" + operationID,
		ExpiresAt:             time.Now().UTC().Add(tradeMediaReservationTTL),
	}
	if productID != 0 {
		input.ExistingReferenceID = strconv.FormatUint(productID, 10)
	}
	reservation, err := g.core.Reserve(ctx, input)
	if errors.Is(err, coremedia.ErrBindingLegacySet) {
		legacyReservation, legacyErr := g.legacy.Reserve(ctx, ownerID, purpose, publicIDs)
		return legacyReservation, mapLegacyMediaError(legacyErr)
	}
	if err != nil {
		return legacymedia.Reservation{}, mapCoreMediaError(err)
	}
	if reservation.Token != input.Token || len(reservation.References) != len(publicIDs) {
		return legacymedia.Reservation{}, ErrMediaInvalid
	}
	result := legacymedia.Reservation{Token: reservation.Token, References: make([]legacymedia.Reference, 0, len(reservation.References))}
	for _, reference := range reservation.References {
		if reference.InternalID == 0 || reference.AssetPublicID == "" {
			return legacymedia.Reservation{}, ErrMediaInvalid
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
	if !coreMediaToken(reservation.Token) {
		return mapLegacyMediaError(g.legacy.Commit(ctx, ownerID, reservation, referenceType, referenceID))
	}
	input, err := coreBindingCommitInput(ownerID, reservation.Token, referenceType, referenceID, reservationPublicIDs(reservation))
	if err != nil {
		return err
	}
	return mapCoreMediaError(g.core.Commit(ctx, input))
}

func (g *HybridMediaGateway) EnsureCommitted(ctx context.Context, ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) error {
	if !coreMediaToken(token) {
		return mapLegacyMediaError(g.legacy.EnsureCommitted(ctx, ownerID, token, referenceType, referenceID, orderedIDs))
	}
	input, err := coreBindingCommitInput(ownerID, token, referenceType, referenceID, orderedIDs)
	if err != nil {
		return err
	}
	return mapCoreMediaError(g.core.EnsureCommitted(ctx, input))
}

func (g *HybridMediaGateway) Cancel(ctx context.Context, ownerID uint, reservation legacymedia.Reservation) error {
	if !coreMediaToken(reservation.Token) {
		return mapLegacyMediaError(g.legacy.Cancel(ctx, ownerID, reservation))
	}
	if ownerID == 0 {
		return ErrMediaInvalid
	}
	return mapCoreMediaError(g.core.Cancel(ctx, uint64(ownerID), reservation.Token))
}

func (g *HybridMediaGateway) DeleteReference(ctx context.Context, referenceType string, referenceID uint64, keep []uint64) error {
	if referenceType != "product" || referenceID == 0 {
		return ErrMediaInvalid
	}
	coreErr := g.core.Revoke(ctx, coremedia.PurposeProduct, strconv.FormatUint(referenceID, 10), append([]uint64(nil), keep...))
	legacyErr := g.legacy.DeleteReference(ctx, referenceType, referenceID, append([]uint64(nil), keep...))
	return errors.Join(mapCoreMediaError(coreErr), mapLegacyMediaError(legacyErr))
}

func (g *HybridMediaGateway) PublishProduct(ctx context.Context, productID uint64) error {
	if productID == 0 {
		return ErrMediaInvalid
	}
	err := g.core.Publish(ctx, coremedia.PurposeProduct, strconv.FormatUint(productID, 10))
	if errors.Is(err, coremedia.ErrNotFound) {
		return nil
	}
	return mapCoreMediaError(err)
}

func coreBindingCommitInput(ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) (coremedia.BindingCommitInput, error) {
	if ownerID == 0 || referenceType != "product" || referenceID == 0 || len(orderedIDs) == 0 {
		return coremedia.BindingCommitInput{}, ErrMediaInvalid
	}
	for _, publicID := range orderedIDs {
		if strings.TrimSpace(publicID) == "" {
			return coremedia.BindingCommitInput{}, ErrMediaInvalid
		}
	}
	return coremedia.BindingCommitInput{
		OwnerID: uint64(ownerID), Token: token, ReferenceType: coremedia.PurposeProduct,
		ReferenceID: strconv.FormatUint(referenceID, 10), OrderedAssetPublicIDs: append([]string(nil), orderedIDs...),
	}, nil
}

func reservationPublicIDs(reservation legacymedia.Reservation) []string {
	result := make([]string, 0, len(reservation.References))
	for _, reference := range reservation.References {
		result = append(result, reference.PublicID)
	}
	return result
}

func coreMediaToken(token string) bool { return strings.HasPrefix(token, "m10_") }

func mapCoreMediaError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, coremedia.ErrBindingConflict):
		return fmt.Errorf("%w: %v", ErrMediaConflict, err)
	case errors.Is(err, coremedia.ErrBindingInvalidAsset), errors.Is(err, coremedia.ErrBindingMismatch),
		errors.Is(err, coremedia.ErrBindingLegacySet), errors.Is(err, coremedia.ErrInvalidULID),
		errors.Is(err, coremedia.ErrInvalidRecord), errors.Is(err, coremedia.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrMediaInvalid, err)
	default:
		return err
	}
}

func mapLegacyMediaError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, legacymedia.ErrAlreadyBound):
		return fmt.Errorf("%w: %v", ErrMediaConflict, err)
	case errors.Is(err, legacymedia.ErrInvalidAsset), errors.Is(err, legacymedia.ErrNotFound), errors.Is(err, legacymedia.ErrForbidden):
		return fmt.Errorf("%w: %v", ErrMediaInvalid, err)
	default:
		return err
	}
}

var (
	_ legacymedia.Gateway   = (*HybridMediaGateway)(nil)
	_ MediaEnsurer          = (*HybridMediaGateway)(nil)
	_ productMediaUpdater   = (*HybridMediaGateway)(nil)
	_ productMediaPublisher = (*HybridMediaGateway)(nil)
)
