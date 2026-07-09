package user

import (
	"errors"
	"net/http"

	"campus-circle/backend/internal/pkg/media"
	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service *Service
	media   media.Gateway
}

func NewHandler(service *Service, mediaGateway ...media.Gateway) *Handler {
	var gateway media.Gateway
	if len(mediaGateway) > 0 {
		gateway = mediaGateway[0]
	}
	return &Handler{service: service, media: gateway}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/wechat-login", h.login)
	rg.GET("/me", h.service.AuthRequired(), h.me)
	rg.PATCH("/me", h.service.AuthRequired(), UnmutedRequired(), h.updateMe)
	rg.GET("/identity", h.service.AuthRequired(), h.identity)
	rg.POST("/identity", h.service.AuthRequired(), h.switchIdentity)
}

func (h *Handler) updateMe(c *gin.Context) {
	var request struct {
		Nickname      string `json:"nickname"`
		School        string `json:"school"`
		Campus        string `json:"campus"`
		AvatarAssetID string `json:"avatarAssetId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid profile")
		return
	}
	current := CurrentUser(c)
	profile := Profile{ID: current.ID, Nickname: request.Nickname, Avatar: current.Avatar, AvatarAssetID: current.AvatarAssetID, School: request.School, Campus: request.Campus}
	reservation := media.Reservation{}
	if request.AvatarAssetID != "" {
		if h.media == nil {
			response.Fail(c, http.StatusServiceUnavailable, "media pipeline is unavailable")
			return
		}
		var err error
		reservation, err = h.media.Reserve(c, current.ID, media.PurposeAvatar, []string{request.AvatarAssetID})
		if err != nil {
			response.Fail(c, http.StatusConflict, err.Error())
			return
		}
		assetID := reservation.References[0].InternalID
		profile.Avatar, profile.AvatarAssetID = reservation.References[0].URL, &assetID
	}
	value, err := h.service.UpdateProfile(c, current.ID, profile)
	if err != nil {
		if h.media != nil && reservation.Token != "" {
			_ = h.media.Cancel(c, current.ID, reservation)
		}
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if reservation.Token != "" {
		if err := h.media.Commit(c, current.ID, reservation, "user", uint64(current.ID)); err != nil {
			// The user row was updated before the cross-domain media commit. Restore
			// the previous profile so a failed commit cannot leave a broken avatar.
			_, _ = h.service.UpdateProfile(c, current.ID, current.Profile())
			_ = h.media.Cancel(c, current.ID, reservation)
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		keep := []uint64{reservation.References[0].InternalID}
		if err := h.media.DeleteReference(c, "user", uint64(current.ID), keep); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	response.OK(c, value)
}

func (h *Handler) login(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
		School   string `json:"school"`
		Campus   string `json:"campus"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	result, err := h.service.WechatLogin(c, LoginInput{Code: req.Code, Nickname: req.Nickname, Avatar: req.Avatar, School: req.School, Campus: req.Campus})
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) me(c *gin.Context) {
	value, err := h.service.Get(c, CurrentUserID(c))
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "get user failed")
		return
	}
	response.OK(c, value)
}

func (h *Handler) identity(c *gin.Context) {
	value, err := h.service.IdentitySummary(c, CurrentUserID(c))
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "get identity failed")
		return
	}
	response.OK(c, value)
}

func (h *Handler) switchIdentity(c *gin.Context) {
	var req struct {
		Identity Identity `json:"identity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.SwitchIdentity(c, CurrentUserID(c), req.Identity)
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, ErrInvalidIdentity):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrIdentityUnavailable):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "switch identity failed")
	}
}
