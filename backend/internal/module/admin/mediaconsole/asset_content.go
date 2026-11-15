package mediaconsole

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

// ObjectURLSigner issues short-lived read URLs for finalized media objects.
// The production implementation is the Tencent COS completion store.
type ObjectURLSigner interface {
	SignedGetURL(ctx context.Context, key, versionID string, ttl time.Duration) (string, error)
}

const adminAssetURLTTL = 10 * time.Minute

// WithObjectSigner enables the administrator media preview route. The regular
// /api/v1/media/assets/:id/content gateway only serves assets already visible
// through a business reference; administrators need to preview every finalized
// asset, including pending-review and unreferenced ones.
func (h *Handler) WithObjectSigner(value ObjectURLSigner) *Handler {
	h.signer = value
	return h
}

// assetContent redirects to a signed COS URL for a finalized asset. It never
// serves pending or staging uploads: those have no final object yet.
func (h *Handler) assetContent(c *gin.Context) {
	if h.signer == nil {
		response.Fail(c, http.StatusServiceUnavailable, "media preview unavailable")
		return
	}
	key, versionID, err := h.service.AssetObject(c.Request.Context(), c.Param("assetId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, "media asset not found")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "media asset lookup failed")
		return
	}
	url, err := h.signer.SignedGetURL(c.Request.Context(), key, versionID, adminAssetURLTTL)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "media preview signing failed")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Redirect(http.StatusFound, url)
}
