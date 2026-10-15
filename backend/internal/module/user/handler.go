package user

import (
	"errors"
	"net/http"
	"strconv"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/media"
	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service  *Service
	media    media.Gateway
	covers   *ProfileCoverService
	profiles *ProfileUpdateService
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
	rg.PUT("/me/cover", h.service.AuthRequired(), UnmutedRequired(), h.updateCover)
	rg.GET("/identity", h.service.AuthRequired(), h.identity)
	rg.POST("/identity", h.service.AuthRequired(), h.switchIdentity)
	rg.GET("/settings", h.service.AuthRequired(), h.settings)
	rg.PATCH("/settings", h.service.AuthRequired(), h.updateSettings)
	rg.POST("/me/phone", h.service.AuthRequired(), h.bindPhone)
	rg.GET("/blocks", h.service.AuthRequired(), h.blocks)
	rg.POST("/blocks/:id", h.service.AuthRequired(), h.addBlock)
	rg.DELETE("/blocks/:id", h.service.AuthRequired(), h.removeBlock)
	rg.POST("/feedbacks", h.service.AuthRequired(), h.submitFeedback)
	rg.DELETE("/me", h.service.AuthRequired(), h.closeAccount)
}

func (h *Handler) settings(c *gin.Context) {
	value, err := h.service.Settings(c, CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "get settings failed")
		return
	}
	response.OK(c, value)
}

func (h *Handler) updateSettings(c *gin.Context) {
	var request UserSettings
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid settings")
		return
	}
	value, err := h.service.UpdateSettings(c, CurrentUserID(c), request)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "update settings failed")
		return
	}
	response.OK(c, value)
}

func (h *Handler) bindPhone(c *gin.Context) {
	var request struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid phone code")
		return
	}
	value, err := h.service.BindPhone(c, CurrentUserID(c), request.Code)
	if err != nil {
		status := http.StatusBadRequest
		message := "手机号绑定失败，请稍后重试"
		if errors.Is(err, ErrPhoneBindingUnavailable) {
			status = http.StatusServiceUnavailable
			message = "手机号绑定暂不可用"
		} else if errors.Is(err, ErrInvalidPhone) {
			message = "微信返回的手机号无效"
		}
		response.Fail(c, status, message)
		return
	}
	response.OK(c, value)
}

func blockTargetID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) blocks(c *gin.Context) {
	values, err := h.service.Blocks(c, CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list blocks failed")
		return
	}
	response.OK(c, gin.H{"items": values})
}

func (h *Handler) addBlock(c *gin.Context) {
	targetID, ok := blockTargetID(c)
	if !ok {
		return
	}
	if err := h.service.AddBlock(c, CurrentUserID(c), targetID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		response.Fail(c, status, err.Error())
		return
	}
	response.OK(c, gin.H{"blocked": true})
}

func (h *Handler) removeBlock(c *gin.Context) {
	targetID, ok := blockTargetID(c)
	if !ok {
		return
	}
	if err := h.service.RemoveBlock(c, CurrentUserID(c), targetID); err != nil {
		response.Fail(c, http.StatusInternalServerError, "remove block failed")
		return
	}
	response.OK(c, gin.H{"blocked": false})
}

func (h *Handler) submitFeedback(c *gin.Context) {
	var request struct {
		Category      string   `json:"category"`
		Content       string   `json:"content"`
		ImageAssetIDs []string `json:"imageAssetIds"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid feedback")
		return
	}
	value, err := h.service.SubmitFeedback(c, CurrentUserID(c), request.Category, request.Content, request.ImageAssetIDs)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrFeedbackUnavailable) {
			status = http.StatusServiceUnavailable
		}
		response.Fail(c, status, err.Error())
		return
	}
	response.Created(c, value)
}

func (h *Handler) closeAccount(c *gin.Context) {
	err := h.service.CloseAccount(c, CurrentUserID(c))
	if err == nil {
		response.OK(c, gin.H{"closed": true})
		return
	}
	var blocked *AccountClosureBlockedError
	if errors.As(err, &blocked) {
		c.JSON(http.StatusConflict, response.Body{Code: http.StatusConflict, Message: "账号暂时无法注销", Data: gin.H{"blockers": blocked.Blockers}})
		return
	}
	if errors.Is(err, ErrAccountClosureUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	response.Fail(c, http.StatusInternalServerError, "close account failed")
}

func (h *Handler) updateMe(c *gin.Context) {
	var request ProfileUpdateInput
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid profile")
		return
	}
	if h.profiles != nil && h.updateCoreProfile(c, request) {
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
		if h.profiles != nil {
			if err := h.profiles.bindings.Revoke(c.Request.Context(), coremedia.PurposeAvatar, strconv.FormatUint(uint64(current.ID), 10), nil); err != nil {
				response.Fail(c, http.StatusInternalServerError, "旧头像清理失败，请重试")
				return
			}
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
