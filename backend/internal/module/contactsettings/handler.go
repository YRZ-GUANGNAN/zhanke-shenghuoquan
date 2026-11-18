package contactsettings

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

const maxRequestBodyBytes int64 = 8 << 10

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register mounts a public read for the miniapp and an admin update endpoint.
// The caller supplies the admin cookie/CSRF authentication middleware.
func (h *Handler) Register(api *gin.RouterGroup, adminAuth gin.HandlerFunc) {
	api.GET("/contact-settings", h.get)
	secured := api.Group("/admin/contact-settings", adminAuth, user.AdminRequired())
	secured.GET("", h.get)
	secured.PUT("", h.update)
}

func (h *Handler) get(c *gin.Context) {
	settings, err := h.service.Get(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "load contact settings failed")
		return
	}
	response.OK(c, settings)
}

func (h *Handler) update(c *gin.Context) {
	var input UpdateInput
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid contact settings payload")
		return
	}
	settings, err := h.service.Update(c.Request.Context(), input)
	switch {
	case err == nil:
		response.OK(c, settings)
	case errors.Is(err, ErrConflict):
		response.Fail(c, http.StatusConflict, "contact settings were updated; reload before saving")
	case errors.Is(err, ErrInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "update contact settings failed")
	}
}
