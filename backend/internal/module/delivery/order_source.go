package delivery

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type SourcePlatform string

const (
	SourceLocal   SourcePlatform = "local"
	SourceTaobao  SourcePlatform = "taobao"
	SourceJD      SourcePlatform = "jd"
	SourceMeituan SourcePlatform = "meituan"
)

var ErrSourceOrderExists = errors.New("source order already has a delivery order")

// SourceMetadata describes an order's provenance; it does not prove a platform
// connection. PickupCode is a display identifier, never an idempotency key.
type SourceMetadata struct {
	SourcePlatform SourcePlatform
	PickupCode     string
	SourceOrderID  *string
	SourceShopID   *string
}

func (p SourcePlatform) Effective() SourcePlatform {
	if p == "" {
		return SourceLocal
	}
	return p
}

func NormalizeSourceMetadata(platform SourcePlatform, pickupCode, orderID, shopID string) (SourceMetadata, error) {
	platform = SourcePlatform(strings.TrimSpace(string(platform))).Effective()
	switch platform {
	case SourceLocal, SourceTaobao, SourceJD, SourceMeituan:
	default:
		return SourceMetadata{}, ErrInvalidRecord
	}
	pickupCode, orderID, shopID = strings.TrimSpace(pickupCode), strings.TrimSpace(orderID), strings.TrimSpace(shopID)
	if !validSourceText(pickupCode, 16) || !validSourceText(orderID, 128) || !validSourceText(shopID, 64) ||
		(orderID == "") != (shopID == "") || (platform == SourceLocal && orderID != "") {
		return SourceMetadata{}, ErrInvalidRecord
	}
	result := SourceMetadata{SourcePlatform: platform, PickupCode: pickupCode}
	if orderID != "" {
		result.SourceOrderID, result.SourceShopID = &orderID, &shopID
	}
	return result, nil
}

func validSourceText(value string, maxRunes int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func validOrderSource(value Order) bool {
	orderID, shopID := "", ""
	if value.SourceOrderID != nil {
		orderID = *value.SourceOrderID
	}
	if value.SourceShopID != nil {
		shopID = *value.SourceShopID
	}
	normalized, err := NormalizeSourceMetadata(value.SourcePlatform, value.PickupCode, orderID, shopID)
	return err == nil && normalized.SourcePlatform == value.SourcePlatform.Effective() &&
		normalized.PickupCode == value.PickupCode &&
		stringPointerEqual(normalized.SourceOrderID, value.SourceOrderID) &&
		stringPointerEqual(normalized.SourceShopID, value.SourceShopID)
}

// ExternalOrderAdapter is a reserved integration boundary. There is deliberately
// no provider implementation or registered webhook in this release. An adapter
// must verify the provider signature, timestamp and event replay token against
// a server-owned shop binding before returning normalized data. Returning an
// event is not permission to charge a wallet or publish a delivery order.
type ExternalOrderAdapter interface {
	Platform() SourcePlatform
	VerifyAndNormalize(context.Context, ExternalOrderEnvelope, ExternalShopBinding) (ExternalOrderEvent, error)
}

type ExternalOrderEnvelope struct {
	Payload    []byte
	Headers    map[string][]string
	ReceivedAt time.Time
}

// ExternalShopBinding must come from authenticated server configuration, never
// from an untrusted callback's merchant ID or a manually selected platform.
type ExternalShopBinding struct {
	MerchantUserID uint64
	Platform       SourcePlatform
	SourceShopID   string
}

type ExternalOrderEventType string

const (
	ExternalOrderCreated   ExternalOrderEventType = "created"
	ExternalOrderUpdated   ExternalOrderEventType = "updated"
	ExternalOrderCancelled ExternalOrderEventType = "cancelled"
)

type ExternalOrderEvent struct {
	EventID    string
	EventType  ExternalOrderEventType
	OccurredAt time.Time
	Source     SourceMetadata
	Pickup     Contact
	Recipient  Contact
	Remark     string
}

// NormalizeExternalOrderEvent is the shared typed boundary adapters must use
// before an event can be handed to an intake workflow. It validates provider
// identity and source identifiers but intentionally does not create, pay, or
// publish a delivery order.
func NormalizeExternalOrderEvent(adapterPlatform SourcePlatform, binding ExternalShopBinding, event ExternalOrderEvent) (ExternalOrderEvent, error) {
	adapterPlatform = adapterPlatform.Effective()
	binding.Platform = binding.Platform.Effective()
	if adapterPlatform == SourceLocal || adapterPlatform != binding.Platform || event.Source.SourcePlatform.Effective() != adapterPlatform ||
		binding.MerchantUserID == 0 || strings.TrimSpace(binding.SourceShopID) == "" ||
		strings.TrimSpace(event.EventID) == "" || !validSourceText(event.EventID, 128) || event.OccurredAt.IsZero() {
		return ExternalOrderEvent{}, ErrInvalidRecord
	}
	if event.EventType != ExternalOrderCreated && event.EventType != ExternalOrderUpdated && event.EventType != ExternalOrderCancelled {
		return ExternalOrderEvent{}, ErrInvalidRecord
	}
	orderID, shopID := "", ""
	if event.Source.SourceOrderID != nil {
		orderID = *event.Source.SourceOrderID
	}
	if event.Source.SourceShopID != nil {
		shopID = *event.Source.SourceShopID
	}
	normalized, err := NormalizeSourceMetadata(adapterPlatform, event.Source.PickupCode, orderID, shopID)
	if err != nil || normalized.SourceShopID == nil || *normalized.SourceShopID != binding.SourceShopID {
		return ExternalOrderEvent{}, ErrInvalidRecord
	}
	event.Source = normalized
	return event, nil
}
