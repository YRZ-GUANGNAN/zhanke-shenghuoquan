package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func deliveryWecomGroupFormatter(merchant, rider bool) notification.WecomGroupFormatter {
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
		result := make([]notification.WecomGroupMessage, 0, 2)
		if merchant {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "merchant_ops",
				Title:    "商家订单 · " + label,
				Summary:  label,
			})
		}
		if rider && riderStatuses[fact.To] {
			result = append(result, notification.WecomGroupMessage{
				GroupKey: "rider_ops",
				Title:    "骑手调度 · " + label,
				Summary:  label,
			})
		}
		return result, nil
	}
}
