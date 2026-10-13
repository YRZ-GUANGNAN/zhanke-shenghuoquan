package campusinfo

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

//go:embed content.json
var content []byte

type Section struct {
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
}
type Guide struct {
	ID         string    `json:"id"`
	Category   string    `json:"category"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	VerifiedAt string    `json:"verifiedAt"`
	SourceName string    `json:"sourceName"`
	SourceURL  string    `json:"sourceUrl"`
	Sections   []Section `json:"sections"`
}
type Handler struct{ guides []Guide }

var validID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func NewHandler() (*Handler, error) {
	var guides []Guide
	if err := json.Unmarshal(content, &guides); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	if len(guides) == 0 {
		return nil, fmt.Errorf("campus guides are empty")
	}
	for _, guide := range guides {
		u, err := url.Parse(guide.SourceURL)
		_, dateErr := time.Parse("2006-01-02", guide.VerifiedAt)
		if !validID.MatchString(guide.ID) || seen[guide.ID] || strings.TrimSpace(guide.Title) == "" || guide.Category == "" || guide.Summary == "" || guide.SourceName == "" || len(guide.Sections) == 0 || dateErr != nil || err != nil || u.Scheme != "https" || u.Host != "www.zjkju.edu.cn" || u.User != nil {
			return nil, fmt.Errorf("invalid campus guide %q", guide.ID)
		}
		for _, section := range guide.Sections {
			if section.Title == "" || len(section.Paragraphs) == 0 {
				return nil, fmt.Errorf("empty section in %q", guide.ID)
			}
			for _, paragraph := range section.Paragraphs {
				if strings.TrimSpace(paragraph) == "" {
					return nil, fmt.Errorf("empty paragraph in %q", guide.ID)
				}
			}
		}
		seen[guide.ID] = true
	}
	return &Handler{guides: guides}, nil
}

func (h *Handler) Register(api *gin.RouterGroup) {
	api.GET("/campus-info/guides", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		response.OK(c, gin.H{"items": h.guides})
	})
	api.GET("/campus-info/guides/:id", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		for _, guide := range h.guides {
			if guide.ID == c.Param("id") {
				response.OK(c, guide)
				return
			}
		}
		response.Fail(c, http.StatusNotFound, "这篇指南不存在或已调整，请返回首页查看")
	})
}
