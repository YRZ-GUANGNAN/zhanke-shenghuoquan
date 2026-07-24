package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"campus-circle/backend/internal/module/user"
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
)

type UserLookup interface {
	Get(context.Context, uint) (user.User, error)
}

type SessionResult struct {
	Token     string
	CSRFToken string
	User      user.User
	ExpiresAt time.Time
}

type AuthenticatedSession struct {
	Session   WebSession
	User      user.User
	CSRFToken string
}

type Service struct {
	repo      Repository
	users     UserLookup
	now       func() time.Time
	dummyHash string
}

func NewService(repo Repository, users UserLookup) (*Service, error) {
	dummy, err := hashPassword("dummy-password-never-valid")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, users: users, now: func() time.Time { return time.Now().UTC() }, dummyHash: dummy}, nil
}

func (s *Service) Bootstrap(ctx context.Context, userID uint, password, eventType string) error {
	if eventType != "credential_bootstrap" && eventType != "credential_reset" {
		return errors.New("invalid credential operation")
	}
	value, err := s.users.Get(ctx, userID)
	if err != nil || value.Status != user.StatusActive || value.Role != user.RoleSuperAdmin {
		return ErrNotSuperAdmin
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := s.now()
	credential, err := s.repo.UpsertCredential(ctx, userID, FixedUsername, hash, now)
	if err != nil {
		return err
	}
	if eventType == "credential_reset" {
		if err := s.repo.RevokeCredentialSessions(ctx, credential.ID, now); err != nil {
			return err
		}
	}
	return s.audit(ctx, &credential, eventType, "success", nil, nil, "")
}

func (s *Service) Login(ctx context.Context, username, password, ip, userAgent string) (SessionResult, error) {
	ipHash := digest(ip)
	userAgentHash := digest(userAgent)
	now := s.now()
	credential, findErr := s.repo.FindCredential(ctx, username)
	encoded := s.dummyHash
	if findErr == nil {
		encoded = credential.PasswordHash
	} else if !errors.Is(findErr, ErrCredentialNotFound) {
		return SessionResult{}, findErr
	}

	passwordOK := verifyPassword(encoded, password)
	if username != FixedUsername || findErr != nil || !passwordOK {
		var credentialPointer *Credential
		if findErr == nil {
			credentialPointer = &credential
		}
		_ = s.audit(ctx, credentialPointer, "login", "failure", ipHash, userAgentHash, "invalid credentials")
		if findErr == nil {
			recent, err := s.repo.CountRecentFailures(ctx, username, ipHash, now.Add(-lockDuration))
			if err == nil {
				attempts := credential.FailedAttempts + 1
				var lockedUntil *time.Time
				if recent >= lockThreshold {
					until := now.Add(lockDuration)
					lockedUntil = &until
					_ = s.audit(ctx, &credential, "account_locked", "blocked", ipHash, userAgentHash, "")
				}
				_ = s.repo.RecordFailure(ctx, credential.ID, attempts, lockedUntil)
			}
		}
		return SessionResult{}, ErrInvalidCredentials
	}
	if credential.LockedUntil != nil && credential.LockedUntil.After(now) {
		_ = s.audit(ctx, &credential, "login", "blocked", ipHash, userAgentHash, "")
		return SessionResult{}, ErrLocked
	}
	value, err := s.users.Get(ctx, credential.UserID)
	if err != nil || value.Status != user.StatusActive || value.Role != user.RoleSuperAdmin {
		_ = s.audit(ctx, &credential, "login", "failure", ipHash, userAgentHash, "account unavailable")
		return SessionResult{}, ErrInvalidCredentials
	}
	token, tokenHash, err := newSecret()
	if err != nil {
		return SessionResult{}, err
	}
	csrf := csrfForToken(token)
	csrfHash := digest(csrf)
	session := WebSession{
		CredentialID: credential.ID, UserID: value.ID,
		TokenHash: tokenHash, CSRFHash: csrfHash,
		IdleExpiresAt: now.Add(idleTTL), AbsoluteExpiresAt: now.Add(absoluteTTL),
		LastSeenAt: now, IPHash: ipHash, UserAgentHash: userAgentHash,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, &session); err != nil {
		return SessionResult{}, err
	}
	if err := s.repo.ResetFailures(ctx, credential.ID); err != nil {
		return SessionResult{}, err
	}
	_ = s.audit(ctx, &credential, "login", "success", ipHash, userAgentHash, "")
	return SessionResult{Token: token, CSRFToken: csrf, User: value, ExpiresAt: session.AbsoluteExpiresAt}, nil
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
	value, err := s.users.Get(ctx, session.UserID)
	if err != nil || value.Status != user.StatusActive || value.Role != user.RoleSuperAdmin {
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
	return AuthenticatedSession{Session: session, User: value, CSRFToken: csrfForToken(token)}, nil
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
	credential, _ := s.repo.FindCredential(ctx, FixedUsername)
	return s.audit(ctx, &credential, "logout", "success", digest(ip), digest(userAgent), "")
}

func (s *Service) ChangePassword(ctx context.Context, token, oldPassword, newPassword, ip, userAgent string) error {
	authenticated, err := s.Authenticate(ctx, token)
	if err != nil {
		return ErrInvalidSession
	}
	credential, err := s.repo.FindCredential(ctx, FixedUsername)
	if err != nil || credential.ID != authenticated.Session.CredentialID || !verifyPassword(credential.PasswordHash, oldPassword) {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repo.UpdatePasswordAndRevoke(ctx, credential.ID, hash, now); err != nil {
		return err
	}
	return s.audit(ctx, &credential, "password_changed", "success", digest(ip), digest(userAgent), "")
}

func (s *Service) audit(ctx context.Context, credential *Credential, eventType, outcome string, ipHash, userAgentHash []byte, detail string) error {
	value := AuditLog{
		Username: FixedUsername, EventType: eventType, Outcome: outcome,
		IPHash: ipHash, UserAgentHash: userAgentHash, Detail: detail, CreatedAt: s.now(),
	}
	if credential != nil {
		value.CredentialID = &credential.ID
		value.UserID = &credential.UserID
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
