package community

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"campus-circle/backend/internal/pkg/searchmetrics"
)

var ErrSearchUnavailable = errors.New("community search is unavailable")

type communitySearchRepository interface {
	SearchPosts(context.Context, uint, string, int, int, ...SearchFilters) ([]Post, int64, error)
	ListRecentTagPosts(context.Context, time.Time) ([]Post, error)
}

type HotTag struct {
	Tag              string `json:"tag"`
	UsageScore       int64  `json:"usageScore"`
	InteractionScore int64  `json:"interactionScore"`
}

func (s *Service) SearchPosts(ctx context.Context, viewerID uint, keyword string, page, size int, options ...SearchFilters) (Page[PostView], error) {
	filters := SearchFilters{}
	if len(options) > 0 {
		filters = options[0]
	}
	if err := validateSearchFilters(filters); err != nil {
		return Page[PostView]{}, err
	}
	display, _, err := searchmetrics.Normalize(keyword)
	if err != nil {
		return Page[PostView]{}, err
	}
	page, size = normalize(page, size)
	repository, ok := s.repo.(communitySearchRepository)
	if !ok {
		return Page[PostView]{}, ErrSearchUnavailable
	}
	values, total, err := repository.SearchPosts(ctx, viewerID, display, page, size, filters)
	if err != nil {
		return Page[PostView]{}, err
	}
	views, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: views, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) HotTags(ctx context.Context, now time.Time) ([]HotTag, error) {
	repository, ok := s.repo.(communitySearchRepository)
	if !ok {
		return nil, ErrSearchUnavailable
	}
	values, err := repository.ListRecentTagPosts(ctx, shanghaiTagDayStart(now).AddDate(0, 0, -6).UTC())
	if err != nil {
		return nil, err
	}
	return rankHotTags(values, now, 12), nil
}

type hotTagAggregate struct {
	tag         string
	usage       int64
	interaction int64
	latest      time.Time
}

func rankHotTags(posts []Post, now time.Time, limit int) []HotTag {
	nowDay := shanghaiTagDayStart(now)
	values := make(map[string]*hotTagAggregate)
	for _, post := range posts {
		days := int(nowDay.Sub(shanghaiTagDayStart(post.CreatedAt)).Hours() / 24)
		if days < 0 || days > 6 {
			continue
		}
		weight := int64(7 - days)
		for _, tag := range post.Tags {
			key := strings.ToLower(tag)
			item := values[key]
			if item == nil {
				item = &hotTagAggregate{tag: tag}
				values[key] = item
			}
			item.usage += weight
			item.interaction += (post.LikedCount + post.CommentCount*2 + post.FavoriteCount*2) * weight
			if post.CreatedAt.After(item.latest) {
				item.latest = post.CreatedAt
				item.tag = tag
			}
		}
	}
	ordered := make([]*hotTagAggregate, 0, len(values))
	for _, item := range values {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.usage != right.usage {
			return left.usage > right.usage
		}
		if left.interaction != right.interaction {
			return left.interaction > right.interaction
		}
		if !left.latest.Equal(right.latest) {
			return left.latest.After(right.latest)
		}
		return strings.ToLower(left.tag) < strings.ToLower(right.tag)
	})
	if limit < 1 || limit > 20 {
		limit = 12
	}
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	result := make([]HotTag, 0, len(ordered))
	for _, item := range ordered {
		result = append(result, HotTag{Tag: item.tag, UsageScore: item.usage, InteractionScore: item.interaction})
	}
	return result
}

func shanghaiTagDayStart(value time.Time) time.Time {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

type SearchHandler struct {
	service *Service
	users   *user.Service
	metrics searchmetrics.Metrics
	now     func() time.Time
}

func NewSearchHandler(service *Service, users *user.Service, metrics searchmetrics.Metrics) *SearchHandler {
	return &SearchHandler{service: service, users: users, metrics: metrics, now: time.Now}
}

func (h *SearchHandler) Register(group *gin.RouterGroup) {
	auth := group.Group("", h.users.AuthRequired())
	auth.POST("/searches", h.search)
	auth.GET("/search-history", h.history)
	auth.DELETE("/search-history", h.clearHistory)
	auth.GET("/hot-searches", h.hotSearches)
	auth.GET("/hot-tags", h.hotTags)
}

func (h *SearchHandler) search(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyForum) {
		response.Fail(c, http.StatusForbidden, "社区功能暂未开放")
		return
	}
	var request struct {
		Keyword  string        `json:"keyword"`
		Page     int           `json:"page"`
		PageSize int           `json:"pageSize"`
		Filters  SearchFilters `json:"filters"`
	}
	if c.ShouldBindJSON(&request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid community search")
		return
	}
	if err := validateSearchFilters(request.Filters); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	keyword, err := h.metrics.Confirm(c, user.CurrentUserID(c), request.Keyword)
	if errors.Is(err, searchmetrics.ErrInvalidKeyword) {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "record community search failed")
		return
	}
	value, err := h.service.SearchPosts(c, user.CurrentUserID(c), keyword, request.Page, request.PageSize, request.Filters)
	write(c, struct {
		Page[PostView]
		FilterVersion int `json:"filterVersion"`
	}{value, 1}, err)
}

func (h *SearchHandler) history(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyForum) {
		response.Fail(c, http.StatusForbidden, "社区功能暂未开放")
		return
	}
	value, err := h.metrics.History(c, user.CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list community search history failed")
		return
	}
	response.OK(c, value)
}

func (h *SearchHandler) clearHistory(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyForum) {
		response.Fail(c, http.StatusForbidden, "社区功能暂未开放")
		return
	}
	if err := h.metrics.ClearHistory(c, user.CurrentUserID(c)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "clear community search history failed")
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

func (h *SearchHandler) hotSearches(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyForum) {
		response.Fail(c, http.StatusForbidden, "社区功能暂未开放")
		return
	}
	value, err := h.metrics.Hot(c)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list community hot searches failed")
		return
	}
	response.OK(c, value)
}

func (h *SearchHandler) hotTags(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyForum) {
		response.Fail(c, http.StatusForbidden, "社区功能暂未开放")
		return
	}
	value, err := h.service.HotTags(c, h.now())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list community hot tags failed")
		return
	}
	response.OK(c, value)
}
