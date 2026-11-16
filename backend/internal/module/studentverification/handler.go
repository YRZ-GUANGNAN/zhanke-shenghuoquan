package studentverification

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

type userVerificationService interface {
	GetMine(context.Context, uint64) (VerificationView, error)
	Submit(context.Context, uint64, SubmitInput) (VerificationView, error)
	RequestCancel(context.Context, uint64, uint64) (VerificationView, error)
}

type adminVerificationService interface {
	ListAdmin(context.Context, AdminListFilter) (AdminListView, error)
	GetAdmin(context.Context, string) (VerificationDetail, error)
	Approve(context.Context, uint64, string, uint64) (VerificationView, error)
	Reject(context.Context, uint64, string, uint64, string) (VerificationView, error)
	ApproveCancel(context.Context, uint64, string, uint64, string) (VerificationView, error)
}

type currentUserIDResolver func(*gin.Context) uint64

type Handler struct {
	service       userVerificationService
	adminService  adminVerificationService
	currentUserID currentUserIDResolver
}

func NewHandler(service userVerificationService, resolvers ...currentUserIDResolver) *Handler {
	resolver := func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	}
	adminService, _ := service.(adminVerificationService)
	return &Handler{service: service, adminService: adminService, currentUserID: resolver}
}

// RegisterUser expects rg to already enforce user.AuthRequired and
// user.UnmutedRequired. Keeping those policies at composition-root level lets
// the same Handler later expose separately protected Admin routes.
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	rg.GET("/student-verification", h.getMine)
	rg.PUT("/student-verification", h.submit)
	rg.POST("/student-verification/cancel-request", h.requestCancel)
}

// RegisterAdmin expects rg to already enforce the admin session and
// user.AdminRequired policies.
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	if h == nil || h.adminService == nil {
		panic("student verification admin service is required")
	}
	rg.GET("", h.listAdmin)
	rg.GET("/:applicationId", h.getAdmin)
	rg.POST("/:applicationId/approve", h.approve)
	rg.POST("/:applicationId/reject", h.reject)
	rg.POST("/:applicationId/approve-cancel", h.approveCancel)
}

type submitRequest struct {
	RealName           string   `json:"realName"`
	StudentNo          string   `json:"studentNo"`
	College            string   `json:"college"`
	AssetIDs           []string `json:"assetIds"`
	ExpectedRowVersion uint64   `json:"expectedRowVersion"`
}

type cancelRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

type rejectRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	Reason             string `json:"reason"`
}

type approveRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func (h *Handler) getMine(c *gin.Context) {
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "student verification request failed")
		return
	}
	value, err := h.service.GetMine(c, h.currentUserID(c))
	if err != nil {
		writeUserVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) submit(c *gin.Context) {
	request, err := decodeSubmitRequest(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification request")
		return
	}
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "student verification request failed")
		return
	}
	value, err := h.service.Submit(c, h.currentUserID(c), SubmitInput{
		RealName:           request.RealName,
		StudentNo:          request.StudentNo,
		College:            request.College,
		AssetPublicIDs:     request.AssetIDs,
		ExpectedRowVersion: request.ExpectedRowVersion,
	})
	if err != nil {
		writeUserVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) requestCancel(c *gin.Context) {
	var request cancelRequest
	if err := decodeStrictRequest(c, map[string]struct{}{"expectedRowVersion": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification request")
		return
	}
	if h == nil || h.service == nil || h.currentUserID == nil {
		response.Fail(c, http.StatusInternalServerError, "student verification request failed")
		return
	}
	value, err := h.service.RequestCancel(c, h.currentUserID(c), request.ExpectedRowVersion)
	if err != nil {
		writeUserVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) listAdmin(c *gin.Context) {
	filter, err := parseAdminListFilter(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification filter")
		return
	}
	value, err := h.adminService.ListAdmin(c, filter)
	if err != nil {
		writeAdminVerificationError(c, err)
		return
	}
	if value.Items == nil {
		value.Items = make([]VerificationView, 0)
	}
	response.OK(c, value)
}

func (h *Handler) getAdmin(c *gin.Context) {
	value, err := h.adminService.GetAdmin(c, c.Param("applicationId"))
	if err != nil {
		writeAdminVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) approve(c *gin.Context) {
	var request approveRequest
	if err := decodeStrictRequest(c, map[string]struct{}{"expectedRowVersion": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification decision")
		return
	}
	value, err := h.adminService.Approve(c, h.currentUserID(c), c.Param("applicationId"), request.ExpectedRowVersion)
	if err != nil {
		writeAdminVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) reject(c *gin.Context) {
	var request rejectRequest
	if err := decodeStrictRequest(c, map[string]struct{}{"expectedRowVersion": {}, "reason": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification decision")
		return
	}
	value, err := h.adminService.Reject(c, h.currentUserID(c), c.Param("applicationId"), request.ExpectedRowVersion, request.Reason)
	if err != nil {
		writeAdminVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) approveCancel(c *gin.Context) {
	var request rejectRequest
	if err := decodeStrictRequest(c, map[string]struct{}{"expectedRowVersion": {}, "reason": {}}, &request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid student verification decision")
		return
	}
	value, err := h.adminService.ApproveCancel(c, h.currentUserID(c), c.Param("applicationId"), request.ExpectedRowVersion, request.Reason)
	if err != nil {
		writeAdminVerificationError(c, err)
		return
	}
	response.OK(c, value)
}

func parseAdminListFilter(c *gin.Context) (AdminListFilter, error) {
	query := c.Request.URL.Query()
	allowed := map[string]struct{}{"status": {}, "page": {}, "pageSize": {}}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return AdminListFilter{}, ErrInvalidInput
		}
	}
	filter := AdminListFilter{Status: Status(query.Get("status")), Page: 1, PageSize: 20}
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
	if (filter.Status != "" && !filter.Status.Valid()) ||
		filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return AdminListFilter{}, ErrInvalidInput
	}
	return filter, nil
}

func decodeSubmitRequest(c *gin.Context) (submitRequest, error) {
	allowed := map[string]struct{}{
		"realName": {}, "studentNo": {}, "college": {}, "assetIds": {}, "expectedRowVersion": {},
	}
	var request submitRequest
	if err := decodeStrictRequest(c, allowed, &request); err != nil {
		return submitRequest{}, err
	}
	return request, nil
}

func decodeStrictRequest(c *gin.Context, allowed map[string]struct{}, target any) error {
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

func writeUserVerificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, "invalid student verification request")
	case errors.Is(err, ErrApplicationNotFound):
		response.Fail(c, http.StatusNotFound, "student verification not found")
	case errors.Is(err, ErrApplicationPending), errors.Is(err, ErrApplicationApproved),
		errors.Is(err, ErrVersionConflict), errors.Is(err, ErrCancelNotAllowed),
		errors.Is(err, ErrProofMediaConflict):
		response.Fail(c, http.StatusConflict, "student verification conflict")
	default:
		response.Fail(c, http.StatusInternalServerError, "student verification request failed")
	}
}

func writeAdminVerificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, "invalid student verification request")
	case errors.Is(err, ErrSelfReview):
		response.Fail(c, http.StatusForbidden, "student verification self review forbidden")
	case errors.Is(err, ErrApplicationNotFound):
		response.Fail(c, http.StatusNotFound, "student verification not found")
	case errors.Is(err, ErrDecisionConflict), errors.Is(err, ErrVersionConflict),
		errors.Is(err, ErrProofMediaConflict), errors.Is(err, ErrProofMediaUnavailable):
		response.Fail(c, http.StatusConflict, "student verification conflict")
	default:
		response.Fail(c, http.StatusInternalServerError, "student verification request failed")
	}
}
