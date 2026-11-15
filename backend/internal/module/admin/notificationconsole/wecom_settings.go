package notificationconsole

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/pkg/response"
)

// webhookHint masks a webhook URL for display; only the key suffix is shown.
func webhookHint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if index := strings.LastIndex(raw, "key="); index >= 0 {
		key := raw[index+4:]
		if len(key) > 6 {
			key = key[len(key)-6:]
		}
		return "尾号 " + key
	}
	if len(raw) > 8 {
		return "尾号 " + raw[len(raw)-6:]
	}
	return "已配置"
}

// updateWecom saves administrator-managed WeCom webhooks and applies them to
// the running senders immediately. Empty values disable the channel even when
// env configuration exists, and this choice survives restarts.
func (h *Handler) updateWecom(c *gin.Context) {
	if h.settings == nil {
		response.Fail(c, http.StatusServiceUnavailable, "渠道配置暂不可用")
		return
	}
	// 指针字段：缺省表示"保持不变"，空字符串表示"停用该群"。
	var request struct {
		MerchantWebhook    *string `json:"merchantWebhook"`
		RiderWebhook       *string `json:"riderWebhook"`
		ExpectedRowVersion uint64  `json:"expectedRowVersion"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.ExpectedRowVersion == 0 {
		response.Fail(c, http.StatusBadRequest, "请求格式无效，请刷新后重试")
		return
	}
	current, err := h.settings.LoadWeComSettings(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "读取渠道配置失败")
		return
	}
	merchant := current.MerchantWebhook
	if request.MerchantWebhook != nil {
		merchant = strings.TrimSpace(*request.MerchantWebhook)
	}
	rider := current.RiderWebhook
	if request.RiderWebhook != nil {
		rider = strings.TrimSpace(*request.RiderWebhook)
	}
	if err := notification.ValidateWeComGroupURL(merchant); err != nil {
		response.Fail(c, http.StatusBadRequest, "商家值班群地址必须是企业微信群机器人 Webhook（https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…）")
		return
	}
	if err := notification.ValidateWeComGroupURL(rider); err != nil {
		response.Fail(c, http.StatusBadRequest, "骑手值班群地址必须是企业微信群机器人 Webhook（https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…）")
		return
	}
	var actor uint64
	if h.actor != nil {
		actor = h.actor(c)
	}
	saved, err := h.settings.SaveWeComSettings(c.Request.Context(), merchant, rider, request.ExpectedRowVersion, actor, h.now())
	if err != nil {
		if errors.Is(err, ErrSettingsConflict) {
			response.Fail(c, http.StatusConflict, "配置已被其他管理员修改，请刷新后重试")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "保存渠道配置失败")
		return
	}
	// 库中留空的字段沿用环境变量，避免只填写一个群时把另一个群静默停用。
	effectiveMerchant, effectiveRider := h.effectiveWecom(saved)
	if h.sender != nil {
		_ = h.sender.SetGroupURL("merchant_ops", effectiveMerchant)
		_ = h.sender.SetGroupURL("rider_ops", effectiveRider)
	}
	if h.riderSender != nil {
		_ = h.riderSender.SetWebhookURL(effectiveRider)
	}
	response.OK(c, gin.H{
		"merchantConfigured": effectiveMerchant != "",
		"riderConfigured":    effectiveRider != "",
		"merchantHint":       webhookHint(effectiveMerchant),
		"riderHint":          webhookHint(effectiveRider),
		"settingsSaved":      saved.Saved,
		"rowVersion":         saved.RowVersion,
	})
}

// effectiveWecom 合并数据库配置与环境变量：数据库字段为空时沿用 env。
func (h *Handler) effectiveWecom(saved WeComSettings) (string, string) {
	merchant := strings.TrimSpace(saved.MerchantWebhook)
	if merchant == "" {
		merchant = strings.TrimSpace(h.config.EnvMerchantWebhook)
	}
	rider := strings.TrimSpace(saved.RiderWebhook)
	if rider == "" {
		rider = strings.TrimSpace(h.config.EnvRiderWebhook)
	}
	return merchant, rider
}
