package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func deliveryWecomGroupFormatter(db *gorm.DB, merchantEnabled, riderEnabled func() bool, miniAppID string) notification.WecomGroupFormatter {
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
		reassigned := event.EventType == delivery.EventOrderReassigned
		if event.AggregateType != "delivery_order" ||
			(event.EventType != delivery.EventOrderStatusChanged && !reassigned) {
			return nil, nil
		}
		// 两个值班群都未启用时卡片不会产出，闸门查询也不执行：
		// 群通道特有的 SQL 问题不应连累站内信/订阅等非群聊通道。
		merchantOn, riderOn := merchantEnabled(), riderEnabled()
		if !merchantOn && !riderOn {
			return nil, nil
		}
		decoder := json.NewDecoder(bytes.NewReader(event.Payload))
		decoder.DisallowUnknownFields()
		var fact delivery.OrderNotificationPayload
		if decoder.Decode(&fact) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) ||
			fact.OrderPublicID != event.AggregatePublicID || fact.MerchantUserID == 0 {
			return nil, errors.New("invalid WeCom order notification payload")
		}
		if reassigned {
			// 改派不改状态（From==To），但必须真的换了骑手。
			if fact.From != fact.To || fact.RiderUserID == nil || fact.PreviousRiderUserID == nil ||
				*fact.RiderUserID == *fact.PreviousRiderUserID {
				return nil, errors.New("invalid WeCom order notification payload")
			}
		} else if !delivery.CanTransition(fact.From, fact.To) {
			return nil, errors.New("invalid WeCom order notification payload")
		}
		// 群卡片发送前用实时订单行复核事件，避免重试/乱序把过期状态广播到值班群。
		// 查询失败必须显式报错（走重试/死信，可见），不能伪装成「无需发送」。
		if db != nil {
			var current struct {
				Status      string
				RowVersion  uint64
				RiderUserID uint64
			}
			if err := db.WithContext(context.Background()).Raw(
				`SELECT status, row_version, IFNULL(rider_user_id, 0) AS rider_user_id FROM delivery_orders WHERE delivery_order_public_id = ?`,
				fact.OrderPublicID).Scan(&current).Error; err != nil {
				return nil, fmt.Errorf("wecom order state lookup failed: %w", err)
			}
			stale := current.Status == "" || current.RowVersion < event.AggregateVersion
			if reassigned {
				// 改派卡讲的是骑手归属：订单后续推进不使卡片过期，
				// 只有「又被改派」或订单已终结才让旧事件失效。
				stale = stale || fact.RiderUserID == nil || current.RiderUserID == 0 ||
					*fact.RiderUserID != current.RiderUserID ||
					current.Status == string(delivery.StatusCancelled) || current.Status == string(delivery.StatusDelivered)
			} else {
				stale = stale || current.RowVersion != event.AggregateVersion || current.Status != string(fact.To)
			}
			if stale {
				return nil, nil
			}
		}
		label, ok := labels[fact.To]
		if !ok && !reassigned {
			return nil, nil
		}
		if reassigned {
			label = "订单已改派"
		}
		platform := sourcePlatformCardLabel(fact.SourcePlatform)
		// 卡片遵循京东/美团式层级：标题即状态，状态不再重复出现在条目里。
		desc := platform + " · 单号 " + shortOrderID(fact.OrderPublicID)
		items := []notification.WecomCardItem{
			{Key: "单号", Value: fact.OrderPublicID},
			{Key: "起点", Value: fact.PickupAddress},
			{Key: "终点", Value: fact.DeliveryAddress},
			{Key: "时间", Value: event.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("01-02 15:04")},
		}
		result := make([]notification.WecomGroupMessage, 0, 2)
		if merchantOn {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "merchant_ops",
				Title:    "商家订单 · " + label,
				Summary:  label,
				Card: &notification.WecomTemplateCard{
					SourceDesc:     "湛科生活圈 · " + platform,
					Title:          label,
					Desc:           desc,
					EmphasisTitle:  yuanLabel(fact.DeliveryFeeCents),
					EmphasisDesc:   "配送费",
					Items:          items,
					ActionURL:      wecomOrderActionURL,
					ActionAppID:    miniAppID,
					ActionPagePath: wecomMerchantOrderPagePath,
				},
			})
		}
		if riderOn && riderStatuses[fact.To] {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "rider_ops",
				Title:    "骑手调度 · " + label,
				Summary:  label,
				Card: &notification.WecomTemplateCard{
					SourceDesc:     "湛科生活圈 · " + platform,
					Title:          label,
					Desc:           desc,
					EmphasisTitle:  yuanLabel(fact.RiderFeeCents),
					EmphasisDesc:   "赏金",
					Items:          items,
					ActionURL:      wecomOrderActionURL,
					ActionAppID:    miniAppID,
					ActionPagePath: wecomRiderOrderPagePath,
				},
			})
		}
		return result, nil
	}
}

// 企微卡片底部动作：优先跳转小程序（card_action type 2），桌面端回退站点 URL。
const wecomOrderActionURL = "https://zhanduiwang.top/"

const (
	wecomMerchantOrderPagePath = "pages/merchant-home/index"
	wecomRiderOrderPagePath    = "pages/delivery-rider/index"
)

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

// shortOrderID 把 ULID 压成「前6…后4」的短号，便于在卡片副标题里一眼核对；
// 完整单号仍保留在条目中供值班人员复制。
func shortOrderID(publicID string) string {
	if len(publicID) <= 12 {
		return publicID
	}
	return publicID[:6] + "…" + publicID[len(publicID)-4:]
}

func yuanLabel(cents uint64) string {
	if cents%100 == 0 {
		return fmt.Sprintf("%d元", cents/100)
	}
	return fmt.Sprintf("%d.%02d元", cents/100, cents%100)
}
