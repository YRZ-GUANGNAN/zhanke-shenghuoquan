package notificationconsole

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/pkg/response"
)

type Config struct {
	UserNotifications       bool
	SubscribeEnabled        bool
	NotificationSending     bool
	NotificationAudience    string
	OfficialEnabled         bool
	OfficialAppID           string
	OfficialTemplateReady   bool
	WecomMerchantConfigured bool
	WecomRiderConfigured    bool
	// Workers 返回通知 worker 的运行状态，未配置时后台不展示该区块。
	Workers func() []notification.WorkerState
}

type Handler struct {
	repository Repository
	config     Config
	sender     *notification.WeComGroupSender
	now        func() time.Time
}

func NewHandler(repository Repository, config Config, sender *notification.WeComGroupSender, now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{repository: repository, config: config, sender: sender, now: now}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/notification-channels", h.overview)
	group.GET("/notification-channels/deliveries", h.deliveries)
	group.POST("/notification-channels/wecom/test", h.testWecom)
}

type DeliveryView struct {
	DeliveryID    string     `json:"deliveryId"`
	RecipientID   uint64     `json:"recipientId"`
	Channel       string     `json:"channel"`
	Purpose       string     `json:"purpose"`
	Status        string     `json:"status"`
	AttemptCount  uint32     `json:"attemptCount"`
	ErrorCode     string     `json:"errorCode"`
	Error         string     `json:"error"`
	NextAttemptAt *time.Time `json:"nextAttemptAt"`
	ProcessedAt   *time.Time `json:"processedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type DeliveryPage struct {
	Items    []DeliveryView `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}

// 只允许按状态筛选，避免把任意 SQL 条件透传到仓储层。
var deliveryStatuses = map[string]notification.DeliveryStatus{
	"pending":    notification.DeliveryPending,
	"processing": notification.DeliveryProcessing,
	"accepted":   notification.DeliveryAccepted,
	"delivered":  notification.DeliveryDelivered,
	"read":       notification.DeliveryRead,
	"retry_wait": notification.DeliveryRetryWait,
	"unknown":    notification.DeliveryUnknown,
	"blocked":    notification.DeliveryBlocked,
	"skipped":    notification.DeliverySkipped,
	"deadletter": notification.DeliveryDeadletter,
}

func (h *Handler) deliveries(c *gin.Context) {
	page := parsePositive(c.Query("page"), 1)
	pageSize := parsePositive(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	status := notification.DeliveryStatus("")
	if raw := c.Query("status"); raw != "" {
		value, ok := deliveryStatuses[raw]
		if !ok {
			response.Fail(c, http.StatusBadRequest, "invalid delivery status")
			return
		}
		status = value
	}
	records, total, err := h.repository.RecentDeliveries(c.Request.Context(), status, pageSize, (page-1)*pageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "notification deliveries unavailable")
		return
	}
	items := make([]DeliveryView, 0, len(records))
	for _, record := range records {
		items = append(items, DeliveryView{
			DeliveryID: record.DeliveryPublicID, RecipientID: record.RecipientUserID,
			Channel: string(record.Channel), Purpose: string(record.Purpose), Status: string(record.Status),
			AttemptCount: record.AttemptCount, ErrorCode: record.LastErrorCode, Error: record.LastError,
			NextAttemptAt: record.NextAttemptAt, ProcessedAt: record.ProcessedAt, CreatedAt: record.CreatedAt,
		})
	}
	response.OK(c, DeliveryPage{Items: items, Page: page, PageSize: pageSize, Total: total})
}

func parsePositive(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

type ChannelView struct {
	Channel    notification.Channel `json:"channel"`
	Label      string               `json:"label"`
	Enabled    bool                 `json:"enabled"`
	Pending    int64                `json:"pending"`
	Accepted   int64                `json:"accepted"`
	RetryWait  int64                `json:"retryWait"`
	Blocked    int64                `json:"blocked"`
	Deadletter int64                `json:"deadletter"`
}

type Overview struct {
	CheckedAt      time.Time     `json:"checkedAt"`
	SendingEnabled bool          `json:"sendingEnabled"`
	Audience       string        `json:"audience"`
	Channels       []ChannelView `json:"channels"`
	Official       struct {
		Enabled            bool  `json:"enabled"`
		TemplateConfigured bool  `json:"templateConfigured"`
		Unknown            int64 `json:"unknown"`
		Following          int64 `json:"following"`
		NotFollowing       int64 `json:"notFollowing"`
	} `json:"officialAccount"`
	Wecom struct {
		MerchantConfigured bool `json:"merchantConfigured"`
		RiderConfigured    bool `json:"riderConfigured"`
	} `json:"wecom"`
	Workers []notification.WorkerState `json:"workers"`
}

func (h *Handler) overview(c *gin.Context) {
	now := h.now().UTC()
	stats, err := h.repository.ChannelStats(c.Request.Context(), now.Add(-24*time.Hour))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "notification channel stats unavailable")
		return
	}
	officialCounts, err := h.repository.OfficialAccountCounts(c.Request.Context(), h.config.OfficialAppID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "official account stats unavailable")
		return
	}
	channels := []ChannelView{
		{Channel: notification.ChannelInApp, Label: "小程序内通知", Enabled: h.config.UserNotifications},
		{Channel: notification.ChannelWechatSubscribe, Label: "微信小程序订阅", Enabled: h.config.SubscribeEnabled},
		{Channel: notification.ChannelWechatOfficial, Label: "微信服务号", Enabled: h.config.OfficialEnabled},
		{Channel: notification.ChannelWecom, Label: "企业微信群", Enabled: h.config.WecomMerchantConfigured || h.config.WecomRiderConfigured},
	}
	index := map[notification.Channel]int{}
	for i := range channels {
		index[channels[i].Channel] = i
	}
	for _, stat := range stats {
		i, ok := index[stat.Channel]
		if !ok {
			continue
		}
		switch stat.Status {
		case notification.DeliveryPending, notification.DeliveryProcessing:
			channels[i].Pending += stat.Count
		case notification.DeliveryAccepted, notification.DeliveryDelivered, notification.DeliveryRead:
			channels[i].Accepted += stat.Count
		case notification.DeliveryRetryWait, notification.DeliveryUnknown:
			channels[i].RetryWait += stat.Count
		case notification.DeliveryBlocked, notification.DeliverySkipped:
			channels[i].Blocked += stat.Count
		case notification.DeliveryDeadletter:
			channels[i].Deadletter += stat.Count
		}
	}
	audience := h.config.NotificationAudience
	if audience != "all" {
		audience = "test"
	}
	result := Overview{
		CheckedAt: now, SendingEnabled: h.config.NotificationSending,
		Audience: audience, Channels: channels,
	}
	result.Official.Enabled = h.config.OfficialEnabled
	result.Official.TemplateConfigured = h.config.OfficialTemplateReady
	result.Official.Unknown = officialCounts.Unknown
	result.Official.Following = officialCounts.Following
	result.Official.NotFollowing = officialCounts.NotFollowing
	result.Wecom.MerchantConfigured = h.config.WecomMerchantConfigured
	result.Wecom.RiderConfigured = h.config.WecomRiderConfigured
	if h.config.Workers != nil {
		result.Workers = h.config.Workers()
	}
	response.OK(c, result)
}

func (h *Handler) testWecom(c *gin.Context) {
	var request struct {
		Group string `json:"group"`
	}
	if c.ShouldBindJSON(&request) != nil || (request.Group != "merchant_ops" && request.Group != "rider_ops") {
		response.Fail(c, http.StatusBadRequest, "invalid WeCom group")
		return
	}
	if h.sender == nil || !h.sender.Enabled(request.Group) {
		response.Fail(c, http.StatusServiceUnavailable, "WeCom group is not configured")
		return
	}
	now := h.now().UTC()
	status, err := h.sender.Send(c.Request.Context(), notification.WecomGroupSnapshot{
		Version: 1, GroupKey: request.Group, Title: "通知通道测试",
		Summary: "后台测试消息，不关联真实订单", AggregateType: "delivery_order",
		AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", AggregateVersion: 1, OccurredAt: now,
	})
	if err != nil || status != notification.SendStatusAccepted {
		if errors.Is(err, notification.ErrWecomConfiguration) {
			response.Fail(c, http.StatusServiceUnavailable, "WeCom group is unavailable")
		} else {
			response.Fail(c, http.StatusBadGateway, "WeCom test message was not accepted")
		}
		return
	}
	response.OK(c, gin.H{"accepted": true, "group": request.Group})
}
