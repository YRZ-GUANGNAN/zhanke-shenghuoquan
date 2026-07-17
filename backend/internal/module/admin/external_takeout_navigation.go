package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const maxNavigationRequestBytes int64 = 32 << 10

type externalTakeoutMerchantRequest struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	IconText string `json:"iconText"`
	AppID    string `json:"appId"`
	Path     string `json:"path"`
	Color    string `json:"color"`
	Sort     int    `json:"sort"`
}

type externalTakeoutOperatingStatusRequest struct {
	OperatingStatus *navigation.OperatingStatus `json:"operatingStatus"`
}

type externalTakeoutVisibilityRequest struct {
	Enabled *bool `json:"enabled"`
}

func (h *Handler) externalTakeoutDirectory(c *gin.Context) {
	value, err := h.navigation.ExternalTakeoutDirectory(c)
	writeNavigation(c, http.StatusOK, value, err)
}

func (h *Handler) createExternalTakeoutMerchant(c *gin.Context) {
	var request externalTakeoutMerchantRequest
	if decodeStrictNavigationJSON(c, &request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid external takeout merchant request")
		return
	}
	value, err := h.navigation.CreateExternalTakeoutMerchant(c, request.input())
	writeNavigation(c, http.StatusCreated, value, err)
}

func (h *Handler) updateExternalTakeoutMerchant(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var request externalTakeoutMerchantRequest
	if decodeStrictNavigationJSON(c, &request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid external takeout merchant request")
		return
	}
	value, err := h.navigation.UpdateExternalTakeoutMerchant(c, id, request.input())
	writeNavigation(c, http.StatusOK, value, err)
}

func (h *Handler) deleteExternalTakeoutMerchant(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	err := h.navigation.DeleteExternalTakeoutMerchant(c, id)
	writeNavigation(c, http.StatusOK, gin.H{"deleted": err == nil}, err)
}

func (h *Handler) setExternalTakeoutOperatingStatus(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var request externalTakeoutOperatingStatusRequest
	if decodeStrictNavigationJSON(c, &request) != nil || request.OperatingStatus == nil {
		response.Fail(c, http.StatusBadRequest, "invalid operating status request")
		return
	}
	err := h.navigation.SetExternalTakeoutOperatingStatus(c, id, *request.OperatingStatus)
	writeNavigation(c, http.StatusOK, gin.H{"updated": err == nil}, err)
}

func (h *Handler) setExternalTakeoutVisibility(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var request externalTakeoutVisibilityRequest
	if decodeStrictNavigationJSON(c, &request) != nil || request.Enabled == nil {
		response.Fail(c, http.StatusBadRequest, "invalid visibility request")
		return
	}
	err := h.navigation.SetExternalTakeoutVisibility(c, id, *request.Enabled)
	writeNavigation(c, http.StatusOK, gin.H{"updated": err == nil}, err)
}

func (h *Handler) setExternalTakeoutCategoryVisibility(c *gin.Context) {
	var request externalTakeoutVisibilityRequest
	if decodeStrictNavigationJSON(c, &request) != nil || request.Enabled == nil {
		response.Fail(c, http.StatusBadRequest, "invalid visibility request")
		return
	}
	err := h.navigation.SetExternalTakeoutCategoryVisibility(c, *request.Enabled)
	writeNavigation(c, http.StatusOK, gin.H{"updated": err == nil}, err)
}

func (request externalTakeoutMerchantRequest) input() navigation.ExternalTakeoutMerchantInput {
	return navigation.ExternalTakeoutMerchantInput{
		Title: request.Title, Subtitle: request.Subtitle, IconText: request.IconText,
		AppID: request.AppID, Path: request.Path, Color: request.Color, Sort: request.Sort,
	}
}

func decodeStrictNavigationJSON(c *gin.Context, destination any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxNavigationRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return errors.New("JSON object required")
	}
	strictDecoder := json.NewDecoder(bytes.NewReader(trimmed))
	strictDecoder.DisallowUnknownFields()
	if err := strictDecoder.Decode(destination); err != nil {
		return err
	}
	if err := strictDecoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeNavigation(c *gin.Context, successStatus int, data any, err error) {
	if err == nil {
		if successStatus == http.StatusCreated {
			response.Created(c, data)
			return
		}
		response.OK(c, data)
		return
	}

	switch {
	case errors.Is(err, navigation.ErrInvalidExternalTakeoutInput):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, navigation.ErrInvalidNavigationInput):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, navigation.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "navigation entry not found")
	case errors.Is(err, navigation.ErrDuplicateTarget):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, navigation.ErrSystemManaged):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "navigation operation failed")
	}
}
