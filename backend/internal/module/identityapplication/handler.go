package identityapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

const maxSubmitRequestBytes int64 = 32 << 10

type userApplicationService interface {
	ListMine(context.Context, uint64) ([]ApplicationView, error)
	GetMine(context.Context, uint64, Role) (ApplicationView, error)
	Submit(context.Context, uint64, Role, SubmitInput) (ApplicationView, error)
}

type adminApplicationService interface {
	ListAdmin(context.Context, AdminListFilter) (AdminListView, error)
	GetAdmin(context.Context, string) (ApplicationDetail, error)
	Approve(context.Context, uint64, string, uint64) (ApplicationView, error)
	Reject(context.Context, uint64, string, uint64, string) (ApplicationView, error)
}

type currentUserIDResolver func(*gin.Context) uint64

type Handler struct {
	service       userApplicationService
	adminService  adminApplicationService
	currentUserID currentUserIDResolver
}

func NewHandler(service userApplicationService, resolvers ...currentUserIDResolver) *Handler {
	resolver := func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	}
	adminService, _ := service.(adminApplicationService)
	return &Handler{service: service, adminService: adminService, currentUserID: resolver}
}

// RegisterUser expects rg to already enforce user.AuthRequired and
// user.UnmutedRequired. Keeping those policies at composition-root level lets
// the same Handler later expose separately protected Admin routes.
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	rg.GET("/identity-applications", h.listMine)
	rg.GET("/identity-applications/:role", h.getMine)
	rg.PUT("/identity-applications/:role", h.submit)
}

// RegisterAdmin expects rg to already enforce user.AuthRequired and
// user.AdminRequired. A handler constructed without the Admin service fails
// immediately instead of silently exposing routes that can never work.
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	if h == nil || h.adminService == nil {
		panic("identity application admin service is required")
	}
	rg.GET("", h.listAdmin)
	rg.GET("/:applicationId", h.getAdmin)
	rg.POST("/:applicationId/approve", h.approve)
	rg.POST("/:applicationId/reject", h.reject)
}

type submitRequest struct {
	RealName           string   `json:"realName"`
	Phone              string   `json:"phone"`
	MerchantName       string   `json:"merchantName"`
	MerchantAddress    string   `json:"merchantAddress"`
	AssetIDs           []string `json:"assetIds"`
	ExpectedRowVersion uint64   `json:"expectedRowVersion"`
}

type approveRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

type rejectRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	Reason             string `json:"reason"`
}

func (h *Handler) listAdmin(c *gin.Context) {
	filter, err := parseAdminListFilter(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid identity application filter")
		return
	}
	value, err := h.adminService.ListAdmin(c, filter)
	if err != nil {
		writeAdminApplicationError(c, err)
		return
	}
	if value.Items == nil {
		value.Items = make([]ApplicationView, 0)
	}
	response.OK(c, value)
}

func (h *Handler) getAdmin(c *gin.Context) {
	value, err := h.adminService.GetAdmin(c, c.Param("applicationId"))
	if err != nil {
		writeAdminApplicationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) approve(c *gin.Context) {
	var request approveRequest
	if err := decodeAdminRequest(c, map[string]struct{}{"expectedRowVersion": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid identity application decision")
		return
	}
	value, err := h.adminService.Approve(c, h.currentUserID(c), c.Param("applicationId"), request.ExpectedRowVersion)
	if err != nil {
		writeAdminApplicationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) reject(c *gin.Context) {
	var request rejectRequest
	if err := decodeAdminRequest(c, map[string]struct{}{"expectedRowVersion": {}, "reason": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid identity application decision")
		return
	}
	value, err := h.adminService.Reject(c, h.currentUserID(c), c.Param("applicationId"), request.ExpectedRowVersion, request.Reason)
	if err != nil {
		writeAdminApplicationError(c, err)
		return
	}
	response.OK(c, value)
}

func parseAdminListFilter(c *gin.Context) (AdminListFilter, error) {
	query := c.Request.URL.Query()
	allowed := map[string]struct{}{"role": {}, "status": {}, "page": {}, "pageSize": {}}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return AdminListFilter{}, ErrInvalidInput
		}
	}
	filter := AdminListFilter{Role: Role(query.Get("role")), Status: Status(query.Get("status")), Page: 1, PageSize: 20}
	if raw := query.Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return AdminListFilter{}, err
		}
		filter.Page = value
	}
	if raw := query.Get("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return AdminListFilter{}, err
		}
		filter.PageSize = value
	}
	if (filter.Role != "" && !filter.Role.Valid()) || (filter.Status != "" && !filter.Status.Valid()) ||
		filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return AdminListFilter{}, ErrInvalidInput
	}
	return filter, nil
}

func decodeAdminRequest(c *gin.Context, allowed map[string]struct{}, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSubmitRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrInvalidInput
		}
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return ErrInvalidInput
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil || fields == nil {
		return ErrInvalidInput
	}
	for key := range fields {
		if _, exists := allowed[key]; !exists {
			return ErrInvalidInput
		}
	}
	strict := json.NewDecoder(bytes.NewReader(trimmed))
	strict.DisallowUnknownFields()
	if err := strict.Decode(target); err != nil {
		return err
	}
	if err := strict.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrInvalidInput
	}
	return nil
}

func (h *Handler) listMine(c *gin.Context) {
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "identity application request failed")
		return
	}
	values, err := h.service.ListMine(c, h.currentUserID(c))
	if err != nil {
		writeUserApplicationError(c, err)
		return
	}
	if values == nil {
		values = make([]ApplicationView, 0)
	}
	response.OK(c, values)
}

func (h *Handler) getMine(c *gin.Context) {
	role, ok := parseUserApplicationRole(c)
	if !ok {
		return
	}
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "identity application request failed")
		return
	}
	value, err := h.service.GetMine(c, h.currentUserID(c), role)
	if err != nil {
		writeUserApplicationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) submit(c *gin.Context) {
	role, ok := parseUserApplicationRole(c)
	if !ok {
		return
	}
	request, err := decodeSubmitRequest(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid identity application request")
		return
	}
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "identity application request failed")
		return
	}
	value, err := h.service.Submit(c, h.currentUserID(c), role, SubmitInput{
		RealName:           request.RealName,
		Phone:              request.Phone,
		MerchantName:       request.MerchantName,
		MerchantAddress:    request.MerchantAddress,
		AssetPublicIDs:     request.AssetIDs,
		ExpectedRowVersion: request.ExpectedRowVersion,
	})
	if err != nil {
		writeUserApplicationError(c, err)
		return
	}
	response.OK(c, value)
}

func parseUserApplicationRole(c *gin.Context) (Role, bool) {
	role := Role(c.Param("role"))
	if !role.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid identity application role")
		return "", false
	}
	return role, true
}

func decodeSubmitRequest(c *gin.Context) (submitRequest, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSubmitRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return submitRequest{}, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return submitRequest{}, ErrInvalidInput
		}
		return submitRequest{}, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return submitRequest{}, ErrInvalidInput
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil || fields == nil {
		return submitRequest{}, ErrInvalidInput
	}
	allowed := map[string]struct{}{
		"realName": {}, "phone": {}, "merchantName": {}, "merchantAddress": {},
		"assetIds": {}, "expectedRowVersion": {},
	}
	for key := range fields {
		if _, exists := allowed[key]; !exists {
			return submitRequest{}, ErrInvalidInput
		}
	}
	strict := json.NewDecoder(bytes.NewReader(trimmed))
	strict.DisallowUnknownFields()
	var request submitRequest
	if err := strict.Decode(&request); err != nil {
		return submitRequest{}, err
	}
	if err := strict.Decode(&extra); !errors.Is(err, io.EOF) {
		return submitRequest{}, ErrInvalidInput
	}
	return request, nil
}

func writeUserApplicationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidRole):
		response.Fail(c, http.StatusBadRequest, "invalid identity application request")
	case errors.Is(err, ErrApplicationNotFound):
		response.Fail(c, http.StatusNotFound, "identity application not found")
	case errors.Is(err, ErrApplicationPending), errors.Is(err, ErrApplicationApproved),
		errors.Is(err, ErrVersionConflict), errors.Is(err, ErrProofMediaConflict):
		response.Fail(c, http.StatusConflict, "identity application conflict")
	default:
		response.Fail(c, http.StatusInternalServerError, "identity application request failed")
	}
}

func writeAdminApplicationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidRole):
		response.Fail(c, http.StatusBadRequest, "invalid identity application request")
	case errors.Is(err, ErrSelfReview):
		response.Fail(c, http.StatusForbidden, "identity application self review forbidden")
	case errors.Is(err, ErrApplicationNotFound):
		response.Fail(c, http.StatusNotFound, "identity application not found")
	case errors.Is(err, ErrDecisionConflict), errors.Is(err, ErrVersionConflict),
		errors.Is(err, ErrProofMediaConflict), errors.Is(err, ErrProofMediaUnavailable):
		response.Fail(c, http.StatusConflict, "identity application conflict")
	default:
		response.Fail(c, http.StatusInternalServerError, "identity application request failed")
	}
}
