package deliveryrider

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
)

type riderAvailabilityView struct {
	Enabled     bool                    `json:"enabled"`
	RiderUserID uint64                  `json:"riderUserId"`
	State       delivery.RiderDutyState `json:"state"`
	Scope       string                  `json:"scope"`
	InFlight    uint32                  `json:"inFlight"`
	MaxInFlight uint32                  `json:"maxInFlight"`
	RowVersion  uint64                  `json:"rowVersion"`
	UpdatedAt   *time.Time              `json:"updatedAt,omitempty"`
}

type riderBroadcastConfigView struct {
	Enabled             bool       `json:"enabled"`
	MaxInFlightPerRider uint32     `json:"maxInFlightPerRider,omitempty"`
	RowVersion          uint64     `json:"rowVersion,omitempty"`
	UpdatedAt           *time.Time `json:"updatedAt,omitempty"`
}

type availabilityVersionRequest struct {
	ExpectedRowVersion *uint64 `json:"expectedRowVersion" binding:"required"`
}

type riderBroadcastConfigRequest struct {
	ExpectedRowVersion  *uint64 `json:"expectedRowVersion" binding:"required"`
	MaxInFlightPerRider *uint32 `json:"maxInFlightPerRider" binding:"required"`
}

func availabilityView(value delivery.RiderAvailabilitySnapshot) riderAvailabilityView {
	var updatedAt *time.Time
	if !value.UpdatedAt.IsZero() {
		copy := value.UpdatedAt
		updatedAt = &copy
	}
	return riderAvailabilityView{Enabled: true, RiderUserID: value.RiderUserID, State: value.State,
		Scope: value.Scope, InFlight: value.InFlight, MaxInFlight: value.MaxInFlight,
		RowVersion: value.RowVersion, UpdatedAt: updatedAt}
}

func broadcastConfigView(value delivery.RiderBroadcastConfig) riderBroadcastConfigView {
	updatedAt := value.UpdatedAt
	return riderBroadcastConfigView{Enabled: true, MaxInFlightPerRider: value.MaxInFlightPerRider,
		RowVersion: value.RowVersion, UpdatedAt: &updatedAt}
}

func (h *Handler) riderAvailability(c *gin.Context) {
	if h.availability == nil {
		response.OK(c, gin.H{"enabled": false})
		return
	}
	riderID := h.currentRiderID(c)
	if riderID == 0 {
		return
	}
	value, err := h.availability.ReadRiderAvailability(c.Request.Context(), riderID)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, availabilityView(value))
}

func (h *Handler) startRiderAvailability(c *gin.Context) {
	h.changeRiderAvailability(c, delivery.RiderOnDuty)
}

func (h *Handler) stopRiderAvailability(c *gin.Context) {
	h.changeRiderAvailability(c, delivery.RiderOffDuty)
}

func (h *Handler) changeRiderAvailability(c *gin.Context, state delivery.RiderDutyState) {
	if h.availability == nil {
		response.Fail(c, http.StatusServiceUnavailable, "接单功能暂未开放")
		return
	}
	var request availabilityVersionRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedRowVersion == nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	riderID := h.currentRiderID(c)
	if riderID == 0 {
		return
	}
	value, err := h.availability.SetRiderDuty(c.Request.Context(), riderID, state, *request.ExpectedRowVersion, h.now().UTC())
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, availabilityView(value))
}

func (h *Handler) currentRiderID(c *gin.Context) uint64 {
	if h.currentUserID == nil {
		response.Fail(c, http.StatusServiceUnavailable, "接单状态暂不可用")
		return 0
	}
	riderID := h.currentUserID(c)
	if riderID == 0 {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
	}
	return riderID
}

func (h *Handler) riderBroadcastConfig(c *gin.Context) {
	if h.availability == nil {
		response.OK(c, gin.H{"enabled": false})
		return
	}
	value, err := h.availability.ReadRiderBroadcastConfig(c.Request.Context())
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, broadcastConfigView(value))
}

func (h *Handler) updateRiderBroadcastConfig(c *gin.Context) {
	if h.availability == nil {
		response.Fail(c, http.StatusServiceUnavailable, "骑手接单配置暂未开放")
		return
	}
	var request riderBroadcastConfigRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedRowVersion == nil || request.MaxInFlightPerRider == nil ||
		*request.ExpectedRowVersion == 0 || *request.MaxInFlightPerRider == 0 || *request.MaxInFlightPerRider > 100 {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.availability.UpdateRiderBroadcastConfig(c.Request.Context(), *request.MaxInFlightPerRider,
		*request.ExpectedRowVersion, h.now().UTC())
	if err != nil {
		if errors.Is(err, delivery.ErrInvalidRecord) {
			response.Fail(c, http.StatusBadRequest, "invalid request")
			return
		}
		h.write(c, nil, err)
		return
	}
	response.OK(c, broadcastConfigView(value))
}
