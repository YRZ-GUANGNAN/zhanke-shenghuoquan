package miniappflow

import (
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type WechatCallbackHandler struct {
	service       *Service
	callbackToken string
}

func NewWechatCallbackHandler(service *Service, callbackToken string) *WechatCallbackHandler {
	return &WechatCallbackHandler{service: service, callbackToken: callbackToken}
}

func (h *WechatCallbackHandler) Register(group *gin.RouterGroup) {
	group.GET("/wechat/callback", h.verify)
	group.POST("/wechat/callback", h.callback)
}

func (h *WechatCallbackHandler) verify(c *gin.Context) {
	if !h.valid(c) {
		c.Status(http.StatusForbidden)
		return
	}
	c.String(http.StatusOK, c.Query("echostr"))
}

type wechatReviewResult struct {
	TraceID string `json:"trace_id" xml:"trace_id"`
	Result  struct {
		Suggest string `json:"suggest" xml:"suggest"`
		Label   int    `json:"label" xml:"label"`
	} `json:"result" xml:"result"`
}

func (h *WechatCallbackHandler) callback(c *gin.Context) {
	if !h.valid(c) {
		c.Status(http.StatusForbidden)
		return
	}
	var value wechatReviewResult
	var err error
	if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "xml") {
		err = xml.NewDecoder(c.Request.Body).Decode(&value)
	} else {
		err = c.ShouldBindJSON(&value)
	}
	if err != nil || strings.TrimSpace(value.TraceID) == "" || strings.TrimSpace(value.Result.Suggest) == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.service.ApplyWechatReviewResult(c.Request.Context(), value.TraceID, value.Result.Suggest, value.Result.Label); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}

func (h *WechatCallbackHandler) valid(c *gin.Context) bool {
	return VerifyWechatReviewSignature(
		h.callbackToken,
		c.Query("timestamp"),
		c.Query("nonce"),
		c.Query("signature"),
	)
}
