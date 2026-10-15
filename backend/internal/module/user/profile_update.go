package user

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ProfileUpdateRepository interface {
	WithLockedProfile(context.Context, uint, func(context.Context, User) (Profile, error)) (User, error)
}

type ProfileUpdateInput struct {
	Nickname      string `json:"nickname"`
	School        string `json:"school"`
	Campus        string `json:"campus"`
	AvatarAssetID string `json:"avatarAssetId"`
}

type ProfileUpdateService struct {
	repo       ProfileUpdateRepository
	bindings   profileCoverBindings
	publicBase string
	legacy     avatarLegacyRevoker
}

type avatarLegacyRevoker interface {
	DeleteReference(context.Context, string, uint64, []uint64) error
}

func (s *ProfileUpdateService) SetLegacyAvatars(legacy avatarLegacyRevoker) { s.legacy = legacy }

var ErrProfileInvalid = errors.New("请填写昵称，并选择有效的头像图片")

func NewProfileUpdateService(repo ProfileUpdateRepository, bindings profileCoverBindings, publicBase string) (*ProfileUpdateService, error) {
	if repo == nil || bindings == nil {
		return nil, ErrCoverUnavailable
	}
	return &ProfileUpdateService{repo: repo, bindings: bindings, publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/")}, nil
}

// Update serializes profile edits with cover edits and atomically binds the
// new avatar. No profile field is changed if publication or persistence fails.
func (s *ProfileUpdateService) Update(ctx context.Context, userID uint, in ProfileUpdateInput) (User, error) {
	if userID == 0 || strings.TrimSpace(in.Nickname) == "" || (in.AvatarAssetID != "" && !coremedia.ValidULID(in.AvatarAssetID)) {
		return User{}, ErrProfileInvalid
	}
	return s.repo.WithLockedProfile(ctx, userID, func(txctx context.Context, current User) (Profile, error) {
		if current.Status != StatusActive {
			return Profile{}, ErrCoverForbidden
		}
		value := current.Profile()
		value.Nickname, value.School, value.Campus = strings.TrimSpace(in.Nickname), strings.TrimSpace(in.School), strings.TrimSpace(in.Campus)
		if in.AvatarAssetID == "" {
			return value, nil
		}
		operationID, err := coremedia.NewULID()
		if err != nil {
			return Profile{}, err
		}
		token, referenceID := "m10_"+operationID, strconv.FormatUint(uint64(userID), 10)
		reservation, err := s.bindings.Reserve(txctx, coremedia.BindingReserveInput{
			OwnerID: uint64(userID), Purpose: coremedia.PurposeAvatar, Token: token,
			OrderedAssetPublicIDs: []string{in.AvatarAssetID}, ExistingReferenceID: referenceID,
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		})
		if err != nil {
			return Profile{}, err
		}
		if reservation.Token != token || len(reservation.References) != 1 || reservation.References[0].InternalID == 0 || reservation.References[0].AssetPublicID != in.AvatarAssetID || reservation.References[0].Slot != 0 {
			return Profile{}, coremedia.ErrBindingMismatch
		}
		if err := s.bindings.Commit(txctx, coremedia.BindingCommitInput{
			OwnerID: uint64(userID), Token: token, ReferenceType: coremedia.PurposeAvatar,
			ReferenceID: referenceID, OrderedAssetPublicIDs: []string{in.AvatarAssetID},
		}); err != nil {
			return Profile{}, err
		}
		if err := s.bindings.Publish(txctx, coremedia.PurposeAvatar, referenceID); err != nil {
			return Profile{}, err
		}
		internalID := reservation.References[0].InternalID
		if err := s.bindings.Revoke(txctx, coremedia.PurposeAvatar, referenceID, []uint64{internalID}); err != nil {
			return Profile{}, err
		}
		if s.legacy != nil {
			if err := s.legacy.DeleteReference(txctx, "user", uint64(userID), []uint64{internalID}); err != nil {
				return Profile{}, err
			}
		}
		value.Avatar, value.AvatarAssetID = s.publicBase+"/api/v1/media/assets/"+in.AvatarAssetID+"/content", &internalID
		return value, nil
	})
}

func (h *Handler) SetProfileUpdates(service *ProfileUpdateService) { h.profiles = service }

// Legacy 32-hex IDs still pass through the original owner/purpose/approval
// checks. Core errors never fall back to a more permissive media path.
func (h *Handler) updateCoreProfile(c *gin.Context, in ProfileUpdateInput) bool {
	if len(in.AvatarAssetID) == 32 {
		if _, err := hex.DecodeString(in.AvatarAssetID); err == nil {
			return false
		}
	}
	value, err := h.profiles.Update(c.Request.Context(), CurrentUserID(c), in)
	switch {
	case errors.Is(err, coremedia.ErrBindingLegacySet):
		return false
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, ErrProfileInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrCoverForbidden):
		response.Fail(c, http.StatusForbidden, "当前账号不能修改资料")
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, "账号不存在")
	case errors.Is(err, coremedia.ErrBindingInvalidAsset), errors.Is(err, coremedia.ErrBindingConflict), errors.Is(err, coremedia.ErrBindingMismatch), errors.Is(err, coremedia.ErrNotFound):
		response.Fail(c, http.StatusConflict, "头像不可用，请重新选择并完成上传")
	default:
		response.Fail(c, http.StatusInternalServerError, "资料保存失败，请重试")
	}
	return true
}
