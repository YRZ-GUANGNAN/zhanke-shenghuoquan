package im

import "context"

// Peer is presentation data only. The authorized conversation remains the
// source of truth for participant IDs and Tencent IM routing.
type conversationPeer struct {
	ID       uint   `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type conversationView struct {
	ConversationContext
	Peer *conversationPeer `json:"peer,omitempty"`
}

type conversationPageView struct {
	Items    []conversationView `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}

func peerIDForViewer(value ConversationContext, viewerID uint) uint {
	if viewerID == 0 {
		return 0
	}
	if value.BuyerID == viewerID {
		return value.SellerID
	}
	if value.SellerID == viewerID {
		return value.BuyerID
	}
	return 0
}

// Called only after Service has authorized the conversation(s). A profile
// lookup failure must not make an otherwise valid conversation unavailable.
func (h *Handler) conversationViews(ctx context.Context, values []ConversationContext, viewerID uint) []conversationView {
	views := make([]conversationView, len(values))
	ids := make([]uint, 0, len(values))
	seen := make(map[uint]bool)
	for i, value := range values {
		views[i].ConversationContext = value
		if id := peerIDForViewer(value, viewerID); id != 0 && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if h.profiles == nil || len(ids) == 0 {
		return views
	}
	profiles, err := h.profiles(ctx, ids)
	if err != nil {
		return views
	}
	for i, value := range values {
		id := peerIDForViewer(value, viewerID)
		if profile, ok := profiles[id]; id != 0 && ok && profile.ID == id {
			views[i].Peer = &conversationPeer{ID: id, Nickname: profile.Nickname, Avatar: profile.Avatar}
		}
	}
	return views
}
