package featureboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/adminauth"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

const maxRequestBodyBytes int64 = 16 << 10

type userService interface {
	ListUser(context.Context, uint64, string, int, int) (ListView, error)
	Submit(context.Context, uint64, string) (SuggestionView, error)
	SetSupport(context.Context, uint64, string, bool) (SuggestionView, error)
	Notes(context.Context, int, int) (NoteListView, error)
	ListComments(context.Context, string, int, int) (CommentListView, error)
	SubmitComment(context.Context, uint64, string, string, bool) (CommentView, error)
}

type adminService interface {
	ListAdmin(context.Context, string, int, int) (AdminListView, error)
	SetStatus(context.Context, string, string) (AdminSuggestionView, error)
	ListNotesAdmin(context.Context) ([]NoteView, error)
	CreateNote(context.Context, string) (NoteView, error)
	DeleteNote(context.Context, string) error
}

type currentUserIDResolver func(*gin.Context) uint64

type Handler struct {
	service       userService
	adminService  adminService
	currentUserID currentUserIDResolver
}

func NewHandler(service userService, resolvers ...currentUserIDResolver) *Handler {
	resolver := func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	}
	admin, _ := service.(adminService)
	return &Handler{service: service, adminService: admin, currentUserID: resolver}
}

// RegisterUserReads expects rg to enforce user authentication only.
func (h *Handler) RegisterUserReads(rg *gin.RouterGroup) {
	rg.GET("/feature-suggestions", h.list)
	rg.GET("/feature-suggestions/:suggestionId/comments", h.comments)
	rg.GET("/author-notes", h.notes)
}

// RegisterUserWrites expects rg to enforce user authentication and the
// unmuted policy.
func (h *Handler) RegisterUserWrites(rg *gin.RouterGroup) {
	rg.POST("/feature-suggestions", h.submit)
	rg.POST("/feature-suggestions/:suggestionId/support", h.support)
	rg.POST("/feature-suggestions/:suggestionId/comments", h.submitComment)
}

// RegisterAdmin expects rg to enforce the admin session and admin rights.
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/suggestions", h.adminList)
	rg.GET("/suggestions/:suggestionId/comments", h.adminComments)
	rg.POST("/suggestions/:suggestionId/comments", h.adminReply)
	rg.POST("/suggestions/:suggestionId/status", h.adminSetStatus)
	rg.GET("/notes", h.adminNotes)
	rg.POST("/notes", h.adminCreateNote)
	rg.DELETE("/notes/:noteId", h.adminDeleteNote)
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, "invalid feature board request")
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, "feature suggestion not found")
	case errors.Is(err, ErrClosed):
		response.Fail(c, http.StatusConflict, "feature suggestion is closed")
	default:
		response.Fail(c, http.StatusInternalServerError, "feature board operation failed")
	}
}

func pageQuery(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	return page, pageSize
}

type contentRequest struct {
	Content string `json:"content"`
}

type supportRequest struct {
	Supported *bool `json:"supported"`
}

type statusRequest struct {
	Status string `json:"status"`
}

func decodeBody(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid feature board payload")
		return false
	}
	return true
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize := pageQuery(c)
	view, err := h.service.ListUser(c.Request.Context(), h.currentUserID(c), c.Query("sort"), page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) submit(c *gin.Context) {
	var payload contentRequest
	if !decodeBody(c, &payload) {
		return
	}
	view, err := h.service.Submit(c.Request.Context(), h.currentUserID(c), payload.Content)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) support(c *gin.Context) {
	var payload supportRequest
	if !decodeBody(c, &payload) {
		return
	}
	if payload.Supported == nil {
		response.Fail(c, http.StatusBadRequest, "supported flag is required")
		return
	}
	view, err := h.service.SetSupport(c.Request.Context(), h.currentUserID(c), c.Param("suggestionId"), *payload.Supported)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, gin.H{"supported": view.Supported, "supportCount": view.SupportCount})
}

func (h *Handler) notes(c *gin.Context) {
	page, pageSize := pageQuery(c)
	view, err := h.service.Notes(c.Request.Context(), page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) adminList(c *gin.Context) {
	page, pageSize := pageQuery(c)
	view, err := h.adminService.ListAdmin(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) adminSetStatus(c *gin.Context) {
	var payload statusRequest
	if !decodeBody(c, &payload) {
		return
	}
	view, err := h.adminService.SetStatus(c.Request.Context(), c.Param("suggestionId"), payload.Status)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) adminNotes(c *gin.Context) {
	views, err := h.adminService.ListNotesAdmin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, views)
}

func (h *Handler) adminCreateNote(c *gin.Context) {
	var payload contentRequest
	if !decodeBody(c, &payload) {
		return
	}
	view, err := h.adminService.CreateNote(c.Request.Context(), payload.Content)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) adminDeleteNote(c *gin.Context) {
	if err := h.adminService.DeleteNote(c.Request.Context(), c.Param("noteId")); err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *Handler) comments(c *gin.Context) {
	page, pageSize := pageQuery(c)
	view, err := h.service.ListComments(c.Request.Context(), c.Param("suggestionId"), page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) submitComment(c *gin.Context) {
	h.writeComment(c, false)
}

func (h *Handler) adminComments(c *gin.Context) {
	h.comments(c)
}

func (h *Handler) adminReply(c *gin.Context) {
	h.writeComment(c, true)
}

func (h *Handler) writeComment(c *gin.Context, maintainer bool) {
	var payload contentRequest
	if !decodeBody(c, &payload) {
		return
	}
	view, err := h.service.SubmitComment(c.Request.Context(), adminauth.CurrentAccountID(c), c.Param("suggestionId"), payload.Content, maintainer)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, view)
}
