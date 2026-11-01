package notification

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// IMCallbackIdentityResolver maps the provider account identifiers to local
// users and re-checks the application relation at callback time. Implementors
// must fail closed when the mapping, active state, contact permission, or
// block state cannot be established.
type IMCallbackIdentityResolver interface {
	ResolveIMAccount(context.Context, string) (uint64, error)
	ResolveC2CRelation(context.Context, uint64, uint64) (IMNotificationRelation, error)
}

// IMNotificationRelation contains only trusted local facts needed to decide
// whether a private-message notification may be created. The resolver owns
// the source of truth for all fields; provider callback values are never used
// to bypass these checks.
type IMNotificationRelation struct {
	SenderUserID         uint64
	RecipientUserID      uint64
	SenderActive         bool
	RecipientActive      bool
	Allowed              bool
	Blocked              bool
	ConversationPublicID string
	TargetPath           string
}

// IMCallbackEventSink is intentionally smaller than the notification
// repository. It gives the callback handler one atomic write boundary and
// keeps provider-specific code out of the domain repository.
type IMCallbackEventSink interface {
	RecordCallbackAndEvent(context.Context, CallbackReceipt, Event) (bool, error)
}

// IMCallbackHandler accepts Tencent IM's after-send callbacks. It does not
// trust the client-side SDK result, retain message bodies, or emit WeChat
// notifications directly. C2C callbacks are converted into metadata-only
// events; GROUP callbacks are acknowledged but deliberately ignored.
type IMCallbackHandler struct {
	Config       IMCallbackConfig
	Identity     IMCallbackIdentityResolver
	Events       IMCallbackEventSink
	Now          func() time.Time
	MaxBodyBytes int64
}

func NewIMCallbackHandler(cfg IMCallbackConfig, identity IMCallbackIdentityResolver, events IMCallbackEventSink) *IMCallbackHandler {
	return &IMCallbackHandler{Config: cfg, Identity: identity, Events: events, MaxBodyBytes: 128 << 10}
}

// Register mounts the callback beneath the supplied group. For example,
// registering on api.Group("/notification/tencent-im") produces
// POST /notification/tencent-im/callback.
func (h *IMCallbackHandler) Register(group *gin.RouterGroup) {
	if group == nil {
		return
	}
	group.POST("/callback", h.handle)
}

// Handle is exported for composition roots that need to mount the callback at
// a pre-existing route group while preserving the same safety boundary.
func (h *IMCallbackHandler) Handle(c *gin.Context) { h.handle(c) }

func (h *IMCallbackHandler) handle(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.Identity == nil || h.Events == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	maxBytes := h.MaxBodyBytes
	if maxBytes <= 0 || maxBytes > 128<<10 {
		maxBytes = 128 << 10
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusBadRequest)
		return
	}

	// ParseIMCallback requires an absolute URL, but the callback signature is
	// computed from query parameters only. Use a fixed sentinel origin so a
	// spoofed Host header never enters the authentication calculation.
	rawURL := "https://tencent-im.invalid/callback"
	if c.Request.URL != nil && c.Request.URL.RawQuery != "" {
		rawURL += "?" + c.Request.URL.RawQuery
	}
	callback, err := ParseIMCallback(rawURL, body, h.Config)
	if err != nil {
		switch {
		case errors.Is(err, ErrIMCallbackAuth):
			c.Status(http.StatusUnauthorized)
		case errors.Is(err, ErrIMCallbackPayload):
			c.Status(http.StatusBadRequest)
		case errors.Is(err, ErrIMCallbackUnsupported), errors.Is(err, ErrIMCallbackRejected):
			// Unsupported/rejected provider callbacks are not actionable. ACK
			// without echoing provider details so Tencent does not retry forever.
			writeIMCallbackAck(c)
		default:
			c.Status(http.StatusBadRequest)
		}
		return
	}
	if callback.ConversationType == IMCallbackGroup {
		// Group messages are delivery-workgroup traffic, not private-message
		// notifications. The group membership and order lifecycle have their own
		// domain path; this endpoint must never turn a group callback into a C2C
		// inbox item.
		writeIMCallbackAck(c)
		return
	}

	senderID, err := h.Identity.ResolveIMAccount(c.Request.Context(), callback.SenderAccount)
	if err != nil || senderID == 0 {
		// Unknown/stale account mappings are intentionally acknowledged without
		// revealing whether a provider account exists.
		if err != nil && !errors.Is(err, ErrIMCallbackRelation) {
			c.Status(http.StatusServiceUnavailable)
		} else {
			writeIMCallbackAck(c)
		}
		return
	}
	recipientID, err := h.Identity.ResolveIMAccount(c.Request.Context(), callback.RecipientAccount)
	if err != nil || recipientID == 0 || senderID == recipientID {
		if err != nil && !errors.Is(err, ErrIMCallbackRelation) {
			c.Status(http.StatusServiceUnavailable)
		} else {
			writeIMCallbackAck(c)
		}
		return
	}
	relation, err := h.Identity.ResolveC2CRelation(c.Request.Context(), senderID, recipientID)
	if err != nil || !validIMNotificationRelation(relation, senderID, recipientID) {
		if err != nil && !errors.Is(err, ErrIMCallbackRelation) {
			c.Status(http.StatusServiceUnavailable)
		} else {
			writeIMCallbackAck(c)
		}
		return
	}
	event, err := BuildPrivateMessageEvent(callback, relation)
	if err != nil {
		writeIMCallbackAck(c)
		return
	}
	receipt := CallbackReceipt{
		SDKAppID:             callback.SDKAppID,
		ProviderMessageID:    callback.ProviderMessageID,
		SenderAccount:        callback.SenderAccount,
		RecipientAccount:     callback.RecipientAccount,
		SenderUserID:         senderID,
		RecipientUserID:      recipientID,
		CallbackHash:         callback.CallbackHash,
		ConversationType:     callback.ConversationType,
		ConversationPublicID: relation.ConversationPublicID,
		Status:               "received",
		OccurredAt:           callback.OccurredAt,
		CreatedAt:            h.clock(callback.OccurredAt),
	}
	if _, err := h.Events.RecordCallbackAndEvent(c.Request.Context(), receipt, event); err != nil {
		// A persistence outage is the only callback failure exposed as a retryable
		// status. The response still contains no provider body or credentials.
		c.Status(http.StatusServiceUnavailable)
		return
	}
	writeIMCallbackAck(c)
}

func writeIMCallbackAck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ActionStatus": "OK", "ErrorCode": 0, "ErrorInfo": ""})
}

func (h *IMCallbackHandler) clock(fallback time.Time) time.Time {
	if h != nil && h.Now != nil {
		if now := h.Now().UTC(); !now.IsZero() {
			return now
		}
	}
	return fallback.UTC()
}

func validIMNotificationRelation(value IMNotificationRelation, senderID, recipientID uint64) bool {
	return senderID > 0 && recipientID > 0 && senderID != recipientID &&
		value.SenderUserID == senderID && value.RecipientUserID == recipientID &&
		value.SenderActive && value.RecipientActive && value.Allowed && !value.Blocked &&
		safeText(value.ConversationPublicID, 128, true) && validTargetPath(value.TargetPath)
}

func validTargetPath(value string) bool {
	if value != strings.TrimSpace(value) || len(value) > 256 || !strings.HasPrefix(value, "/pages/") || strings.ContainsAny(value, "\r\n\x00\\") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" && parsed.Fragment == "" &&
		!strings.Contains(parsed.Path, "..") && !strings.Contains(parsed.Path, "//")
}

// BuildPrivateMessageEvent creates a metadata-only notification event. The
// event payload intentionally excludes provider accounts and all message
// content; the local relation and user IDs are sufficient for expansion.
func BuildPrivateMessageEvent(callback IMCallback, relation IMNotificationRelation) (Event, error) {
	if callback.ConversationType != IMCallbackC2C || callback.SDKAppID == 0 || !safeText(callback.ProviderMessageID, 128, true) || callback.UnreadMessageCount < 0 || callback.OccurredAt.IsZero() || !validIMNotificationRelation(relation, relation.SenderUserID, relation.RecipientUserID) {
		return Event{}, ErrInvalidRecord
	}
	version := privateMessageVersion(callback)
	now := callback.OccurredAt.UTC()
	expires := now.Add(24 * time.Hour)
	payload, err := json.Marshal(struct {
		SenderUserID       uint64 `json:"senderUserId"`
		RecipientUserID    uint64 `json:"recipientUserId"`
		ProviderMessageID  string `json:"providerMessageId"`
		MessageSequence    uint32 `json:"messageSequence,omitempty"`
		UnreadMessageCount int64  `json:"unreadMessageCount,omitempty"`
		TargetPath         string `json:"targetPath"`
		OccurredAt         string `json:"occurredAt"`
	}{
		SenderUserID:       relation.SenderUserID,
		RecipientUserID:    relation.RecipientUserID,
		ProviderMessageID:  callback.ProviderMessageID,
		MessageSequence:    callback.MsgSequence,
		UnreadMessageCount: callback.UnreadMessageCount,
		TargetPath:         relation.TargetPath,
		OccurredAt:         callback.OccurredAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil || !validMetadata(payload) {
		return Event{}, ErrInvalidRecord
	}
	id, err := newID()
	if err != nil {
		return Event{}, err
	}
	return Event{
		EventPublicID:     id,
		EventType:         EventPrivateMessageReceived,
		AggregateType:     "im_conversation",
		AggregatePublicID: relation.ConversationPublicID,
		AggregateVersion:  version,
		PayloadVersion:    1,
		Payload:           payload,
		Status:            EventPending,
		RowVersion:        1,
		OccurredAt:        now,
		ExpiresAt:         &expires,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func privateMessageVersion(callback IMCallback) uint64 {
	// C2C MsgSeq is not a conversation-wide sequence: two directions may use
	// the same value. Hash the full provider idempotency coordinate instead.
	coordinate, _ := json.Marshal([]string{
		strconv.FormatUint(callback.SDKAppID, 10), callback.ProviderMessageID,
		callback.SenderAccount, callback.RecipientAccount,
	})
	digest := sha256.Sum256(coordinate)
	version := binary.BigEndian.Uint64(digest[:8])
	if version == 0 {
		return 1
	}
	return version
}
