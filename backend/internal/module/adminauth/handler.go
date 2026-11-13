package adminauth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service    *Service
	middleware *Middleware
}

func NewHandler(service *Service, middleware *Middleware) *Handler {
	return &Handler{service: service, middleware: middleware}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.POST("/login", h.login)
	group.GET("/session", h.middleware.Required(), h.session)
	group.POST("/logout", h.middleware.Required(), h.logout)
	group.POST("/password/change", h.middleware.Required(), h.changePassword)
	group.GET("/credentials", h.middleware.Required(), h.listCredentials)
	group.PUT("/credentials/:userId", h.middleware.Required(), h.setCredential)
	group.DELETE("/credentials/:userId", h.middleware.Required(), h.removeCredential)
}

func (h *Handler) listCredentials(c *gin.Context) {
	authenticated, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	values, err := h.service.ListCredentials(c, authenticated.User.ID)
	if err != nil {
		if errors.Is(err, ErrNotSuperAdmin) {
			response.Fail(c, http.StatusForbidden, "only a super administrator can manage administrator accounts")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "administrator accounts unavailable")
		return
	}
	response.OK(c, gin.H{"items": values})
}

func (h *Handler) setCredential(c *gin.Context) {
	authenticated, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	userID, convErr := strconv.ParseUint(c.Param("userId"), 10, 32)
	if convErr != nil || userID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid administrator user")
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account request")
		return
	}
	_, err = h.service.SetCredential(c, authenticated.User.ID, uint(userID), request.Username, request.Password, "credential_set")
	switch {
	case err == nil:
		response.OK(c, gin.H{"saved": true})
	case errors.Is(err, ErrNotSuperAdmin):
		response.Fail(c, http.StatusForbidden, "only a super administrator can manage administrator accounts")
	case errors.Is(err, ErrTargetNotAdmin):
		response.Fail(c, http.StatusBadRequest, "target user is not an active administrator")
	case errors.Is(err, ErrUsernameInvalid):
		response.Fail(c, http.StatusBadRequest, ErrUsernameInvalid.Error())
	case errors.Is(err, ErrUsernameTaken):
		response.Fail(c, http.StatusConflict, "this administrator username is already in use")
	case errors.Is(err, ErrPasswordPolicy):
		response.Fail(c, http.StatusBadRequest, ErrPasswordPolicy.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "save administrator account failed")
	}
}

func (h *Handler) removeCredential(c *gin.Context) {
	authenticated, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	userID, convErr := strconv.ParseUint(c.Param("userId"), 10, 32)
	if convErr != nil || userID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid administrator user")
		return
	}
	err = h.service.RemoveCredential(c, authenticated.User.ID, uint(userID))
	switch {
	case err == nil:
		response.OK(c, gin.H{"removed": true})
	case errors.Is(err, ErrNotSuperAdmin):
		response.Fail(c, http.StatusForbidden, "only a super administrator can manage administrator accounts")
	default:
		response.Fail(c, http.StatusBadRequest, "remove administrator account failed")
	}
}

func (h *Handler) login(c *gin.Context) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusUnauthorized, "invalid administrator credentials")
		return
	}
	result, err := h.service.Login(c, request.Username, request.Password, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		if errors.Is(err, ErrLocked) {
			response.Fail(c, http.StatusTooManyRequests, "administrator login temporarily unavailable")
			return
		}
		response.Fail(c, http.StatusUnauthorized, "invalid administrator credentials")
		return
	}
	setSessionCookie(c, result.Token, int(time.Until(result.ExpiresAt).Seconds()))
	response.OK(c, gin.H{
		"user":      gin.H{"id": result.User.ID, "nickname": result.User.Nickname, "role": result.User.Role},
		"csrfToken": result.CSRFToken, "expiresAt": result.ExpiresAt,
	})
}

func (h *Handler) session(c *gin.Context) {
	authenticated, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	response.OK(c, gin.H{
		"user":      gin.H{"id": authenticated.User.ID, "nickname": authenticated.User.Nickname, "role": authenticated.User.Role},
		"csrfToken": authenticated.CSRFToken, "expiresAt": authenticated.Session.AbsoluteExpiresAt,
	})
}

func (h *Handler) logout(c *gin.Context) {
	token, err := c.Cookie(CookieName)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := h.service.Logout(c, token, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	setSessionCookie(c, "", -1)
	response.OK(c, gin.H{"loggedOut": true})
}

func (h *Handler) changePassword(c *gin.Context) {
	token, err := c.Cookie(CookieName)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	var request struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid password change request")
		return
	}
	err = h.service.ChangePassword(c, token, request.OldPassword, request.NewPassword, c.ClientIP(), c.GetHeader("User-Agent"))
	switch {
	case err == nil:
		setSessionCookie(c, "", -1)
		response.OK(c, gin.H{"passwordChanged": true})
	case errors.Is(err, ErrPasswordPolicy):
		response.Fail(c, http.StatusBadRequest, ErrPasswordPolicy.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, "current password is invalid")
	default:
		response.Fail(c, http.StatusInternalServerError, "change password failed")
	}
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(CookieName, value, maxAge, "/", "", true, true)
}
