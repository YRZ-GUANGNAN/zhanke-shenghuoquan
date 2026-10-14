package user

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

var (
	ErrCoverInvalid     = errors.New("无效的封面图片")
	ErrCoverForbidden   = errors.New("当前账号不能修改封面")
	ErrCoverUnavailable = errors.New("封面服务暂不可用")
)

type ProfileCover struct {
	URL     string
	AssetID *uint64
}

// The callback and media mutations share the same database transaction.
type ProfileCoverRepository interface {
	WithLockedCover(context.Context, uint, func(context.Context, User) (ProfileCover, error)) (User, error)
}

type profileCoverBindings interface {
	Reserve(context.Context, coremedia.BindingReserveInput) (coremedia.BindingReservation, error)
	Commit(context.Context, coremedia.BindingCommitInput) error
	Publish(context.Context, coremedia.Purpose, string) error
	Revoke(context.Context, coremedia.Purpose, string, []uint64) error
}

type ProfileCoverService struct {
	repo       ProfileCoverRepository
	bindings   profileCoverBindings
	publicBase string
}

func NewProfileCoverService(repo ProfileCoverRepository, bindings profileCoverBindings, publicBase string) (*ProfileCoverService, error) {
	if repo == nil || bindings == nil {
		return nil, ErrCoverUnavailable
	}
	return &ProfileCoverService{repo: repo, bindings: bindings, publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/")}, nil
}

func (s *ProfileCoverService) Update(ctx context.Context, userID uint, assetID string) (User, error) {
	if userID == 0 || (assetID != "" && !coremedia.ValidULID(assetID)) {
		return User{}, ErrCoverInvalid
	}
	return s.repo.WithLockedCover(ctx, userID, func(txctx context.Context, current User) (ProfileCover, error) {
		if current.Status != StatusActive {
			return ProfileCover{}, ErrCoverForbidden
		}
		referenceID := strconv.FormatUint(uint64(userID), 10)
		if assetID == "" {
			if err := s.bindings.Revoke(txctx, coremedia.PurposeProfileCover, referenceID, nil); err != nil {
				return ProfileCover{}, err
			}
			return ProfileCover{}, nil
		}
		operationID, err := coremedia.NewULID()
		if err != nil {
			return ProfileCover{}, err
		}
		token := "m10_" + operationID
		reservation, err := s.bindings.Reserve(txctx, coremedia.BindingReserveInput{
			OwnerID: uint64(userID), Purpose: coremedia.PurposeProfileCover, Token: token,
			OrderedAssetPublicIDs: []string{assetID}, ExistingReferenceID: referenceID,
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		})
		if err != nil {
			return ProfileCover{}, err
		}
		if reservation.Token != token || len(reservation.References) != 1 || reservation.References[0].InternalID == 0 || reservation.References[0].AssetPublicID != assetID {
			return ProfileCover{}, coremedia.ErrBindingMismatch
		}
		if err := s.bindings.Commit(txctx, coremedia.BindingCommitInput{
			OwnerID: uint64(userID), Token: token, ReferenceType: coremedia.PurposeProfileCover,
			ReferenceID: referenceID, OrderedAssetPublicIDs: []string{assetID},
		}); err != nil {
			return ProfileCover{}, err
		}
		if err := s.bindings.Publish(txctx, coremedia.PurposeProfileCover, referenceID); err != nil {
			return ProfileCover{}, err
		}
		internalID := reservation.References[0].InternalID
		if err := s.bindings.Revoke(txctx, coremedia.PurposeProfileCover, referenceID, []uint64{internalID}); err != nil {
			return ProfileCover{}, err
		}
		return ProfileCover{URL: s.publicBase + "/api/v1/media/assets/" + assetID + "/content", AssetID: &internalID}, nil
	})
}

func (h *Handler) SetProfileCovers(service *ProfileCoverService) { h.covers = service }

func (h *Handler) updateCover(c *gin.Context) {
	var input struct {
		AssetID *string `json:"assetId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.AssetID == nil {
		response.Fail(c, http.StatusBadRequest, "请选择封面图片，或明确恢复默认背景")
		return
	}
	if h.covers == nil {
		response.Fail(c, http.StatusServiceUnavailable, ErrCoverUnavailable.Error())
		return
	}
	value, err := h.covers.Update(c.Request.Context(), CurrentUserID(c), *input.AssetID)
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, ErrCoverInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrCoverForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, "账号不存在")
	case errors.Is(err, coremedia.ErrBindingInvalidAsset), errors.Is(err, coremedia.ErrBindingConflict), errors.Is(err, coremedia.ErrBindingMismatch), errors.Is(err, coremedia.ErrBindingLegacySet), errors.Is(err, coremedia.ErrNotFound):
		response.Fail(c, http.StatusConflict, "封面不可用，请确认图片已通过审核并重新选择")
	default:
		response.Fail(c, http.StatusInternalServerError, "封面保存失败，请重试")
	}
}
