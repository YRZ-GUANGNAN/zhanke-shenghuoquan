package featureflags

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"campus-circle/backend/internal/module/user"
)

const (
	KeyForum               = "forum"
	KeyPost                = "post"
	KeyComment             = "comment"
	KeySecondhand          = "secondhand"
	KeySecondhandPublish   = "secondhand_publish"
	KeySecondhandChat      = "secondhand_chat"
	KeyTakeout             = "takeout"
	KeyCampusTools         = "campus_tools"
	KeyDelivery            = "delivery"
	KeyMerchant            = "merchant"
	KeyRider               = "rider"
	KeyMessages            = "messages"
	KeyFollowing           = "following"
	KeySearch              = "search"
	KeyLikes               = "likes"
	KeyFavorites           = "favorites"
	KeyNotifications       = "notifications"
	KeyIdentityApplication = "identity_application"
	KeyPublish             = "publish"
	KeySecondhandComments  = "secondhand_comments"
)

var AllKeys = []string{
	KeyForum, KeyPost, KeyComment, KeySecondhand, KeySecondhandPublish,
	KeySecondhandChat, KeyTakeout, KeyCampusTools, KeyDelivery, KeyMerchant,
	KeyRider, KeyMessages, KeyFollowing,
	KeySearch, KeyLikes, KeyFavorites, KeyNotifications, KeyIdentityApplication,
	KeyPublish, KeySecondhandComments,
}

var (
	ErrLabDisabled        = errors.New("feature policy lab is disabled")
	ErrVersionConflict    = errors.New("feature policy has changed; reload and retry")
	ErrInvalidKey         = errors.New("unknown feature policy key")
	ErrInvalidPostingMode = errors.New("postingMode must be admin_only or all_users")
	ErrUnavailable        = errors.New("功能配置暂时无法读取，请稍后重试")
)

type Policy struct {
	PresentationMode string          `json:"presentationMode"`
	Flags            map[string]bool `json:"flags"`
	PostingMode      string          `json:"postingMode"`
	RowVersion       uint64          `json:"rowVersion"`
	UpdatedBy        uint            `json:"updatedBy"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	LabEnabled       bool            `json:"labEnabled"`
	Enabled          bool            `json:"enabled"`
}

type UpdateInput struct {
	Flags              map[string]bool `json:"flags"`
	PostingMode        string          `json:"postingMode"`
	ExpectedRowVersion uint64          `json:"expectedRowVersion"`
	PresentationMode   *string         `json:"presentationMode,omitempty"`
}

type Store interface {
	Ensure(context.Context) error
	Load(context.Context) (Policy, bool, error)
	CreateIfMissing(context.Context, Policy) error
	Save(context.Context, Policy, Policy) error
}

var state = struct {
	sync.RWMutex
	labEnabled  bool
	policy      Policy
	store       Store
	refreshedAt time.Time
	loadError   error
}{policy: newDefaultPolicy(false)}

func newDefaultFlags() map[string]bool {
	flags := make(map[string]bool, len(AllKeys))
	for _, key := range AllKeys {
		flags[key] = true
	}
	return flags
}

func newDefaultPolicy(labEnabled bool) Policy {
	return Policy{PresentationMode: PresentationStandard, Flags: newDefaultFlags(), PostingMode: "admin_only", RowVersion: 1, LabEnabled: labEnabled, Enabled: labEnabled}
}

// Configure without a store is reserved for tests. The server uses ConfigureStore.
func Configure(enabled bool) {
	state.Lock()
	defer state.Unlock()
	state.labEnabled = enabled
	state.policy = newDefaultPolicy(enabled)
	state.store = nil
	state.refreshedAt = time.Time{}
	state.loadError = nil
}

// ConfigureStore requires an explicitly migrated schema and durable storage.
func ConfigureStore(ctx context.Context, enabled bool, store Store) error {
	Configure(enabled)
	if !enabled {
		return nil
	}
	if store == nil {
		return errors.New("feature policy store is required when lab is enabled")
	}
	if err := store.Ensure(ctx); err != nil {
		return err
	}
	value, found, err := store.Load(ctx)
	if err != nil {
		return err
	}
	if !found {
		value = newDefaultPolicy(true)
		value.UpdatedAt = time.Now().UTC()
		if err := store.CreateIfMissing(ctx, value); err != nil {
			// Another local process may have initialized the singleton first.
			if loaded, ok, loadErr := store.Load(ctx); loadErr == nil && ok {
				value = loaded
			} else {
				return err
			}
		}
	}
	value.LabEnabled = true
	value.Enabled = true
	if !validPresentationMode(value.PresentationMode) {
		return ErrInvalidPresentationMode
	}
	state.Lock()
	state.policy = clonePolicy(value)
	state.store = store
	state.refreshedAt = time.Now()
	state.Unlock()
	return nil
}

func LabEnabled() bool {
	state.RLock()
	defer state.RUnlock()
	return state.labEnabled
}

func Snapshot() Policy {
	refresh(false)
	state.RLock()
	defer state.RUnlock()
	return clonePolicy(state.policy)
}

func Get() Policy { return Snapshot() }

func ReadPolicy(force bool) (Policy, error) {
	refresh(force)
	state.RLock()
	defer state.RUnlock()
	if state.loadError != nil {
		return clonePolicy(state.policy), ErrUnavailable
	}
	return clonePolicy(state.policy), nil
}

func Update(actorID uint, input UpdateInput) (Policy, error) {
	return UpdateContext(context.Background(), actorID, input)
}

func UpdateContext(ctx context.Context, actorID uint, input UpdateInput) (Policy, error) {
	refresh(true)
	state.Lock()
	defer state.Unlock()
	if !state.labEnabled {
		return clonePolicy(state.policy), ErrLabDisabled
	}
	if state.loadError != nil {
		return clonePolicy(state.policy), ErrUnavailable
	}
	if input.ExpectedRowVersion != state.policy.RowVersion {
		return clonePolicy(state.policy), ErrVersionConflict
	}
	mode := strings.TrimSpace(input.PostingMode)
	if mode != "admin_only" && mode != "all_users" {
		return clonePolicy(state.policy), ErrInvalidPostingMode
	}
	presentation := state.policy.PresentationMode
	if input.PresentationMode != nil {
		presentation = *input.PresentationMode
	}
	if !validPresentationMode(presentation) {
		return clonePolicy(state.policy), ErrInvalidPresentationMode
	}
	if input.Flags == nil || len(input.Flags) != len(AllKeys) {
		return clonePolicy(state.policy), ErrInvalidKey
	}
	flags := newDefaultFlags()
	for key, value := range input.Flags {
		if _, ok := flags[key]; !ok {
			return clonePolicy(state.policy), ErrInvalidKey
		}
		flags[key] = value
	}
	next := clonePolicy(state.policy)
	next.Flags = flags
	next.PostingMode = mode
	next.PresentationMode = presentation
	next.RowVersion++
	next.UpdatedBy = actorID
	next.UpdatedAt = time.Now().UTC()
	if state.store != nil {
		if err := state.store.Save(ctx, clonePolicy(state.policy), next); err != nil {
			return clonePolicy(state.policy), err
		}
	}
	state.policy = next
	state.refreshedAt = time.Now()
	return clonePolicy(next), nil
}

func Set(next Policy) Policy {
	state.Lock()
	defer state.Unlock()
	flags := newDefaultFlags()
	for key, value := range next.Flags {
		if _, ok := flags[key]; ok {
			flags[key] = value
		}
	}
	mode := strings.TrimSpace(next.PostingMode)
	if mode != "all_users" {
		mode = "admin_only"
	}
	state.policy.Flags = flags
	state.policy.PostingMode = mode
	if validPresentationMode(next.PresentationMode) {
		state.policy.PresentationMode = next.PresentationMode
	}
	state.policy.RowVersion++
	state.policy.UpdatedBy = next.UpdatedBy
	state.policy.UpdatedAt = time.Now().UTC()
	return clonePolicy(state.policy)
}

func Can(key string) bool {
	value := EffectivePolicy(Snapshot())
	if !value.Enabled {
		return true
	}
	return Effective(value.Flags, key)
}

func CanPost(role user.Role) bool {
	value := EffectivePolicy(Snapshot())
	if !value.Enabled {
		return true
	}
	if !Effective(value.Flags, KeyPost) {
		return false
	}
	return value.PostingMode == "all_users" || role == user.RoleAdmin || role == user.RoleSuperAdmin
}

func CanComment(role user.Role) bool {
	value := EffectivePolicy(Snapshot())
	if !value.Enabled {
		return true
	}
	if !Effective(value.Flags, KeyComment) {
		return false
	}
	return value.PostingMode == "all_users" || role == user.RoleAdmin || role == user.RoleSuperAdmin
}

// Effective resolves dependencies without changing saved child values.
func Effective(flags map[string]bool, key string) bool {
	if !flags[key] {
		return false
	}
	switch key {
	case KeyPost:
		return flags[KeyForum] && flags[KeyPublish]
	case KeyComment, KeyFollowing, KeyNotifications:
		return flags[KeyForum]
	case KeySecondhandPublish:
		return flags[KeySecondhand] && flags[KeyPublish]
	case KeySecondhandChat:
		return flags[KeySecondhand] && flags[KeyMessages]
	case KeySecondhandComments:
		return flags[KeySecondhand]
	}
	return true
}

func refresh(force bool) {
	state.Lock()
	defer state.Unlock()
	if !state.labEnabled || state.store == nil || (!force && time.Since(state.refreshedAt) < 2*time.Second) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, found, err := state.store.Load(ctx)
	state.refreshedAt = time.Now()
	if err != nil || !found || !validPresentationMode(value.PresentationMode) {
		state.loadError = ErrUnavailable
		for key := range state.policy.Flags {
			state.policy.Flags[key] = false
		}
		return
	}
	state.loadError = nil
	value.Enabled, value.LabEnabled = true, true
	state.policy = clonePolicy(value)
}

func clonePolicy(value Policy) Policy {
	flags := make(map[string]bool, len(value.Flags))
	for key, enabled := range value.Flags {
		flags[key] = enabled
	}
	value.Flags = flags
	return value
}
