package sharesettings

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ store Store }

func NewHandler(store Store) *Handler { return &Handler{store: store} }

// The caller supplies the existing admin cookie/CSRF authentication middleware.
func (h *Handler) Register(api *gin.RouterGroup, adminAuth gin.HandlerFunc) {
	api.GET("/share-settings", h.get)
	secured := api.Group("/admin/share-settings", adminAuth, user.AdminRequired())
	secured.GET("", h.get)
	secured.PUT("", h.update)
}

func (h *Handler) get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	value, err := h.store.Load(c.Request.Context())
	if err != nil {
		response.Fail(c, 503, "暂时无法读取分享配置，请稍后重试")
		return
	}
	response.OK(c, value)
}

func (h *Handler) update(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var input UpdateInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.Fail(c, 400, "分享配置格式无效")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.Fail(c, 400, "分享配置格式无效")
		return
	}
	if input.ExpectedRowVersion == 0 {
		response.Fail(c, 400, "缺少配置版本，请重新读取")
		return
	}
	if err := input.Cards.Validate(); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	value, err := h.store.Save(c.Request.Context(), user.CurrentUserID(c), input)
	switch {
	case errors.Is(err, ErrConflict):
		response.Fail(c, 409, err.Error())
	case err != nil:
		response.Fail(c, 503, "保存失败，请重新读取以确认状态")
	default:
		response.OK(c, value)
	}
}
