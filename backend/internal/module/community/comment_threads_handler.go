package community

import (
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) commentThreads(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	page, size := pagination(c)
	value, err := h.service.CommentThreads(c, id, user.CurrentUserID(c), page, size)
	write(c, value, err)
}

func commentParamID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid comment id")
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) commentReplies(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	rootID, ok := commentParamID(c)
	if !ok {
		return
	}
	page, size := pagination(c)
	value, err := h.service.CommentReplies(c, id, rootID, user.CurrentUserID(c), page, size)
	write(c, value, err)
}

func (h *Handler) replyComment(c *gin.Context) {
	current := user.CurrentUser(c)
	if !featureflags.CanComment(current.Role) {
		response.Fail(c, http.StatusForbidden, "comments are currently disabled")
		return
	}
	id, ok := paramID(c)
	if !ok {
		return
	}
	targetID, ok := commentParamID(c)
	if !ok {
		return
	}
	var input struct {
		Content         string `json:"content"`
		ClientRequestID string `json:"clientRequestId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid comment")
		return
	}
	key, keyed, err := resolveCommunityIdempotencyKey(c.GetHeader("Idempotency-Key"), input.ClientRequestID)
	if err != nil || !keyed {
		response.Fail(c, http.StatusBadRequest, "valid idempotency key is required")
		return
	}
	risk := h.safety.CheckText(c.Request.Context(), current.OpenID, input.Content)
	value, err := h.service.ReplyCommentKeyed(c, id, targetID, current.ID, input.Content, key, risk)
	writeKeyedCreate(c, value.Value, value.Replayed, err)
}

func (h *Handler) setCommentLiked(desired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := paramID(c)
		if !ok {
			return
		}
		commentID, ok := commentParamID(c)
		if !ok {
			return
		}
		value, err := h.service.SetCommentLiked(c, id, commentID, user.CurrentUserID(c), desired)
		writeTargetState(c, value, err)
	}
}
