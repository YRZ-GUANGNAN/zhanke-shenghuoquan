package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const testSecret = "test-secret-that-is-longer-than-thirty-two-characters"

type memoryRepository struct {
	users map[uint]User
}

func newMemoryRepository(values ...User) *memoryRepository {
	r := &memoryRepository{users: map[uint]User{}}
	for _, value := range values {
		r.users[value.ID] = value
	}
	return r
}

func (r *memoryRepository) FindByOpenID(_ context.Context, openID string) (User, error) {
	for _, value := range r.users {
		if value.OpenID == openID {
			return value, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *memoryRepository) FindByID(_ context.Context, id uint) (User, error) {
	value, ok := r.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) FindProfiles(_ context.Context, ids []uint) (map[uint]Profile, error) {
	values := make(map[uint]Profile, len(ids))
	for _, id := range ids {
		if value, ok := r.users[id]; ok {
			values[id] = value.Profile()
		}
	}
	return values, nil
}

func (r *memoryRepository) Create(_ context.Context, value *User) error {
	if value.ID == 0 {
		value.ID = uint(len(r.users) + 1)
	}
	r.users[value.ID] = *value
	return nil
}

func (r *memoryRepository) UpdateProfile(_ context.Context, id uint, profile Profile) error {
	value, ok := r.users[id]
	if !ok {
		return ErrNotFound
	}
	value.Nickname, value.Avatar, value.School, value.Campus = profile.Nickname, profile.Avatar, profile.School, profile.Campus
	r.users[id] = value
	return nil
}

func (r *memoryRepository) List(_ context.Context, _, _ int) ([]User, int64, error) {
	values := make([]User, 0, len(r.users))
	for _, value := range r.users {
		values = append(values, value)
	}
	return values, int64(len(values)), nil
}

func (r *memoryRepository) UpdateStatusAndRevoke(_ context.Context, id uint, status Status, protectLast bool) error {
	if protectLast {
		active, _ := r.CountActiveSuperAdmins(context.Background())
		if active <= 1 {
			return ErrLastActiveSuperAdmin
		}
	}
	value, ok := r.users[id]
	if !ok {
		return ErrNotFound
	}
	value.Status = status
	value.TokenVersion++
	r.users[id] = value
	return nil
}

func (r *memoryRepository) UpdateRoleAndRevoke(_ context.Context, id uint, role Role, protectLast bool) error {
	if protectLast {
		active, _ := r.CountActiveSuperAdmins(context.Background())
		if active <= 1 {
			return ErrLastActiveSuperAdmin
		}
	}
	value, ok := r.users[id]
	if !ok {
		return ErrNotFound
	}
	value.Role = role
	value.TokenVersion++
	r.users[id] = value
	return nil
}

func (r *memoryRepository) CountActiveSuperAdmins(_ context.Context) (int64, error) {
	var total int64
	for _, value := range r.users {
		if value.Role == RoleSuperAdmin && value.Status == StatusActive {
			total++
		}
	}
	return total, nil
}

func (r *memoryRepository) Count(context.Context) (int64, int64, error) {
	return int64(len(r.users)), 0, nil
}

func TestTokenRoundTripAndRejectsTampering(t *testing.T) {
	token, err := generateToken(testSecret, 42, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(token, ".")) != 3 {
		t.Fatalf("expected compact JWT, got %q", token)
	}
	claims, err := parseToken(testSecret, token)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := claims.UserID(); id != 42 || claims.TokenVersion != 7 {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	parts := strings.Split(token, ".")
	if parts[2][0] == 'A' {
		parts[2] = "B" + parts[2][1:]
	} else {
		parts[2] = "A" + parts[2][1:]
	}
	if _, err := parseToken(testSecret, strings.Join(parts, ".")); !errors.Is(err, errInvalidToken) {
		t.Fatalf("tampered token accepted: %v", err)
	}

	expired, err := generateToken(testSecret, 42, 7, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseToken(testSecret, expired); !errors.Is(err, errInvalidToken) && !errors.Is(err, errTokenExpired) {
		t.Fatalf("expired token accepted: %v", err)
	}
}

func TestAuthenticateUsesLiveStatusRoleAndTokenVersion(t *testing.T) {
	repo := newMemoryRepository(User{ID: 1, OpenID: "wx-1", Role: RoleUser, Status: StatusActive, TokenVersion: 1})
	service := NewService(repo, testSecret, "", "", true)
	token, err := generateToken(testSecret, 1, 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	value := repo.users[1]
	value.Role = RoleAdmin
	repo.users[1] = value
	authenticated, err := service.Authenticate(context.Background(), token)
	if err != nil || authenticated.Role != RoleAdmin {
		t.Fatalf("live role was not loaded: user=%+v err=%v", authenticated, err)
	}

	value = repo.users[1]
	value.Status = StatusBanned
	repo.users[1] = value
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrBanned) {
		t.Fatalf("banned user accepted: %v", err)
	}

	value.Status = StatusActive
	value.TokenVersion = 2
	repo.users[1] = value
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("revoked token accepted: %v", err)
	}
}

func TestMockWechatLoginMustBeExplicitAndCannotCreateAdmin(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, testSecret, "", "", false)
	if _, err := service.WechatLogin(context.Background(), LoginInput{Code: "admin"}); !errors.Is(err, ErrWechatNotConfigured) {
		t.Fatalf("implicit mock login was allowed: %v", err)
	}

	service = NewService(repo, testSecret, "", "", true)
	result, err := service.WechatLogin(context.Background(), LoginInput{Code: "admin", Avatar: "https://unreviewed.example/avatar.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Role != RoleUser || result.User.OpenID != "mock_openid_admin" {
		t.Fatalf("mock login escalated privileges: %+v", result.User)
	}
	if result.User.Avatar != "" {
		t.Fatalf("first-login avatar was persisted before media review: %q", result.User.Avatar)
	}
}

func TestAdministrativeAccountBoundariesAndImmediateRevocation(t *testing.T) {
	repo := newMemoryRepository(
		User{ID: 1, Role: RoleSuperAdmin, Status: StatusActive, TokenVersion: 1},
		User{ID: 2, Role: RoleAdmin, Status: StatusActive, TokenVersion: 1},
		User{ID: 3, Role: RoleUser, Status: StatusActive, TokenVersion: 1},
	)
	service := NewService(repo, testSecret, "", "", true)
	userToken, _ := generateToken(testSecret, 3, 1, time.Minute)

	if err := service.AdminUpdateStatus(context.Background(), 2, 1, StatusBanned); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin changed superadmin status: %v", err)
	}
	if err := service.AdminUpdateStatus(context.Background(), 2, 2, StatusBanned); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("admin changed own status: %v", err)
	}
	if err := service.AdminUpdateStatus(context.Background(), 2, 3, StatusMuted); err != nil {
		t.Fatalf("admin could not moderate user: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), userToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("status change did not revoke prior token: %v", err)
	}
	if err := service.AdminUpdateRole(context.Background(), 2, 3, RoleAdmin); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin promoted a user: %v", err)
	}
	if err := service.AdminUpdateRole(context.Background(), 1, 3, RoleAdmin); err != nil {
		t.Fatalf("superadmin could not promote user: %v", err)
	}
}

func TestMutedUserCanReadButCannotWriteOrAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRepository(User{ID: 1, Role: RoleAdmin, Status: StatusMuted, TokenVersion: 1})
	service := NewService(repo, testSecret, "", "", true)
	token, _ := generateToken(testSecret, 1, 1, time.Minute)

	for _, test := range []struct {
		name       string
		middleware []gin.HandlerFunc
		want       int
	}{
		{name: "read", middleware: []gin.HandlerFunc{service.AuthRequired()}, want: http.StatusOK},
		{name: "write", middleware: []gin.HandlerFunc{service.AuthRequired(), UnmutedRequired()}, want: http.StatusForbidden},
		{name: "admin", middleware: []gin.HandlerFunc{service.AuthRequired(), AdminRequired()}, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			handlers := append(test.middleware, func(c *gin.Context) { c.Status(http.StatusOK) })
			router.GET("/test", handlers...)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != test.want {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
		})
	}
}
