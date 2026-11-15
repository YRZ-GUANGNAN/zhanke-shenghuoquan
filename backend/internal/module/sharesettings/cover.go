package sharesettings

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
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

const maxCoverBytes = 5 << 20

var coverNamePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png)$`)

// WithCoverStorage enables administrator cover uploads. dir must be a
// writable directory that survives releases; publicBase is the public HTTPS
// origin (for example https://zhanduiwang.top) used to build cover URLs.
func (h *Handler) WithCoverStorage(dir, publicBase string) *Handler {
	h.coverDir = strings.TrimSpace(dir)
	h.publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return h
}

// uploadCover accepts a multipart image, stores it content-addressed and
// returns the public HTTPS URL that can be saved as the share cover.
func (h *Handler) uploadCover(c *gin.Context) {
	if h.coverDir == "" || h.publicBase == "" {
		response.Fail(c, http.StatusServiceUnavailable, "封面图上传暂未开启，请联系管理员")
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size <= 0 {
		response.Fail(c, http.StatusBadRequest, "请选择要上传的图片")
		return
	}
	if file.Size > maxCoverBytes {
		response.Fail(c, http.StatusBadRequest, "图片需在 5MB 以内")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无法读取上传的图片")
		return
	}
	defer src.Close()
	payload, err := io.ReadAll(io.LimitReader(src, maxCoverBytes+1))
	if err != nil || len(payload) == 0 || len(payload) > maxCoverBytes {
		response.Fail(c, http.StatusBadRequest, "图片需在 5MB 以内")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "图片格式无法识别，请使用 JPG 或 PNG")
		return
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 {
		response.Fail(c, http.StatusBadRequest, "图片尺寸需在 4096×4096 以内")
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
	if err := os.MkdirAll(h.coverDir, 0o750); err != nil {
		response.Fail(c, http.StatusInternalServerError, "保存封面图失败")
		return
	}
	path := filepath.Join(h.coverDir, name)
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存封面图失败")
			return
		}
	}
	response.OK(c, gin.H{"url": h.publicBase + "/api/v1/share-covers/" + name})
}

// serveCover serves uploaded covers publicly; names are content hashes so the
// responses are immutable and safe to cache aggressively.
func (h *Handler) serveCover(c *gin.Context) {
	if h.coverDir == "" {
		c.Status(http.StatusNotFound)
		return
	}
	name := c.Param("name")
	if !coverNamePattern.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	if strings.HasSuffix(name, ".jpg") {
		c.Header("Content-Type", "image/jpeg")
	} else {
		c.Header("Content-Type", "image/png")
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(filepath.Join(h.coverDir, name))
}
