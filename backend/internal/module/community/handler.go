package community

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service *Service
	users   *user.Service
	safety  review.TextSafetyChecker
}

func NewHandler(service *Service, users *user.Service, safety review.TextSafetyChecker) *Handler {
	return &Handler{service: service, users: users, safety: safety}
}
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/posts", h.feed)
	rg.GET("/posts/:id", h.post)
	rg.GET("/posts/:id/comments", h.comments)
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/following-feed", h.followingFeed)
	auth.GET("/notifications", h.notifications)
	auth.GET("/posts/:id/viewer-state", h.viewerState)
	auth.POST("/notifications/:id/read", h.readNotification)
	auth.POST("/posts", user.UnmutedRequired(), h.create)
	auth.POST("/posts/:id/comments", user.UnmutedRequired(), h.addComment)
	auth.POST("/posts/:id/like", h.like)
	auth.POST("/posts/:id/favorite", h.favorite)
	auth.POST("/users/:id/follow", h.follow)
	auth.PUT("/posts/:id/like", h.setLike(true))
	auth.DELETE("/posts/:id/like", h.setLike(false))
	auth.PUT("/posts/:id/favorite", h.setFavorite(true))
	auth.DELETE("/posts/:id/favorite", h.setFavorite(false))
	auth.PUT("/users/:id/follow", h.setFollow(true))
	auth.DELETE("/users/:id/follow", h.setFollow(false))
}
func (h *Handler) viewerState(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.ViewerState(c, id, user.CurrentUserID(c))
	write(c, value, err)
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
		Title           string   `json:"title"`
		Content         string   `json:"content"`
		Topic           string   `json:"topic"`
		ImageAssetIDs   []string `json:"imageAssetIds"`
		ClientRequestID string   `json:"clientRequestId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid post")
		return
	}
	clientRequestID, keyed, err := resolveCommunityIdempotencyKey(c.GetHeader("Idempotency-Key"), req.ClientRequestID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	currentUser := user.CurrentUser(c)
	risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Title+"\n"+req.Content)
	input := CreatePostInput{UserID: currentUser.ID, Title: req.Title, Content: req.Content, Topic: req.Topic, ImageAssetIDs: req.ImageAssetIDs}
	if keyed {
		result, err := h.service.CreatePostKeyed(c, input, clientRequestID, risk)
		writeKeyedCreate(c, result.Value, result.Replayed, err)
		return
	}
	result, err := h.service.CreatePost(c, input, risk)
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
		Content         string `json:"content"`
		ClientRequestID string `json:"clientRequestId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid comment")
		return
	}
	clientRequestID, keyed, err := resolveCommunityIdempotencyKey(c.GetHeader("Idempotency-Key"), req.ClientRequestID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	currentUser := user.CurrentUser(c)
	risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Content)
	if keyed {
		result, err := h.service.AddCommentKeyed(c, id, currentUser.ID, req.Content, clientRequestID, risk)
		writeKeyedCreate(c, result.Value, result.Replayed, err)
		return
	}
	result, err := h.service.AddComment(c, id, currentUser.ID, req.Content, risk)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, result)
}

func resolveCommunityIdempotencyKey(headerValue, bodyValue string) (string, bool, error) {
	headerValue = strings.TrimSpace(headerValue)
	bodyValue = strings.TrimSpace(bodyValue)
	if headerValue != "" && bodyValue != "" && headerValue != bodyValue {
		return "", false, errors.New("idempotency key mismatch")
	}
	value := headerValue
	if value == "" {
		value = bodyValue
	}
	if value == "" {
		return "", false, nil
	}
	if !validWriteContractULID(value) {
		return "", false, errors.New("invalid idempotency key")
	}
	return value, true, nil
}

func writeKeyedCreate(c *gin.Context, data any, replayed bool, err error) {
	switch {
	case errors.Is(err, ErrIdempotencyConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrWriteContractInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrWriteContractUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Fail(c, http.StatusGatewayTimeout, err.Error())
	case err != nil:
		response.Fail(c, http.StatusInternalServerError, err.Error())
	case replayed:
		c.Header("Idempotency-Replayed", "true")
		response.OK(c, data)
	default:
		response.Created(c, data)
	}
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
func (h *Handler) setLike(desired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := paramID(c)
		if !ok {
			return
		}
		liked, err := h.service.SetLike(c, id, user.CurrentUserID(c), desired)
		writeTargetState(c, gin.H{"liked": liked}, err)
	}
}
func (h *Handler) setFavorite(desired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := paramID(c)
		if !ok {
			return
		}
		favorited, err := h.service.SetFavorite(c, id, user.CurrentUserID(c), desired)
		writeTargetState(c, gin.H{"favorited": favorited}, err)
	}
}
func (h *Handler) setFollow(desired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := paramID(c)
		if !ok {
			return
		}
		followed, err := h.service.SetFollow(c, user.CurrentUserID(c), id, desired)
		writeTargetState(c, gin.H{"following": followed}, err)
	}
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

func writeTargetState(c *gin.Context, data any, err error) {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUserNotFound) {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, ErrCannotFollowSelf) {
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, data)
}
