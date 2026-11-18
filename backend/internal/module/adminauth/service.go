package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const (
	idleTTL       = 30 * time.Minute
	absoluteTTL   = 8 * time.Hour
	lockDuration  = 30 * time.Minute
	lockThreshold = int64(10)
)

var (
	ErrInvalidCredentials = errors.New("invalid administrator credentials")
	ErrLocked             = errors.New("administrator login temporarily locked")
	ErrInvalidSession     = errors.New("invalid administrator session")
	ErrCSRF               = errors.New("invalid csrf token")
	ErrNotSuperAdmin      = errors.New("active superadmin required")
	ErrCannotModifySelf   = errors.New("cannot disable or demote yourself")
	ErrInvalidRole        = errors.New("invalid administrator role")
	ErrInvalidStatus      = errors.New("invalid administrator status")
	ErrInvalidUsername    = errors.New("admin username must be 3-64 characters of letters, digits, dot or underscore")
	ErrInvalidDisplayName = errors.New("administrator display name must be at most 64 characters")
)

type SessionResult struct {
	Token     string
	CSRFToken string
	Account   Account
	ExpiresAt time.Time
}

type AuthenticatedSession struct {
	Session   WebSession
	Account   Account
	CSRFToken string
}

type CreateAccountInput struct {
	Username    string
	DisplayName string
	Password    string
	Role        Role
}

type Service struct {
	repo      Repository
	now       func() time.Time
	dummyHash string
}

func NewService(repo Repository) (*Service, error) {
	dummy, err := hashPassword("dummy-password-never-valid")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }, dummyHash: dummy}, nil
}

func validAdminUsername(value string) bool {
	if len(value) < 3 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func normalizeDisplayName(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len([]rune(value)) > 64 {
		return "", ErrInvalidDisplayName
	}
	return value, nil
}

func (s *Service) Login(ctx context.Context, username, password, ip, userAgent string) (SessionResult, error) {
	username = strings.TrimSpace(username)
	ipHash := digest(ip)
	userAgentHash := digest(userAgent)
	now := s.now()
	account, findErr := s.repo.FindAccountByUsername(ctx, username)
	encoded := s.dummyHash
	if findErr == nil {
		encoded = account.PasswordHash
	} else if !errors.Is(findErr, ErrCredentialNotFound) {
		return SessionResult{}, findErr
	}

	passwordOK := verifyPassword(encoded, password)
	if findErr != nil || !passwordOK {
		var credentialPointer *Account
		if findErr == nil {
			credentialPointer = &account
		}
		_ = s.audit(ctx, credentialPointer, nil, username, "login", "failure", ipHash, userAgentHash, "invalid credentials")
		if findErr == nil {
			recent, err := s.repo.CountRecentFailures(ctx, account.Username, ipHash, now.Add(-lockDuration))
			if err == nil {
				attempts := account.FailedAttempts + 1
				var lockedUntil *time.Time
				if recent >= lockThreshold {
					until := now.Add(lockDuration)
					lockedUntil = &until
					_ = s.audit(ctx, &account, nil, account.Username, "account_locked", "blocked", ipHash, userAgentHash, "")
				}
				_ = s.repo.RecordFailure(ctx, account.ID, attempts, lockedUntil)
			}
		}
		return SessionResult{}, ErrInvalidCredentials
	}
	if account.LockedUntil != nil && account.LockedUntil.After(now) {
		_ = s.audit(ctx, &account, nil, account.Username, "login", "blocked", ipHash, userAgentHash, "")
		return SessionResult{}, ErrLocked
	}
	// 账号状态与角色现在完全由后台账号自身决定，不再回查 users 表。
	if account.Status != StatusActive || !account.Role.Valid() {
		_ = s.audit(ctx, &account, nil, account.Username, "login", "failure", ipHash, userAgentHash, "account unavailable")
		return SessionResult{}, ErrInvalidCredentials
	}
	token, tokenHash, err := newSecret()
	if err != nil {
		return SessionResult{}, err
	}
	csrf := csrfForToken(token)
	csrfHash := digest(csrf)
	session := WebSession{
		CredentialID: account.ID,
		TokenHash:    tokenHash, CSRFHash: csrfHash,
		IdleExpiresAt: now.Add(idleTTL), AbsoluteExpiresAt: now.Add(absoluteTTL),
		LastSeenAt: now, IPHash: ipHash, UserAgentHash: userAgentHash,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, &session); err != nil {
		return SessionResult{}, err
	}
	if err := s.repo.ResetFailures(ctx, account.ID); err != nil {
		return SessionResult{}, err
	}
	_ = s.audit(ctx, &account, nil, account.Username, "login", "success", ipHash, userAgentHash, "")
	return SessionResult{Token: token, CSRFToken: csrf, Account: account, ExpiresAt: session.AbsoluteExpiresAt}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (AuthenticatedSession, error) {
	if token == "" {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	session, err := s.repo.FindSession(ctx, digest(token))
	if err != nil {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	now := s.now()
	if session.RevokedAt != nil || !session.IdleExpiresAt.After(now) || !session.AbsoluteExpiresAt.After(now) {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	account, err := s.repo.FindAccountByID(ctx, session.CredentialID)
	if err != nil || account.Status != StatusActive || !account.Role.Valid() {
		_ = s.repo.RevokeSession(ctx, session.ID, now)
		return AuthenticatedSession{}, ErrInvalidSession
	}
	idleExpires := now.Add(idleTTL)
	if idleExpires.After(session.AbsoluteExpiresAt) {
		idleExpires = session.AbsoluteExpiresAt
	}
	if err := s.repo.TouchSession(ctx, session.ID, now, idleExpires); err != nil {
		return AuthenticatedSession{}, err
	}
	session.IdleExpiresAt, session.LastSeenAt = idleExpires, now
	return AuthenticatedSession{Session: session, Account: account, CSRFToken: csrfForToken(token)}, nil
}

func (s *Service) VerifyCSRF(session WebSession, csrf string) error {
	if csrf == "" || !equalDigest(session.CSRFHash, digest(csrf)) {
		return ErrCSRF
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, token, ip, userAgent string) error {
	authenticated, err := s.Authenticate(ctx, token)
	if err != nil {
		return ErrInvalidSession
	}
	now := s.now()
	if err := s.repo.RevokeSession(ctx, authenticated.Session.ID, now); err != nil {
		return err
	}
	return s.audit(ctx, &authenticated.Account, nil, authenticated.Account.Username, "logout", "success", digest(ip), digest(userAgent), "")
}

// Bootstrap 供运维 CLI 使用：创建管理员账号，或在账号已存在时重置其密码。
// 不依赖任何小程序 users 行。
func (s *Service) Bootstrap(ctx context.Context, username, displayName string, role Role, password, eventType string) error {
	if eventType != "credential_bootstrap" && eventType != "credential_reset" {
		return errors.New("invalid credential operation")
	}
	username = strings.TrimSpace(username)
	if !validAdminUsername(username) {
		return ErrInvalidUsername
	}
	if !role.Valid() {
		return ErrInvalidRole
	}
	displayName, err := normalizeDisplayName(displayName, username)
	if err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := s.now()
	existing, findErr := s.repo.FindAccountByUsername(ctx, username)
	if errors.Is(findErr, ErrCredentialNotFound) {
		account, err := s.repo.CreateAccount(ctx, Account{
			Username: username, DisplayName: displayName, Role: role, Status: StatusActive,
			PasswordHash: hash, PasswordChangedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return s.audit(ctx, &account, nil, username, eventType, "success", nil, nil, "")
	}
	if findErr != nil {
		return findErr
	}
	if err := s.repo.ResetAccountPassword(ctx, existing.ID, hash, now); err != nil {
		return err
	}
	existing.PasswordHash = hash
	return s.audit(ctx, &existing, nil, username, eventType, "success", nil, nil, "")
}

func (s *Service) ChangePassword(ctx context.Context, token, oldPassword, newPassword, ip, userAgent string) error {
	authenticated, err := s.Authenticate(ctx, token)
	if err != nil {
		return ErrInvalidSession
	}
	if !verifyPassword(authenticated.Account.PasswordHash, oldPassword) {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repo.ResetAccountPassword(ctx, authenticated.Account.ID, hash, now); err != nil {
		return err
	}
	return s.audit(ctx, &authenticated.Account, &authenticated.Account.ID, authenticated.Account.Username, "password_changed", "success", digest(ip), digest(userAgent), "")
}

// ---------- 管理员账号管理（仅启用中的超级管理员可操作） ----------

func (s *Service) requireSuperAdmin(ctx context.Context, accountID uint64) (Account, error) {
	account, err := s.repo.FindAccountByID(ctx, accountID)
	if err != nil || account.Status != StatusActive || account.Role != RoleSuperAdmin {
		return Account{}, ErrNotSuperAdmin
	}
	return account, nil
}

func (s *Service) CreateAccount(ctx context.Context, actorID uint64, input CreateAccountInput) (AccountView, error) {
	if _, err := s.requireSuperAdmin(ctx, actorID); err != nil {
		return AccountView{}, err
	}
	username := strings.TrimSpace(input.Username)
	if !validAdminUsername(username) {
		return AccountView{}, ErrInvalidUsername
	}
	if !input.Role.Valid() {
		return AccountView{}, ErrInvalidRole
	}
	displayName, err := normalizeDisplayName(input.DisplayName, username)
	if err != nil {
		return AccountView{}, err
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		return AccountView{}, err
	}
	now := s.now()
	account, err := s.repo.CreateAccount(ctx, Account{
		Username: username, DisplayName: displayName, Role: input.Role, Status: StatusActive,
		PasswordHash: hash, PasswordChangedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return AccountView{}, err
	}
	_ = s.audit(ctx, &account, &actorID, account.Username, "account_created", "success", nil, nil, "")
	return accountView(account, now), nil
}

func (s *Service) UpdateAccount(ctx context.Context, actorID, targetID uint64, displayName string, role Role) (AccountView, error) {
	actor, err := s.requireSuperAdmin(ctx, actorID)
	if err != nil {
		return AccountView{}, err
	}
	if !role.Valid() {
		return AccountView{}, ErrInvalidRole
	}
	displayName, err = normalizeDisplayName(displayName, "")
	if err != nil {
		return AccountView{}, err
	}
	if actorID == targetID && role != actor.Role {
		return AccountView{}, ErrCannotModifySelf
	}
	now := s.now()
	account, err := s.repo.UpdateAccount(ctx, targetID, displayName, role, now)
	if err != nil {
		return AccountView{}, err
	}
	event := "account_updated"
	if actor.Role != account.Role && account.ID != actorID {
		event = "admin_role_changed"
	}
	_ = s.audit(ctx, &account, &actorID, account.Username, event, "success", nil, nil, "")
	return accountView(account, now), nil
}

func (s *Service) SetAccountStatus(ctx context.Context, actorID, targetID uint64, status Status) (AccountView, error) {
	if _, err := s.requireSuperAdmin(ctx, actorID); err != nil {
		return AccountView{}, err
	}
	if !status.Valid() {
		return AccountView{}, ErrInvalidStatus
	}
	if actorID == targetID && status == StatusDisabled {
		return AccountView{}, ErrCannotModifySelf
	}
	now := s.now()
	account, err := s.repo.SetAccountStatus(ctx, targetID, status, now)
	if err != nil {
		return AccountView{}, err
	}
	event := "account_enabled"
	if status == StatusDisabled {
		event = "account_disabled"
	}
	_ = s.audit(ctx, &account, &actorID, account.Username, event, "success", nil, nil, "")
	return accountView(account, now), nil
}

func (s *Service) ResetPassword(ctx context.Context, actorID, targetID uint64, password string) error {
	if _, err := s.requireSuperAdmin(ctx, actorID); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repo.ResetAccountPassword(ctx, targetID, hash, now); err != nil {
		return err
	}
	account, err := s.repo.FindAccountByID(ctx, targetID)
	if err == nil {
		_ = s.audit(ctx, &account, &actorID, account.Username, "password_reset", "success", nil, nil, "")
	}
	return nil
}

func (s *Service) ListAccounts(ctx context.Context, actorID uint64) ([]AccountView, error) {
	if _, err := s.requireSuperAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	result := make([]AccountView, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountView(account, now))
	}
	return result, nil
}

func accountView(account Account, now time.Time) AccountView {
	return AccountView{
		ID: account.ID, Username: account.Username, DisplayName: account.DisplayName,
		Role: account.Role, Status: account.Status,
		Locked:        account.LockedUntil != nil && account.LockedUntil.After(now),
		PasswordSetAt: account.PasswordChangedAt, LegacyUserID: account.LegacyUserID,
	}
}

func (s *Service) audit(ctx context.Context, account *Account, actorID *uint64, username, eventType, outcome string, ipHash, userAgentHash []byte, detail string) error {
	value := AuditLog{
		ActorCredentialID: actorID,
		Username:          strings.TrimSpace(username),
		EventType:         eventType, Outcome: outcome,
		IPHash: ipHash, UserAgentHash: userAgentHash, Detail: detail, CreatedAt: s.now(),
	}
	if len([]rune(value.Username)) > 64 {
		value.Username = string([]rune(value.Username)[:64])
	}
	if value.Username == "" {
		value.Username = "unknown"
	}
	if account != nil {
		value.CredentialID = &account.ID
		value.UserID = account.LegacyUserID
	}
	return s.repo.CreateAudit(ctx, value)
}

func newSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return value, digest(value), nil
}

func digest(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func csrfForToken(token string) string {
	return base64.RawURLEncoding.EncodeToString(digest("admin-csrf:" + token))
}

func equalDigest(left, right []byte) bool {
	if len(left) != sha256.Size || len(right) != sha256.Size {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
