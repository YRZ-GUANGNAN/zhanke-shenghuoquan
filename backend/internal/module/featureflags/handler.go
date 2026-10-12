package featureflags

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func PublicHandler(c *gin.Context) {
	value, err := ReadPolicy(false)
	c.Header("Cache-Control", "no-store")
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	response.OK(c, gin.H{"flags": value.Flags, "postingMode": value.PostingMode, "rowVersion": value.RowVersion, "enabled": value.Enabled})
}

func AdminGetHandler(c *gin.Context) {
	if !LabEnabled() {
		response.Fail(c, http.StatusServiceUnavailable, "功能配置服务未启用")
		return
	}
	value, err := ReadPolicy(true)
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, value)
}

func AdminUpdateHandler(c *gin.Context) {
	if !LabEnabled() {
		response.Fail(c, http.StatusServiceUnavailable, "功能配置服务未启用")
		return
	}
	actor := user.CurrentUser(c)
	if actor.Role != user.RoleSuperAdmin || actor.Status != user.StatusActive {
		response.Fail(c, http.StatusForbidden, "仅启用中的超级管理员可修改功能配置")
		return
	}
	var input UpdateInput
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid feature policy")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.Fail(c, http.StatusBadRequest, "invalid feature policy")
		return
	}
	value, err := UpdateContext(c.Request.Context(), user.CurrentUserID(c), input)
	switch {
	case errors.Is(err, ErrVersionConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrLabDisabled):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrInvalidKey), errors.Is(err, ErrInvalidPostingMode):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case err != nil:
		response.Fail(c, http.StatusInternalServerError, "update feature policy failed")
	default:
		response.OK(c, value)
	}
}
