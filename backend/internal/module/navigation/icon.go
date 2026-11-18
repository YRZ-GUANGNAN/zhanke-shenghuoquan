package navigation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

const maxIconBytes = 1 << 20 // 1MB

// registerIconAdmin mounts the administrator upload endpoints. The caller
// supplies the admin session middleware via its route group.
func (h *Handler) RegisterAdminIcons(rg *gin.RouterGroup) {
	rg.POST("/items/:id/icon", h.uploadItemIcon)
	rg.DELETE("/items/:id/icon", h.deleteItemIcon)
}

// uploadItemIcon stores a content-addressed PNG/JPG and binds it to the item.
func (h *Handler) uploadItemIcon(c *gin.Context) {
	if h.iconDir == "" || h.publicBase == "" {
		response.Fail(c, http.StatusServiceUnavailable, "图标上传暂未开启")
		return
	}
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "无效的入口编号")
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size <= 0 {
		response.Fail(c, http.StatusBadRequest, "请选择要上传的图标")
		return
	}
	if file.Size > maxIconBytes {
		response.Fail(c, http.StatusBadRequest, "图标需在 1MB 以内")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无法读取上传的图标")
		return
	}
	defer src.Close()
	payload, err := io.ReadAll(io.LimitReader(src, maxIconBytes+1))
	if err != nil || len(payload) == 0 || len(payload) > maxIconBytes {
		response.Fail(c, http.StatusBadRequest, "图标需在 1MB 以内")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "图标格式无法识别，请使用 JPG 或 PNG")
		return
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 1024 || config.Height > 1024 {
		response.Fail(c, http.StatusBadRequest, "图标尺寸需在 1024×1024 以内")
		return
	}
	ext := ""
	switch format {
	case "jpeg":
		ext = ".jpg"
	case "png":
		ext = ".png"
	default:
		response.Fail(c, http.StatusBadRequest, "仅支持 JPG 或 PNG 图片")
		return
	}
	sum := sha256.Sum256(payload)
	name := hex.EncodeToString(sum[:16]) + ext
	if err := os.MkdirAll(h.iconDir, 0o750); err != nil {
		response.Fail(c, http.StatusInternalServerError, "保存图标失败")
		return
	}
	target := filepath.Join(h.iconDir, name)
	if _, statErr := os.Stat(target); errors.Is(statErr, os.ErrNotExist) {
		if err := os.WriteFile(target, payload, 0o644); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存图标失败")
			return
		}
	}
	if _, err := h.service.SetItemIconPath(c.Request.Context(), uint(id), name); err != nil {
		writeNavigationError(c, err)
		return
	}
	response.OK(c, gin.H{"iconPath": name, "iconUrl": h.publicBase + "/api/v1/navigation-icons/" + name})
}

func (h *Handler) deleteItemIcon(c *gin.Context) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "无效的入口编号")
		return
	}
	if _, err := h.service.SetItemIconPath(c.Request.Context(), uint(id), ""); err != nil {
		writeNavigationError(c, err)
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

// serveIcon serves uploaded icons publicly; names are content hashes, so the
// responses are immutable and safe to cache aggressively.
func (h *Handler) serveIcon(c *gin.Context) {
	if h.iconDir == "" {
		c.Status(http.StatusNotFound)
		return
	}
	name := c.Param("name")
	if !iconPathPattern.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	if strings.HasSuffix(name, ".jpg") {
		c.Header("Content-Type", "image/jpeg")
	} else {
		c.Header("Content-Type", "image/png")
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(filepath.Join(h.iconDir, name))
}

func writeNavigationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, "入口不存在")
	case errors.Is(err, ErrInvalidNavigationInput):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "保存图标失败")
	}
}
