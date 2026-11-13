package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
)

// The optional store is the rollout boundary: identity approval alone must
// never turn a historical pool event into a broadcast. The production store
// rechecks current identity, duty and capacity under its admission locks.
func (r *orderNotificationRecipients) resolveRiderPoolRecipients(ctx context.Context, event notification.Event, recipientID uint64) ([]notification.Recipient, error) {
	if r.availability == nil {
		return nil, nil
	}
	if r.orders == nil || r.users == nil {
		return nil, notification.ErrInvalidRecord
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	if !delivery.ValidULID(event.EventPublicID) || !delivery.ValidULID(event.AggregatePublicID) || event.AggregateVersion == 0 ||
		event.OccurredAt.IsZero() || event.OccurredAt.After(now) || event.ExpiresAt == nil ||
		!event.ExpiresAt.After(event.OccurredAt) || event.ExpiresAt.After(event.OccurredAt.Add(10*time.Minute)) {
		return nil, notification.ErrInvalidRecord
	}
	if !event.ExpiresAt.After(now) {
		return nil, nil
	}
	var fact delivery.RiderPoolOpenedPayload
	if err := json.Unmarshal(event.Payload, &fact); err != nil || fact.OrderPublicID != event.AggregatePublicID || fact.DeliveryFeeCents == 0 {
		return nil, notification.ErrInvalidRecord
	}
	if current, err := r.riderPoolOrderCurrent(ctx, event, fact); err != nil || !current {
		return nil, err
	}
	candidates := []delivery.RiderAvailabilitySnapshot{{RiderUserID: recipientID}}
	if recipientID == 0 {
		var err error
		candidates, err = r.availability.ListOnDutyRiderAvailability(ctx, now)
		if err != nil {
			return nil, errors.New("notification rider candidates lookup failed")
		}
	}
	seen := make(map[uint64]bool, len(candidates))
	result := make([]notification.Recipient, 0, len(candidates))
	for _, candidate := range candidates {
		id := candidate.RiderUserID
		if id == 0 || uint64(uint(id)) != id || seen[id] {
			continue
		}
		seen[id] = true
		// The list only nominates IDs. Do not trust an old candidate snapshot
		// after a concurrent stop, capacity edit, qualification change or claim.
		available, err := r.availability.ReadRiderAvailability(ctx, id)
		if errors.Is(err, delivery.ErrRiderUnavailable) || errors.Is(err, delivery.ErrRiderNotRegistered) || errors.Is(err, delivery.ErrRiderOffDuty) || errors.Is(err, delivery.ErrRiderAtCapacity) {
			continue
		}
		if err != nil {
			return nil, errors.New("notification rider availability lookup failed")
		}
		if !available.Valid() || available.RiderUserID != id {
			return nil, notification.ErrInvalidRecord
		}
		if available.State != delivery.RiderOnDuty || available.Scope != delivery.RiderAvailabilityScopeCampus || available.InFlight >= available.MaxInFlight {
			continue
		}
		account, err := r.users.Get(ctx, uint(id))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, errors.New("notification pool account lookup failed")
		}
		if account.ID != uint(id) || account.Status != user.StatusActive || account.ClosedAt != nil || account.DeletedAt.Valid {
			continue
		}
		settings, err := r.users.Settings(ctx, uint(id))
		if err != nil {
			return nil, errors.New("notification pool preference lookup failed")
		}
		if !settings.OrderNotice {
			continue
		}
		recipient := notification.Recipient{UserID: id, Purpose: notification.PurposeOrder, Title: "有新的校园配送单",
			Summary: "进入接单大厅查看，订单以实时状态为准", AggregateType: event.AggregateType,
			AggregatePublicID: event.AggregatePublicID, AggregateVersion: event.AggregateVersion, TargetPath: "/pages/delivery-rider/index"}
		if r.template.allowsWechat(id) {
			hour := now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour()
			if !settings.QuietHours || (hour >= 7 && hour < 23) {
				snapshot, err := r.template.snapshot(event, recipient)
				if err != nil {
					return nil, err
				}
				recipient.AllowWechat = true
				recipient.WechatTemplateKey = "order"
				recipient.WechatTemplateSnapshot = snapshot
			}
		}
		result = append(result, recipient)
	}
	// Candidate checks can wait for concurrent transactions. Re-read the order
	// after them as well, so a claim/cancellation during the scan cannot create
	// a stale broadcast; SubscribeGuard repeats only its recipient before send.
	if current, err := r.riderPoolOrderCurrent(ctx, event, fact); err != nil || !current {
		return nil, err
	}
	if notificationEventExpired(event, r.now) {
		return nil, nil
	}
	return result, nil
}

func (r *orderNotificationRecipients) riderPoolOrderCurrent(ctx context.Context, event notification.Event, fact delivery.RiderPoolOpenedPayload) (bool, error) {
	current, err := r.orders.Find(ctx, event.AggregatePublicID)
	if errors.Is(err, delivery.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("notification pool order lookup failed")
	}
	if current.DeliveryOrderPublicID != event.AggregatePublicID || current.MerchantUserID == 0 || current.DeliveryFeeCents != fact.DeliveryFeeCents {
		return false, notification.ErrInvalidRecord
	}
	return current.Status == delivery.StatusReadyForClaim && current.RiderUserID == nil && current.RowVersion == event.AggregateVersion, nil
}
