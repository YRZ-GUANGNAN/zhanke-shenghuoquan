package takeoutbuyer

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	queries       *takeout.BuyerQueryService
	orders        *takeout.OrderService
	currentUserID func(*gin.Context) uint64
}

func NewHandler(queries *takeout.BuyerQueryService, orders *takeout.OrderService, currentUserID func(*gin.Context) uint64) *Handler {
	return &Handler{queries: queries, orders: orders, currentUserID: currentUserID}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/merchants", h.listMerchants)
	group.GET("/merchants/:merchantPublicId", h.merchant)
	group.GET("/merchants/:merchantPublicId/dishes", h.listDishes)
	group.POST("/orders", h.createOrder)
	group.GET("/orders", h.listOrders)
	group.GET("/orders/:orderPublicId", h.order)
}

func (h *Handler) listMerchants(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ActiveMerchants(c.Request.Context(), page, size)
	h.write(c, Page[MerchantView]{Items: mapValues(values, merchantView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) merchant(c *gin.Context) {
	value, err := h.queries.ActiveMerchant(c.Request.Context(), c.Param("merchantPublicId"))
	h.write(c, merchantView(value), err)
}

func (h *Handler) listDishes(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.OnShelfDishes(c.Request.Context(), c.Param("merchantPublicId"), page, size)
	h.write(c, Page[DishView]{Items: mapValues(values, dishView), Page: page, PageSize: size, Total: total}, err)
}

type createOrderRequest struct {
	MerchantPublicID string             `json:"merchantPublicId"`
	Items            []createOrderItem  `json:"items"`
	Address          createOrderAddress `json:"address"`
	Remark           string             `json:"remark"`
}

type createOrderItem struct {
	DishPublicID string `json:"dishPublicId"`
	Quantity     uint32 `json:"quantity"`
}

type createOrderAddress struct {
	ContactName     string `json:"contactName"`
	ContactPhone    string `json:"contactPhone"`
	AddressBuilding string `json:"addressBuilding"`
	AddressRoom     string `json:"addressRoom"`
}

func (h *Handler) createOrder(c *gin.Context) {
	var request createOrderRequest
	if c.ShouldBindJSON(&request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	merchantPublicID := strings.TrimSpace(request.MerchantPublicID)
	if _, err := h.queries.ActiveMerchant(c.Request.Context(), merchantPublicID); err != nil {
		h.write(c, nil, err)
		return
	}
	items := make([]takeout.OrderItemRequest, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, takeout.OrderItemRequest{DishPublicID: strings.TrimSpace(item.DishPublicID), Quantity: item.Quantity})
	}
	value, err := h.orders.CreateOrder(c.Request.Context(), takeout.CreateOrderInput{
		BuyerUserID: h.currentUserID(c), MerchantPublicID: merchantPublicID, Items: items,
		Address: takeout.OrderAddress{
			ContactName: strings.TrimSpace(request.Address.ContactName), ContactPhone: strings.TrimSpace(request.Address.ContactPhone),
			AddressBuilding: strings.TrimSpace(request.Address.AddressBuilding), AddressRoom: strings.TrimSpace(request.Address.AddressRoom),
		},
		Remark: strings.TrimSpace(request.Remark),
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	detail, err := h.orderDetail(c, value)
	h.writeCreated(c, detail, err)
}

func (h *Handler) listOrders(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.OrdersByBuyer(c.Request.Context(), h.currentUserID(c), takeout.OrderStatus(c.Query("status")), page, size)
	h.write(c, Page[OrderView]{Items: mapValues(values, orderView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) order(c *gin.Context) {
	value, err := h.orders.FindOrder(c.Request.Context(), c.Param("orderPublicId"))
	if err == nil && value.BuyerUserID != h.currentUserID(c) {
		err = takeout.ErrNotFound
	}
	if err != nil {
		h.write(c, nil, err)
		return
	}
	detail, err := h.orderDetail(c, value)
	h.write(c, detail, err)
}

func (h *Handler) orderDetail(c *gin.Context, value takeout.Order) (OrderDetailView, error) {
	items, err := h.orders.ListOrderItems(c.Request.Context(), value.OrderPublicID)
	return OrderDetailView{Order: orderView(value), Items: mapValues(items, orderItemView)}, err
}

func pagination(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || size < 1 || size > 100 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	return page, size, true
}

func (h *Handler) writeCreated(c *gin.Context, value any, err error) {
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, value)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, takeout.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "takeout record not found")
	case errors.Is(err, takeout.ErrDishUnavailable), errors.Is(err, takeout.ErrInvalidTransition):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, takeout.ErrInvalidRecord), errors.Is(err, takeout.ErrInvalidULID), errors.Is(err, takeout.ErrAmountOverflow):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "takeout buyer operation failed")
	}
}
