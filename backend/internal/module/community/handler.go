package community

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service *Service
	users   *user.Service
	safety  *review.SafetyClient
}

func NewHandler(service *Service, users *user.Service, safety *review.SafetyClient) *Handler {
	return &Handler{service: service, users: users, safety: safety}
}
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/posts", h.feed)
	rg.GET("/posts/:id", h.post)
	rg.GET("/posts/:id/comments", h.comments)
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/following-feed", h.followingFeed)
	auth.GET("/notifications", h.notifications)
	auth.POST("/notifications/:id/read", h.readNotification)
	auth.POST("/posts", user.UnmutedRequired(), h.create)
	auth.POST("/posts/:id/comments", user.UnmutedRequired(), h.addComment)
	auth.POST("/posts/:id/like", h.like)
	auth.POST("/posts/:id/favorite", h.favorite)
	auth.POST("/users/:id/follow", h.follow)
}
func (h *Handler) feed(c *gin.Context) {
	page, size := pagination(c)
	result, err := h.service.Feed(c, FeedQuery{Topic: c.Query("topic"), Keyword: c.Query("keyword"), Sort: c.Query("sort"), Page: page, PageSize: size})
	write(c, result, err)
}
func (h *Handler) followingFeed(c *gin.Context) {
	page, size := pagination(c)
	result, err := h.service.FollowingFeed(c, user.CurrentUserID(c), FeedQuery{Page: page, PageSize: size})
	write(c, result, err)
}
func (h *Handler) post(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	result, err := h.service.Post(c, id)
	write(c, result, err)
}
func (h *Handler) create(c *gin.Context) {
	var req struct {
		Title         string   `json:"title"`
		Content       string   `json:"content"`
		Topic         string   `json:"topic"`
		ImageAssetIDs []string `json:"imageAssetIds"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid post")
		return
	}
	risk := h.safety.CheckText(c, "", req.Title+"\n"+req.Content)
	result, err := h.service.CreatePost(c, CreatePostInput{UserID: user.CurrentUserID(c), Title: req.Title, Content: req.Content, Topic: req.Topic, ImageAssetIDs: req.ImageAssetIDs}, risk)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, result)
}
func (h *Handler) comments(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	page, size := pagination(c)
	result, err := h.service.Comments(c, id, page, size)
	write(c, result, err)
}
func (h *Handler) addComment(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid comment")
		return
	}
	risk := h.safety.CheckText(c, "", req.Content)
	result, err := h.service.AddComment(c, id, user.CurrentUserID(c), req.Content, risk)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, result)
}
func (h *Handler) like(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	liked, err := h.service.ToggleLike(c, id, user.CurrentUserID(c))
	write(c, gin.H{"liked": liked}, err)
}
func (h *Handler) favorite(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	favorited, err := h.service.ToggleFavorite(c, id, user.CurrentUserID(c))
	write(c, gin.H{"favorited": favorited}, err)
}
func (h *Handler) follow(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	followed, err := h.service.ToggleFollow(c, user.CurrentUserID(c), id)
	write(c, gin.H{"followed": followed}, err)
}
func (h *Handler) notifications(c *gin.Context) {
	page, size := pagination(c)
	value, err := h.service.Notifications(c, user.CurrentUserID(c), page, size)
	write(c, value, err)
}
func (h *Handler) readNotification(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	err := h.service.MarkNotificationRead(c, user.CurrentUserID(c), id)
	write(c, gin.H{"read": err == nil}, err)
}
func pagination(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return p, s
}
func paramID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		response.Fail(c, 400, "invalid id")
		return 0, false
	}
	return uint(v), true
}
func write(c *gin.Context, data any, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, data)
}
