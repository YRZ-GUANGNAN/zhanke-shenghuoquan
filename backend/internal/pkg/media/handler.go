package media

import (
	"encoding/xml"
	"errors"
	"net/http"
	"strings"

	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service       *Service
	callbackToken string
	auth          gin.HandlerFunc
	unmuted       gin.HandlerFunc
	identity      func(*gin.Context) (uint, string)
}

func NewHandler(service *Service, callbackToken string, auth, unmuted gin.HandlerFunc, identity func(*gin.Context) (uint, string)) *Handler {
	return &Handler{service: service, callbackToken: callbackToken, auth: auth, unmuted: unmuted, identity: identity}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/assets/:id/content", h.content)
	rg.GET("/wechat/callback", h.verifyCallback)
	rg.POST("/wechat/callback", h.callback)
	auth := rg.Group("", h.auth)
	auth.POST("/uploads", h.unmuted, h.createUpload)
	auth.POST("/uploads/:id/complete", h.unmuted, h.complete)
	auth.GET("/uploads/:id", h.asset)
	auth.DELETE("/uploads/:id", h.cancel)
}

func (h *Handler) createUpload(c *gin.Context) {
	var request struct {
		Purpose        Purpose `json:"purpose"`
		FileName       string  `json:"fileName"`
		MIMEType       string  `json:"mimeType"`
		SizeBytes      int64   `json:"sizeBytes"`
		IdempotencyKey string  `json:"idempotencyKey"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid upload request")
		return
	}
	ownerID, _ := h.identity(c)
	ticket, err := h.service.CreateUpload(c, CreateUploadInput{OwnerUserID: ownerID, Purpose: request.Purpose, FileName: request.FileName, MIMEType: request.MIMEType, SizeBytes: request.SizeBytes, IdempotencyKey: request.IdempotencyKey})
	if err != nil {
		writeMediaError(c, err)
		return
	}
	response.Created(c, ticket)
}

func (h *Handler) complete(c *gin.Context) {
	ownerID, openID := h.identity(c)
	value, err := h.service.Complete(c, ownerID, openID, c.Param("id"))
	if err != nil {
		writeMediaError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) asset(c *gin.Context) {
	ownerID, _ := h.identity(c)
	value, err := h.service.Asset(c, ownerID, c.Param("id"))
	if err != nil {
		writeMediaError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) cancel(c *gin.Context) {
	ownerID, _ := h.identity(c)
	if err := h.service.CancelUpload(c, ownerID, c.Param("id")); err != nil {
		writeMediaError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *Handler) content(c *gin.Context) {
	value, err := h.service.ContentURL(c, c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Redirect(http.StatusFound, value)
}

func (h *Handler) verifyCallback(c *gin.Context) {
	if !h.validCallback(c) {
		c.Status(http.StatusForbidden)
		return
	}
	c.String(http.StatusOK, c.Query("echostr"))
}

type wechatResult struct {
	TraceID string `json:"trace_id" xml:"trace_id"`
	Result  struct {
		Suggest string `json:"suggest" xml:"suggest"`
		Label   int    `json:"label" xml:"label"`
	} `json:"result" xml:"result"`
}

func (h *Handler) callback(c *gin.Context) {
	if !h.validCallback(c) {
		c.Status(http.StatusForbidden)
		return
	}
	var value wechatResult
	contentType := c.GetHeader("Content-Type")
	var err error
	if strings.Contains(contentType, "xml") {
		err = xml.NewDecoder(c.Request.Body).Decode(&value)
	} else {
		err = c.ShouldBindJSON(&value)
	}
	if err != nil || value.TraceID == "" || value.Result.Suggest == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.service.ApplyWechatResult(c, value.TraceID, value.Result.Suggest, value.Result.Label); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}

func (h *Handler) validCallback(c *gin.Context) bool {
	return VerifyWechatSignature(h.callbackToken, c.Query("timestamp"), c.Query("nonce"), c.Query("signature"))
}

func writeMediaError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrInvalidAsset), errors.Is(err, ErrInvalidState), errors.Is(err, ErrAlreadyBound):
		status = http.StatusConflict
	}
	response.Fail(c, status, err.Error())
}
