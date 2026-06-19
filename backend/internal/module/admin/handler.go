package admin

import (
	"errors"
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/community"
	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/trade"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service    *Service
	reviews    *review.Service
	users      *user.Service
	community  *community.Service
	trade      *trade.Service
	navigation *navigation.Service
}

func NewHandler(service *Service, reviews *review.Service, users *user.Service, communityService *community.Service, tradeService *trade.Service, navigationService *navigation.Service) *Handler {
	return &Handler{service: service, reviews: reviews, users: users, community: communityService, trade: tradeService, navigation: navigationService}
}
func (h *Handler) Register(api *gin.RouterGroup) {
	api.POST("/reports", h.users.AuthRequired(), user.UnmutedRequired(), h.submitReport)
	secured := api.Group("/admin", h.users.AuthRequired(), user.AdminRequired())
	secured.GET("/stats", h.stats)
	secured.GET("/reviews", h.reviewList)
	secured.POST("/reviews/:id/start", h.startReview)
	secured.POST("/reviews/:id/approve", h.approve)
	secured.POST("/reviews/:id/reject", h.reject)
	secured.GET("/reports", h.reports)
	secured.GET("/users", h.usersList)
	secured.PATCH("/users/:id/status", h.userStatus)
	secured.PATCH("/users/:id/role", user.SuperAdminRequired(), h.userRole)
	secured.GET("/posts", h.posts)
	secured.GET("/products", h.products)
	secured.GET("/navigation", h.navigationTree)
	secured.POST("/navigation/categories", h.createCategory)
	secured.PUT("/navigation/categories/:id", h.updateCategory)
	secured.DELETE("/navigation/categories/:id", h.deleteCategory)
	secured.POST("/navigation/items", h.createItem)
	secured.PUT("/navigation/items/:id", h.updateItem)
	secured.DELETE("/navigation/items/:id", h.deleteItem)
}
func (h *Handler) submitReport(c *gin.Context) {
	var req struct {
		TargetType string `json:"targetType"`
		TargetID   uint   `json:"targetId"`
		Reason     string `json:"reason"`
		Detail     string `json:"detail"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid report")
		return
	}
	value, err := h.service.SubmitReport(c, user.CurrentUserID(c), req.TargetType, req.TargetID, req.Reason, req.Detail)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, value)
}
func (h *Handler) stats(c *gin.Context) {
	ut, ud, err := h.users.Stats(c)
	if err != nil {
		response.Fail(c, 500, "user stats failed")
		return
	}
	pt, pd, err := h.community.Stats(c)
	if err != nil {
		response.Fail(c, 500, "community stats failed")
		return
	}
	tt, td, err := h.trade.Stats(c)
	if err != nil {
		response.Fail(c, 500, "trade stats failed")
		return
	}
	pending, err := h.reviews.PendingCount(c)
	if err != nil {
		response.Fail(c, 500, "review stats failed")
		return
	}
	response.OK(c, gin.H{"totals": gin.H{"users": ut, "posts": pt, "products": tt, "pendingReview": pending}, "today": gin.H{"users": ud, "posts": pd, "products": td}})
}
func (h *Handler) reviewList(c *gin.Context) {
	page, size := pagination(c)
	values, total, err := h.reviews.List(c, review.Status(c.Query("status")), page, size)
	if err != nil {
		response.Fail(c, 500, "list reviews failed")
		return
	}
	response.OK(c, gin.H{"items": values, "page": page, "pageSize": size, "total": total})
}
func (h *Handler) startReview(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.reviews.Start(c, id, user.CurrentUserID(c))
	write(c, value, err)
}
func (h *Handler) approve(c *gin.Context) { h.decide(c, review.StatusApproved) }
func (h *Handler) reject(c *gin.Context)  { h.decide(c, review.StatusRejected) }
func (h *Handler) decide(c *gin.Context, status review.Status) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	value, err := h.reviews.Decide(c, id, user.CurrentUserID(c), status, req.Reason)
	write(c, value, err)
}
func (h *Handler) reports(c *gin.Context) {
	page, size := pagination(c)
	values, total, err := h.service.Reports(c, page, size)
	if err != nil {
		response.Fail(c, 500, "list reports failed")
		return
	}
	response.OK(c, gin.H{"items": values, "page": page, "pageSize": size, "total": total})
}
func (h *Handler) usersList(c *gin.Context) {
	page, size := pagination(c)
	values, total, err := h.users.AdminList(c, page, size)
	if err != nil {
		response.Fail(c, 500, "list users failed")
		return
	}
	response.OK(c, gin.H{"items": values, "page": page, "pageSize": size, "total": total})
}
func (h *Handler) userStatus(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req struct {
		Status user.Status `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid status")
		return
	}
	err := h.users.AdminUpdateStatus(c, user.CurrentUserID(c), id, req.Status)
	write(c, gin.H{"updated": err == nil}, err)
}
func (h *Handler) userRole(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req struct {
		Role user.Role `json:"role"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid role")
		return
	}
	err := h.users.AdminUpdateRole(c, user.CurrentUserID(c), id, req.Role)
	write(c, gin.H{"updated": err == nil}, err)
}
func (h *Handler) posts(c *gin.Context) {
	page, size := pagination(c)
	value, err := h.community.AdminList(c, page, size)
	write(c, value, err)
}
func (h *Handler) products(c *gin.Context) {
	page, size := pagination(c)
	value, err := h.trade.AdminList(c, page, size)
	write(c, value, err)
}
func (h *Handler) navigationTree(c *gin.Context) {
	value, err := h.navigation.AdminTree(c)
	write(c, value, err)
}
func (h *Handler) createCategory(c *gin.Context) {
	var value navigation.Category
	if c.ShouldBindJSON(&value) != nil {
		response.Fail(c, 400, "invalid category")
		return
	}
	result, err := h.navigation.CreateCategory(c, value)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, result)
}
func (h *Handler) updateCategory(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var value navigation.Category
	if c.ShouldBindJSON(&value) != nil {
		response.Fail(c, 400, "invalid category")
		return
	}
	value.ID = id
	write(c, gin.H{"updated": true}, h.navigation.UpdateCategory(c, value))
}
func (h *Handler) deleteCategory(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	write(c, gin.H{"deleted": true}, h.navigation.DeleteCategory(c, id))
}
func (h *Handler) createItem(c *gin.Context) {
	var value navigation.Item
	if c.ShouldBindJSON(&value) != nil {
		response.Fail(c, 400, "invalid item")
		return
	}
	result, err := h.navigation.CreateItem(c, value)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, result)
}
func (h *Handler) updateItem(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var value navigation.Item
	if c.ShouldBindJSON(&value) != nil {
		response.Fail(c, 400, "invalid item")
		return
	}
	value.ID = id
	write(c, gin.H{"updated": true}, h.navigation.UpdateItem(c, value))
}
func (h *Handler) deleteItem(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	write(c, gin.H{"deleted": true}, h.navigation.DeleteItem(c, id))
}
func pagination(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return normalize(p, s)
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
	if errors.Is(err, user.ErrForbidden) || errors.Is(err, user.ErrCannotModifySelf) || errors.Is(err, user.ErrLastActiveSuperAdmin) {
		response.Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, data)
}
