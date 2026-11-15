package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func deliveryWecomGroupFormatter(merchantEnabled, riderEnabled func() bool) notification.WecomGroupFormatter {
	labels := map[delivery.Status]string{
		delivery.StatusPaymentFailed: "配送费支付失败",
		delivery.StatusReadyForClaim: "配送单等待接单",
		delivery.StatusClaimed:       "骑手已接单",
		delivery.StatusToPickup:      "骑手正在前往取货",
		delivery.StatusPickedUp:      "骑手已取货",
		delivery.StatusDelivering:    "配送中",
		delivery.StatusDelivered:     "配送已完成",
		delivery.StatusCancelled:     "配送单已取消",
	}
	riderStatuses := map[delivery.Status]bool{
		delivery.StatusReadyForClaim: true,
		delivery.StatusClaimed:       true,
		delivery.StatusToPickup:      true,
		delivery.StatusPickedUp:      true,
		delivery.StatusDelivering:    true,
		delivery.StatusDelivered:     true,
		delivery.StatusCancelled:     true,
	}
	return func(event notification.Event) ([]notification.WecomGroupMessage, error) {
		if event.AggregateType != "delivery_order" || event.EventType != delivery.EventOrderStatusChanged {
			return nil, nil
		}
		decoder := json.NewDecoder(bytes.NewReader(event.Payload))
		decoder.DisallowUnknownFields()
		var fact delivery.OrderNotificationPayload
		if decoder.Decode(&fact) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) ||
			fact.OrderPublicID != event.AggregatePublicID || fact.MerchantUserID == 0 ||
			!delivery.CanTransition(fact.From, fact.To) {
			return nil, errors.New("invalid WeCom order notification payload")
		}
		label, ok := labels[fact.To]
		if !ok {
			return nil, nil
		}
		platform := sourcePlatformCardLabel(fact.SourcePlatform)
		items := []notification.WecomCardItem{
			{Key: "单号", Value: fact.OrderPublicID},
			{Key: "起点", Value: fact.PickupAddress},
			{Key: "终点", Value: fact.DeliveryAddress},
			{Key: "状态", Value: label},
			{Key: "时间", Value: event.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("01-02 15:04")},
		}
		result := make([]notification.WecomGroupMessage, 0, 2)
		if merchantEnabled() {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "merchant_ops",
				Title:    "商家订单 · " + label,
				Summary:  label,
				Card: &notification.WecomTemplateCard{
					SourceDesc:    "湛科生活圈 · " + platform,
					Title:         "新订单通知",
					Desc:          platform + " · " + label,
					EmphasisTitle: yuanLabel(fact.DeliveryFeeCents),
					EmphasisDesc:  "配送费",
					Items:         items,
					ActionURL:     wecomOrderActionURL,
				},
			})
		}
		if riderEnabled() && riderStatuses[fact.To] {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "rider_ops",
				Title:    "骑手调度 · " + label,
				Summary:  label,
				Card: &notification.WecomTemplateCard{
					SourceDesc:    "湛科生活圈 · " + platform,
					Title:         "新订单通知",
					Desc:          platform + " · " + label,
					EmphasisTitle: yuanLabel(fact.RiderFeeCents),
					EmphasisDesc:  "赏金",
					Items:         items,
					ActionURL:     wecomOrderActionURL,
				},
			})
		}
		return result, nil
	}
}

// 企微卡片底部动作跳转；暂指向站点，小程序接单页 scheme 就绪后替换。
const wecomOrderActionURL = "https://zhanduiwang.top/"

func sourcePlatformCardLabel(platform string) string {
	switch platform {
	case "jd":
		return "京东订单"
	case "taobao":
		return "淘宝订单"
	case "meituan":
		return "美团订单"
	default:
		return "小程序独立订单"
	}
}

func yuanLabel(cents uint64) string {
	if cents%100 == 0 {
		return fmt.Sprintf("%d元", cents/100)
	}
	return fmt.Sprintf("%d.%02d元", cents/100, cents%100)
}
