package identityapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

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

type currentUserIDResolver func(*gin.Context) uint64

type Handler struct {
	service       userApplicationService
	currentUserID currentUserIDResolver
}

func NewHandler(service userApplicationService, resolvers ...currentUserIDResolver) *Handler {
	resolver := func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	}
	return &Handler{service: service, currentUserID: resolver}
}

// RegisterUser expects rg to already enforce user.AuthRequired and
// user.UnmutedRequired. Keeping those policies at composition-root level lets
// the same Handler later expose separately protected Admin routes.
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	rg.GET("/identity-applications", h.listMine)
	rg.GET("/identity-applications/:role", h.getMine)
	rg.PUT("/identity-applications/:role", h.submit)
}

type submitRequest struct {
	RealName           string   `json:"realName"`
	Phone              string   `json:"phone"`
	MerchantName       string   `json:"merchantName"`
	MerchantAddress    string   `json:"merchantAddress"`
	AssetIDs           []string `json:"assetIds"`
	ExpectedRowVersion uint64   `json:"expectedRowVersion"`
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
