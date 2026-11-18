package navigation

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service    *Service
	iconDir    string
	publicBase string
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// WithIconStorage enables custom navigation entry icons served from dir.
func (h *Handler) WithIconStorage(dir, publicBase string) *Handler {
	h.iconDir = strings.TrimSpace(dir)
	h.publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return h
}

// publicIconURL resolves the stored icon file name to its public HTTPS URL.
func (h *Handler) publicIconURL(iconPath string) string {
	if iconPath == "" || h.iconDir == "" || h.publicBase == "" {
		return ""
	}
	return h.publicBase + "/api/v1/navigation-icons/" + iconPath
}

func (h *Handler) decorateCategories(values []PublicCategory) {
	for ci := range values {
		for ii := range values[ci].Items {
			item := &values[ci].Items[ii]
			if item.IconPath != "" {
				item.IconURL = h.publicIconURL(item.IconPath)
			}
		}
	}
}
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
		h.decorateCategories(filtered)
		response.OK(c, filtered)
	})
	rg.GET("/navigation-icons/:name", h.serveIcon)
}
