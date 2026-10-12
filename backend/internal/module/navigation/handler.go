package navigation

import (
	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/categories", func(c *gin.Context) {
		value, err := h.service.PublicTree(c)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list navigation failed")
			return
		}
		filtered := value[:0]
		for _, category := range value {
			isTakeout := category.SystemKey != nil && *category.SystemKey == SystemKeyExternalTakeoutMerchants
			if isTakeout && !featureflags.Can(featureflags.KeyTakeout) {
				continue
			}
			if !isTakeout && !featureflags.Can(featureflags.KeyCampusTools) {
				continue
			}
			filtered = append(filtered, category)
		}
		response.OK(c, filtered)
	})
}
