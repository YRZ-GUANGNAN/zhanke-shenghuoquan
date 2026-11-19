package adminauth

import (
	"context"
	"testing"
	"time"
)

type memoryAccountRepo struct {
	accounts  map[uint64]*Account
	byName    map[string]uint64
	sessions  map[string]*WebSession
	audits    []AuditLog
	nextID    uint64
	sessionID uint64
}

func newMemoryAccountRepo() *memoryAccountRepo {
	return &memoryAccountRepo{accounts: map[uint64]*Account{}, byName: map[string]uint64{}, sessions: map[string]*WebSession{}, nextID: 1}
}

func (r *memoryAccountRepo) FindAccountByUsername(_ context.Context, username string) (Account, error) {
	id, ok := r.byName[username]
	if !ok {
		return Account{}, ErrCredentialNotFound
	}
	return *r.accounts[id], nil
}

func (r *memoryAccountRepo) FindAccountByID(_ context.Context, id uint64) (Account, error) {
	value, ok := r.accounts[id]
	if !ok {
		return Account{}, ErrCredentialNotFound
	}
	return *value, nil
}

func (r *memoryAccountRepo) ListAccounts(context.Context) ([]Account, error) {
	values := make([]Account, 0, len(r.accounts))
	for id := uint64(1); id < r.nextID; id++ {
		if value, ok := r.accounts[id]; ok {
			values = append(values, *value)
		}
	}
	return values, nil
}

func (r *memoryAccountRepo) CreateAccount(_ context.Context, value Account) (Account, error) {
	if _, ok := r.byName[value.Username]; ok {
		return Account{}, ErrUsernameTaken
	}
	value.ID = r.nextID
	r.nextID++
	copied := value
	r.accounts[value.ID] = &copied
	r.byName[value.Username] = value.ID
	return value, nil
}

func (r *memoryAccountRepo) activeSuperAdmins() int {
	count := 0
	for _, value := range r.accounts {
		if value.Role == RoleSuperAdmin && value.Status == StatusActive {
			count++
		}
	}
	return count
}

func (r *memoryAccountRepo) UpdateAccount(_ context.Context, id uint64, displayName string, role Role, now time.Time) (Account, error) {
	value, ok := r.accounts[id]
	if !ok {
		return Account{}, ErrCredentialNotFound
	}
	if value.Role == RoleSuperAdmin && role != RoleSuperAdmin && value.Status == StatusActive && r.activeSuperAdmins() <= 1 {
		return Account{}, ErrLastActiveSuperAdmin
	}
	value.DisplayName, value.Role = displayName, role
	value.RowVersion++
	value.UpdatedAt = now
	return *value, nil
}

func (r *memoryAccountRepo) SetAccountStatus(_ context.Context, id uint64, status Status, now time.Time) (Account, error) {
	value, ok := r.accounts[id]
	if !ok {
		return Account{}, ErrCredentialNotFound
	}
	if value.Status != status && value.Role == RoleSuperAdmin && value.Status == StatusActive && status == StatusDisabled && r.activeSuperAdmins() <= 1 {
		return Account{}, ErrLastActiveSuperAdmin
	}
	value.Status = status
	value.RowVersion++
	value.UpdatedAt = now
	if status == StatusDisabled {
		for _, session := range r.sessions {
			if session.CredentialID == id && session.RevokedAt == nil {
				revoked := now
				session.RevokedAt = &revoked
			}
		}
	}
	return *value, nil
}

func (r *memoryAccountRepo) ResetAccountPassword(_ context.Context, id uint64, hash string, now time.Time) error {
	value, ok := r.accounts[id]
	if !ok {
		return ErrCredentialNotFound
	}
	value.PasswordHash = hash
	value.PasswordChangedAt = now
	value.RowVersion++
	for _, session := range r.sessions {
		if session.CredentialID == id && session.RevokedAt == nil {
			revoked := now
			session.RevokedAt = &revoked
		}
	}
	return nil
}

func (r *memoryAccountRepo) RecordFailure(_ context.Context, id uint64, attempts uint16, lockedUntil *time.Time) error {
	if value, ok := r.accounts[id]; ok {
		value.FailedAttempts = attempts
		value.LockedUntil = lockedUntil
	}
	return nil
}

func (r *memoryAccountRepo) ResetFailures(_ context.Context, id uint64) error {
	if value, ok := r.accounts[id]; ok {
		value.FailedAttempts = 0
		value.LockedUntil = nil
	}
	return nil
}

func (r *memoryAccountRepo) CreateSession(_ context.Context, value *WebSession) error {
	r.sessionID++
	value.ID = r.sessionID
	copied := *value
	r.sessions[string(value.TokenHash)] = &copied
	return nil
}

func (r *memoryAccountRepo) FindSession(_ context.Context, tokenHash []byte) (WebSession, error) {
	value, ok := r.sessions[string(tokenHash)]
	if !ok {
		return WebSession{}, ErrInvalidSession
	}
	return *value, nil
}

func (r *memoryAccountRepo) TouchSession(_ context.Context, id uint64, lastSeen, idleExpires time.Time) error {
	for _, session := range r.sessions {
		if session.ID == id {
			session.LastSeenAt = lastSeen
			session.IdleExpiresAt = idleExpires
		}
	}
	return nil
}

func (r *memoryAccountRepo) RevokeSession(_ context.Context, id uint64, now time.Time) error {
	for _, session := range r.sessions {
		if session.ID == id && session.RevokedAt == nil {
			revoked := now
			session.RevokedAt = &revoked
		}
	}
	return nil
}

func (r *memoryAccountRepo) RevokeCredentialSessions(_ context.Context, credentialID uint64, now time.Time) error {
	for _, session := range r.sessions {
		if session.CredentialID == credentialID && session.RevokedAt == nil {
			revoked := now
			session.RevokedAt = &revoked
		}
	}
	return nil
}

func (r *memoryAccountRepo) CountRecentFailures(context.Context, string, []byte, time.Time) (int64, error) {
	return 0, nil
}

func (r *memoryAccountRepo) CreateAudit(_ context.Context, value AuditLog) error {
	r.audits = append(r.audits, value)
	return nil
}

func seedAccount(t *testing.T, repo *memoryAccountRepo, username string, role Role, status Status, password string) Account {
	t.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	account, err := repo.CreateAccount(context.Background(), Account{
		Username: username, DisplayName: username, Role: role, Status: status,
		PasswordHash: hash, PasswordChangedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func newTestService(t *testing.T, repo *memoryAccountRepo) *Service {
	t.Helper()
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestLoginUsesAccountIdentityNotUsersTable(t *testing.T) {
	repo := newMemoryAccountRepo()
	seedAccount(t, repo, "ops.zhang", RoleSuperAdmin, StatusActive, "correct-horse-battery")
	service := newTestService(t, repo)

	result, err := service.Login(context.Background(), "ops.zhang", "correct-horse-battery", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.Account.Username != "ops.zhang" || result.Account.Role != RoleSuperAdmin || result.Token == "" {
		t.Fatalf("login result=%+v", result)
	}
	authenticated, err := service.Authenticate(context.Background(), result.Token)
	if err != nil || authenticated.Account.ID != result.Account.ID {
		t.Fatalf("authenticate: %+v err=%v", authenticated, err)
	}
	// 审计必须记录真实用户名，而不是历史固定值 administrator。
	if len(repo.audits) == 0 || repo.audits[len(repo.audits)-1].Username != "ops.zhang" {
		t.Fatalf("audit username not recorded: %+v", repo.audits)
	}

	seedAccount(t, repo, "second.super", RoleSuperAdmin, StatusActive, "second-password-1234")
	if _, err := repo.SetAccountStatus(context.Background(), result.Account.ID, StatusDisabled, time.Now().UTC()); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	if _, err := service.Login(context.Background(), "ops.zhang", "correct-horse-battery", "127.0.0.1", "test-agent"); err == nil {
		t.Fatal("disabled account logged in")
	}
	if _, err := service.Authenticate(context.Background(), result.Token); err == nil {
		t.Fatal("disabled account session stayed valid")
	}
}

func TestAccountManagementGuardsAndRoleChecks(t *testing.T) {
	repo := newMemoryAccountRepo()
	super := seedAccount(t, repo, "root.admin", RoleSuperAdmin, StatusActive, "root-password-1234")
	seedAccount(t, repo, "daily.admin", RoleAdmin, StatusActive, "daily-password-1234")
	service := newTestService(t, repo)

	created, err := service.CreateAccount(context.Background(), super.ID, CreateAccountInput{
		Username: "new.admin", DisplayName: "新管理员", Password: "new-admin-password", Role: RoleAdmin,
	})
	if err != nil || created.Username != "new.admin" || created.Status != StatusActive {
		t.Fatalf("create account: %+v err=%v", created, err)
	}
	if _, err := service.CreateAccount(context.Background(), super.ID, CreateAccountInput{
		Username: "new.admin", DisplayName: "", Password: "another-password-123", Role: RoleAdmin,
	}); err != ErrUsernameTaken {
		t.Fatalf("duplicate username err=%v", err)
	}
	if _, err := service.CreateAccount(context.Background(), super.ID, CreateAccountInput{
		Username: "bad name", DisplayName: "", Password: "another-password-123", Role: RoleAdmin,
	}); err != ErrInvalidUsername {
		t.Fatalf("invalid username err=%v", err)
	}
	if _, err := service.CreateAccount(context.Background(), super.ID, CreateAccountInput{
		Username: "short.pw", DisplayName: "", Password: "", Role: RoleAdmin,
	}); err != ErrPasswordPolicy {
		t.Fatalf("empty password err=%v", err)
	}
	// 普通管理员不能管理账号。
	daily := repo.byName["daily.admin"]
	if _, err := service.CreateAccount(context.Background(), daily, CreateAccountInput{
		Username: "sneaky.admin", DisplayName: "", Password: "sneaky-password-123", Role: RoleAdmin,
	}); err != ErrNotSuperAdmin {
		t.Fatalf("plain admin managed accounts err=%v", err)
	}
	// 不能降级或停用自己。
	if _, err := service.UpdateAccount(context.Background(), super.ID, super.ID, "root", RoleAdmin); err != ErrCannotModifySelf {
		t.Fatalf("self demotion err=%v", err)
	}
	if _, err := service.SetAccountStatus(context.Background(), super.ID, super.ID, StatusDisabled); err != ErrCannotModifySelf {
		t.Fatalf("self disable err=%v", err)
	}
	// 最后一名启用中的超管不能被停用（仓储层保护）。
	if _, err := repo.SetAccountStatus(context.Background(), super.ID, StatusDisabled, time.Now().UTC()); err != ErrLastActiveSuperAdmin {
		t.Fatalf("last superadmin guard err=%v", err)
	}
	// 重置密码会吊销目标账号会话。
	login, err := service.Login(context.Background(), "new.admin", "new-admin-password", "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("login new admin: %v", err)
	}
	if err := service.ResetPassword(context.Background(), super.ID, created.ID, "rotated-password-123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), login.Token); err == nil {
		t.Fatal("password reset kept the old session alive")
	}
	if _, err := service.Login(context.Background(), "new.admin", "rotated-password-123", "127.0.0.1", "agent"); err != nil {
		t.Fatalf("login after reset: %v", err)
	}
	accounts, err := service.ListAccounts(context.Background(), super.ID)
	if err != nil || len(accounts) != 3 {
		t.Fatalf("list accounts=%+v err=%v", accounts, err)
	}
}

func TestBootstrapCreatesAndResetsStandaloneAccount(t *testing.T) {
	repo := newMemoryAccountRepo()
	service := newTestService(t, repo)

	if err := service.Bootstrap(context.Background(), "first.admin", "首位管理员", RoleSuperAdmin, "first-password-1234", "credential_bootstrap"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if _, err := service.Login(context.Background(), "first.admin", "first-password-1234", "127.0.0.1", "agent"); err != nil {
		t.Fatalf("login bootstrapped account: %v", err)
	}
	if err := service.Bootstrap(context.Background(), "first.admin", "", RoleSuperAdmin, "second-password-1234", "credential_reset"); err != nil {
		t.Fatalf("bootstrap reset: %v", err)
	}
	if _, err := service.Login(context.Background(), "first.admin", "first-password-1234", "127.0.0.1", "agent"); err == nil {
		t.Fatal("old password still valid after reset")
	}
	if _, err := service.Login(context.Background(), "first.admin", "second-password-1234", "127.0.0.1", "agent"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}
