package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrAuthorizationNonce       = errors.New("invalid or expired authorization nonce")
	ErrAuthorizationConflict    = errors.New("authorization result conflicts with previous report")
	ErrAuthorizationRateLimited = errors.New("authorization requests are too frequent")
)

const (
	authorizationPending  = "authorization_pending"
	authorizationReported = "authorization_reported:"
)

// OpenIDLookup is provided by the composition root. Neither clients nor this
// domain choose a user's OpenID; the result is never returned by HTTP handlers.
type OpenIDLookup func(context.Context, uint64, string) (string, error)

type SubscriptionAuthorizationRepository interface {
	IssueSubscriptionAuthorization(context.Context, Subscription, time.Time) error
	ReportSubscriptionAuthorization(context.Context, AuthorizationReport) (bool, error)
}

type AuthorizationReport struct {
	UserID      uint64
	AppID       string
	OpenID      string
	TemplateKey string
	Nonce       string
	Status      string
	Now         time.Time
	ExpiresAt   *time.Time
}

type SubscriptionConfig struct {
	Enabled          bool
	AppID            string
	Templates        map[string]string
	NonceTTL         time.Duration
	MinIssueInterval time.Duration
	AuthorizationTTL time.Duration
}

type SubscriptionTemplate struct {
	Key        string `json:"templateKey"`
	TemplateID string `json:"templateId,omitempty"`
	Available  bool   `json:"available"`
}

type AuthorizationChallenge struct {
	TemplateKey string    `json:"templateKey"`
	Nonce       string    `json:"nonce"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type SubscriptionService struct {
	repository SubscriptionAuthorizationRepository
	lookup     OpenIDLookup
	config     SubscriptionConfig
	now        func() time.Time
}

func NewSubscriptionService(repository SubscriptionAuthorizationRepository, lookup OpenIDLookup, cfg SubscriptionConfig) *SubscriptionService {
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	copyTemplates := make(map[string]string, len(cfg.Templates))
	for key, templateID := range cfg.Templates {
		if validTemplateKey(key) {
			copyTemplates[key] = strings.TrimSpace(templateID)
		}
	}
	cfg.Templates = copyTemplates
	if cfg.NonceTTL <= 0 || cfg.NonceTTL > 15*time.Minute {
		cfg.NonceTTL = 5 * time.Minute
	}
	if cfg.MinIssueInterval < time.Second {
		cfg.MinIssueInterval = 5 * time.Second
	}
	if cfg.AuthorizationTTL < 0 {
		cfg.AuthorizationTTL = 0
	}
	return &SubscriptionService{repository: repository, lookup: lookup, config: cfg, now: func() time.Time { return time.Now().UTC() }}
}

func (s *SubscriptionService) Templates() []SubscriptionTemplate {
	result := make([]SubscriptionTemplate, 0)
	if s == nil {
		return result
	}
	for key, id := range s.config.Templates {
		available := s.available(key)
		if !available {
			id = ""
		}
		result = append(result, SubscriptionTemplate{Key: key, TemplateID: id, Available: available})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func (s *SubscriptionService) available(key string) bool {
	return s != nil && s.config.Enabled && s.config.AppID != "" && s.config.Templates[key] != "" && s.repository != nil && s.lookup != nil
}

func (s *SubscriptionService) resolve(ctx context.Context, userID uint64, key string) (string, error) {
	if userID == 0 || !validTemplateKey(key) {
		return "", ErrInvalidRecord
	}
	if !s.available(key) {
		return "", ErrSubscriptionEmpty
	}
	value, err := s.lookup(ctx, userID, s.config.AppID)
	value = strings.TrimSpace(value)
	if err != nil || value == "" || len(value) > 128 {
		// Do not propagate an adapter error containing credentials or identifiers.
		return "", ErrSubscriptionEmpty
	}
	return value, nil
}

func (s *SubscriptionService) IssueNonce(ctx context.Context, userID uint64, key string) (AuthorizationChallenge, error) {
	openid, err := s.resolve(ctx, userID, key)
	if err != nil {
		return AuthorizationChallenge{}, err
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return AuthorizationChallenge{}, ErrSubscriptionEmpty
	}
	now := s.now()
	challenge := AuthorizationChallenge{TemplateKey: key, Nonce: hex.EncodeToString(raw[:]) + s.templateTag(key), ExpiresAt: now.Add(s.config.NonceTTL)}
	item := Subscription{UserID: userID, AppID: s.config.AppID, OpenID: openid, TemplateKey: key, AuthorizationNonce: challenge.Nonce,
		Status: "invalid", InvalidationReason: authorizationPending, AuthorizedAt: now, ExpiresAt: &challenge.ExpiresAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.IssueSubscriptionAuthorization(ctx, item, now.Add(-s.config.MinIssueInterval)); err != nil {
		return AuthorizationChallenge{}, err
	}
	return challenge, nil
}

func (s *SubscriptionService) Authorize(ctx context.Context, userID uint64, key, result, nonce string) (bool, error) {
	status, ok := map[string]string{"accept": "accepted", "reject": "rejected", "ban": "banned", "close": "closed", "exception": "invalid"}[result]
	if !ok || !validAuthorizationNonce(nonce) {
		return false, ErrInvalidRecord
	}
	openid, err := s.resolve(ctx, userID, key)
	if err != nil {
		return false, err
	}
	// A template key may be remapped while a challenge is outstanding. Do not
	// credit a report for the old template to the newly configured template.
	if nonce[32:] != s.templateTag(key) {
		return false, ErrAuthorizationNonce
	}
	now := s.now()
	var expires *time.Time
	if s.config.AuthorizationTTL > 0 {
		value := now.Add(s.config.AuthorizationTTL)
		expires = &value
	}
	return s.repository.ReportSubscriptionAuthorization(ctx, AuthorizationReport{UserID: userID, AppID: s.config.AppID, OpenID: openid, TemplateKey: key, Nonce: nonce, Status: status, Now: now, ExpiresAt: expires})
}

func (s *SubscriptionService) templateTag(key string) string {
	return authorizationTemplateTag(s.config.AppID, key, s.config.Templates[key])
}

func validTemplateKey(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

func validAuthorizationNonce(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validAuthorizationIdentity(item Subscription) bool {
	return item.UserID > 0 && item.AppID != "" && len(item.AppID) <= 64 && item.OpenID != "" && len(item.OpenID) <= 128 && validTemplateKey(item.TemplateKey) && validAuthorizationNonce(item.AuthorizationNonce) && item.RowVersion > 0 && !item.CreatedAt.IsZero() && !item.UpdatedAt.IsZero()
}

func validAuthorizationStatus(status string) bool {
	return status == "accepted" || status == "rejected" || status == "banned" || status == "closed" || status == "invalid"
}
