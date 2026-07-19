package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LoginInput struct {
	Code     string
	Nickname string
	Avatar   string
	School   string
	Campus   string
}

type LoginResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

const (
	accessTokenTTL       = 15 * time.Minute
	localMockTokenTTL    = 12 * time.Hour
	productionAppEnv     = "prod"
	defaultWechatAPIBase = "https://api.weixin.qq.com"
)

var (
	ErrBanned               = errors.New("user is banned")
	ErrMuted                = errors.New("user is muted")
	ErrForbidden            = errors.New("forbidden")
	ErrCannotModifySelf     = errors.New("cannot modify own administrative account")
	ErrLastActiveSuperAdmin = errors.New("cannot disable or demote the last active superadmin")
	ErrTokenRevoked         = errors.New("token revoked")
	ErrWechatNotConfigured  = errors.New("wechat login is not configured")
)

type Service struct {
	repo                Repository
	jwtSecret           string
	wechatAppID         string
	wechatSecret        string
	wechatAPIBase       string
	allowMockWechat     bool
	appEnv              string
	httpClient          *http.Client
	identityEligibility IdentityEligibility
	phoneResolver       WechatPhoneResolver
	feedbackMedia       FeedbackMedia
	accountClosure      AccountClosureCoordinator
}

type ServiceOption func(*Service)

func WithAppEnv(appEnv string) ServiceOption {
	return func(s *Service) { s.appEnv = appEnv }
}

func WithWechatAPIBase(value string) ServiceOption {
	return func(s *Service) {
		normalized := strings.TrimRight(strings.TrimSpace(value), "/")
		if normalized != "" {
			s.wechatAPIBase = normalized
		}
	}
}

func WithIdentityEligibility(eligibility IdentityEligibility) ServiceOption {
	return func(s *Service) { s.identityEligibility = eligibility }
}

func WithWechatPhoneResolver(resolver WechatPhoneResolver) ServiceOption {
	return func(s *Service) { s.phoneResolver = resolver }
}

func NewService(repo Repository, jwtSecret, wechatAppID, wechatSecret string, allowMockWechat bool, options ...ServiceOption) *Service {
	service := &Service{
		repo: repo, jwtSecret: jwtSecret, wechatAppID: wechatAppID, wechatSecret: wechatSecret,
		wechatAPIBase: defaultWechatAPIBase, allowMockWechat: allowMockWechat,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	for _, option := range options {
		option(service)
	}
	if service.phoneResolver == nil && wechatAppID != "" && wechatSecret != "" {
		service.phoneResolver = NewWechatPhoneClient(wechatAppID, wechatSecret, service.httpClient)
	}
	return service
}

func (s *Service) SetFeedbackMedia(value FeedbackMedia) { s.feedbackMedia = value }

func (s *Service) SetAccountClosureCoordinator(value AccountClosureCoordinator) {
	s.accountClosure = value
}

func (s *Service) WechatLogin(ctx context.Context, input LoginInput) (LoginResult, error) {
	if input.Code == "" {
		return LoginResult{}, errors.New("wechat code is required")
	}
	openID, err := s.exchangeCode(ctx, input.Code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("wechat login: %w", err)
	}
	value, err := s.repo.FindByOpenID(ctx, openID)
	if errors.Is(err, ErrNotFound) {
		value = User{
			OpenID: openID, Nickname: fallback(input.Nickname, "校园用户"), Avatar: "",
			School: fallback(input.School, "未设置"), Campus: fallback(input.Campus, "未设置"),
			CurrentIdentity: IdentityStudent, Role: RoleUser, Status: StatusActive, TokenVersion: 1,
		}
		if err := s.repo.Create(ctx, &value); err != nil {
			if !errors.Is(err, ErrOpenIDConflict) {
				return LoginResult{}, fmt.Errorf("create user: %w", err)
			}
			value, err = s.repo.FindByOpenID(ctx, openID)
			if err != nil {
				return LoginResult{}, fmt.Errorf("find user after create conflict: %w", err)
			}
		}
	} else if err != nil {
		return LoginResult{}, err
	}
	if value.Status == StatusBanned {
		return LoginResult{}, ErrBanned
	}
	if value.Status == StatusClosed {
		return LoginResult{}, ErrTokenRevoked
	}
	if value.TokenVersion == 0 {
		value.TokenVersion = 1
	}
	token, err := generateToken(s.jwtSecret, value.ID, value.TokenVersion, s.accessTokenTTL())
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate token: %w", err)
	}
	return LoginResult{Token: token, User: value}, nil
}

func (s *Service) accessTokenTTL() time.Duration {
	if s.localMockWechatEnabled() {
		return localMockTokenTTL
	}
	return accessTokenTTL
}

func (s *Service) localMockWechatEnabled() bool {
	return s.appEnv != productionAppEnv && s.allowMockWechat && s.wechatAppID == "" && s.wechatSecret == ""
}

func (s *Service) exchangeCode(ctx context.Context, code string) (string, error) {
	if s.wechatAppID == "" || s.wechatSecret == "" {
		if !s.allowMockWechat {
			return "", ErrWechatNotConfigured
		}
		return "mock_openid_" + code, nil
	}
	query := url.Values{}
	query.Set("appid", s.wechatAppID)
	query.Set("secret", s.wechatSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint := s.wechatAPIBase + "/sns/jscode2session?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errors.New("wechat code exchange request failed")
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("wechat code exchange request failed: %w", context.DeadlineExceeded)
		}
		if errors.Is(err, context.Canceled) {
			return "", fmt.Errorf("wechat code exchange request failed: %w", context.Canceled)
		}
		return "", errors.New("wechat code exchange request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wechat code exchange http status %d", resp.StatusCode)
	}
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&result); err != nil {
		return "", errors.New("wechat code exchange returned invalid JSON")
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat code exchange errcode %d", result.ErrCode)
	}
	if result.OpenID == "" {
		return "", errors.New("wechat code exchange returned empty openid")
	}
	return result.OpenID, nil
}

func (s *Service) Get(ctx context.Context, id uint) (User, error) { return s.repo.FindByID(ctx, id) }

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	claims, err := parseToken(s.jwtSecret, token)
	if err != nil {
		return User{}, err
	}
	id, err := claims.UserID()
	if err != nil {
		return User{}, err
	}
	value, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if value.Status == StatusBanned {
		return User{}, ErrBanned
	}
	if value.Status == StatusClosed {
		return User{}, ErrTokenRevoked
	}
	if value.TokenVersion == 0 || claims.TokenVersion != value.TokenVersion {
		return User{}, ErrTokenRevoked
	}
	return value, nil
}
func (s *Service) UpdateProfile(ctx context.Context, id uint, profile Profile) (User, error) {
	if profile.Nickname == "" {
		return User{}, errors.New("nickname is required")
	}
	if err := s.repo.UpdateProfile(ctx, id, profile); err != nil {
		return User{}, err
	}
	return s.repo.FindByID(ctx, id)
}
func (s *Service) Profiles(ctx context.Context, ids []uint) (map[uint]Profile, error) {
	return s.repo.FindProfiles(ctx, ids)
}
func (s *Service) AdminList(ctx context.Context, page, size int) ([]User, int64, error) {
	return s.repo.List(ctx, normalizePage(page), normalizeSize(size))
}
func (s *Service) AdminUpdateStatus(ctx context.Context, actorID, id uint, status Status) error {
	if status != StatusActive && status != StatusMuted && status != StatusBanned {
		return errors.New("invalid user status")
	}
	actor, target, err := s.adminActorAndTarget(ctx, actorID, id)
	if err != nil {
		return err
	}
	if actor.ID == target.ID {
		return ErrCannotModifySelf
	}
	if actor.Role == RoleAdmin && target.Role != RoleUser {
		return ErrForbidden
	}
	protectLast := target.Role == RoleSuperAdmin && target.Status == StatusActive && status != StatusActive
	return s.repo.UpdateStatusAndRevoke(ctx, id, status, protectLast)
}

func (s *Service) AdminUpdateRole(ctx context.Context, actorID, id uint, role Role) error {
	if role != RoleUser && role != RoleAdmin && role != RoleSuperAdmin {
		return errors.New("invalid user role")
	}
	actor, target, err := s.adminActorAndTarget(ctx, actorID, id)
	if err != nil {
		return err
	}
	if actor.Role != RoleSuperAdmin {
		return ErrForbidden
	}
	if actor.ID == target.ID {
		return ErrCannotModifySelf
	}
	protectLast := target.Role == RoleSuperAdmin && target.Status == StatusActive && role != RoleSuperAdmin
	return s.repo.UpdateRoleAndRevoke(ctx, id, role, protectLast)
}

func (s *Service) adminActorAndTarget(ctx context.Context, actorID, targetID uint) (User, User, error) {
	actor, err := s.repo.FindByID(ctx, actorID)
	if err != nil {
		return User{}, User{}, err
	}
	if actor.Status != StatusActive || (actor.Role != RoleAdmin && actor.Role != RoleSuperAdmin) {
		return User{}, User{}, ErrForbidden
	}
	target, err := s.repo.FindByID(ctx, targetID)
	return actor, target, err
}

func (s *Service) Stats(ctx context.Context) (int64, int64, error) { return s.repo.Count(ctx) }

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}
func normalizeSize(size int) int {
	if size < 1 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}
