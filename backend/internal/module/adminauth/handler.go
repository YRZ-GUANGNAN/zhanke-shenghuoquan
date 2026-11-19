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

	accounts := group.Group("/accounts", h.middleware.Required(), SuperAdminRequired())
	accounts.GET("", h.listAccounts)
	accounts.POST("", h.createAccount)
	accounts.PATCH("/:id", h.updateAccount)
	accounts.POST("/:id/status", h.setAccountStatus)
	accounts.POST("/:id/password", h.resetPassword)
}

func accountJSON(account Account) gin.H {
	result := gin.H{
		"id": account.ID, "username": account.Username, "displayName": account.DisplayName,
		"role": account.Role, "status": account.Status,
	}
	if account.LegacyUserID != nil {
		result["legacyUserId"] = *account.LegacyUserID
	}
	return result
}

const maxAuthBodyBytes int64 = 16 << 10

func (h *Handler) login(c *gin.Context) {
	// 密码长度不设策略上限；这里只做请求体大小保护，避免超长输入拖慢 Argon2。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAuthBodyBytes)
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
		"account":   accountJSON(result.Account),
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
		"account":   accountJSON(authenticated.Account),
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

func (h *Handler) listAccounts(c *gin.Context) {
	account, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	values, err := h.service.ListAccounts(c, account.Account.ID)
	if err != nil {
		writeAccountError(c, err)
		return
	}
	response.OK(c, gin.H{"items": values})
}

func (h *Handler) createAccount(c *gin.Context) {
	account, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	var request struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		Role        Role   `json:"role"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account request")
		return
	}
	value, err := h.service.CreateAccount(c, account.Account.ID, CreateAccountInput{
		Username: request.Username, DisplayName: request.DisplayName, Password: request.Password, Role: request.Role,
	})
	if err != nil {
		writeAccountError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) updateAccount(c *gin.Context) {
	account, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	targetID, convErr := strconv.ParseUint(c.Param("id"), 10, 32)
	if convErr != nil || targetID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account")
		return
	}
	var request struct {
		DisplayName string `json:"displayName"`
		Role        Role   `json:"role"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account request")
		return
	}
	value, err := h.service.UpdateAccount(c, account.Account.ID, targetID, request.DisplayName, request.Role)
	if err != nil {
		writeAccountError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) setAccountStatus(c *gin.Context) {
	account, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	targetID, convErr := strconv.ParseUint(c.Param("id"), 10, 32)
	if convErr != nil || targetID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account")
		return
	}
	var request struct {
		Status Status `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account request")
		return
	}
	value, err := h.service.SetAccountStatus(c, account.Account.ID, targetID, request.Status)
	if err != nil {
		writeAccountError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) resetPassword(c *gin.Context) {
	account, err := AuthenticatedFromContext(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	targetID, convErr := strconv.ParseUint(c.Param("id"), 10, 32)
	if convErr != nil || targetID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account")
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid administrator account request")
		return
	}
	if err := h.service.ResetPassword(c, account.Account.ID, targetID, request.Password); err != nil {
		writeAccountError(c, err)
		return
	}
	response.OK(c, gin.H{"reset": true})
}

func writeAccountError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, ErrNotSuperAdmin):
		response.Fail(c, http.StatusForbidden, "only a super administrator can manage administrator accounts")
	case errors.Is(err, ErrCannotModifySelf):
		response.Fail(c, http.StatusBadRequest, "cannot disable or demote yourself")
	case errors.Is(err, ErrLastActiveSuperAdmin):
		response.Fail(c, http.StatusConflict, "cannot disable or demote the last active superadmin")
	case errors.Is(err, ErrUsernameTaken):
		response.Fail(c, http.StatusConflict, "this administrator username is already in use")
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidRole),
		errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrInvalidDisplayName),
		errors.Is(err, ErrPasswordPolicy):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrCredentialNotFound):
		response.Fail(c, http.StatusNotFound, "administrator account not found")
	default:
		response.Fail(c, http.StatusInternalServerError, "administrator account operation failed")
	}
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(CookieName, value, maxAge, "/", "", true, true)
}
