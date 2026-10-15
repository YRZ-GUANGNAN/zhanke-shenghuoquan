package user

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)

type profileRepositoryStub struct{ value User }

func (r *profileRepositoryStub) WithLockedProfile(ctx context.Context, _ uint, change func(context.Context, User) (Profile, error)) (User, error) {
	value, err := change(ctx, r.value)
	if err != nil {
		return User{}, err
	}
	r.value.Nickname, r.value.School, r.value.Campus = value.Nickname, value.School, value.Campus
	r.value.Avatar, r.value.AvatarAssetID = value.Avatar, value.AvatarAssetID
	return r.value, nil
}

func TestProfileUpdateHTTPUsesNewAvatarAndKeepsGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		status Status
		want   int
	}{
		{`{"nickname":"校园用户","avatarAssetId":"` + coverTestID + `"}`, StatusActive, 200},
		{`{"nickname":"新昵称"}`, StatusActive, 200},
		{`{"nickname":"   "}`, StatusActive, 400},
		{`{"nickname":"用户","avatarAssetId":"https://untrusted.example/image"}`, StatusActive, 400},
		{`{"nickname":"用户","avatarAssetId":"` + coverTestID + `"}`, StatusMuted, 403},
		{`{"nickname":"用户"}`, StatusClosed, 403},
	} {
		repo := &profileRepositoryStub{value: User{ID: 7, Status: tc.status, Avatar: "old-avatar", Cover: "old-cover"}}
		bindings := &coverBindingsStub{}
		service, _ := NewProfileUpdateService(repo, bindings, "https://media.example")
		h := NewHandler(NewService(newMemoryRepository(repo.value), testSecret, "", "", true))
		h.SetProfileUpdates(service)
		router := gin.New()
		router.PATCH("/me", func(c *gin.Context) { SetCurrentUser(c, repo.value); h.updateMe(c) })
		request := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.body, tc.status, response.Code, response.Body.String())
		}
		if repo.value.Cover != "old-cover" {
			t.Fatal("avatar endpoint changed cover")
		}
		if tc.want == 200 && strings.Contains(tc.body, coverTestID) && bindings.published != coremedia.PurposeAvatar {
			t.Fatal("HTTP path did not publish core avatar")
		}
	}
	h := NewHandler(NewService(newMemoryRepository(), testSecret, "", "", true))
	router := gin.New()
	h.Register(router.Group("/auth"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth/me", strings.NewReader(`{"nickname":"匿名"}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatal("anonymous profile modification allowed")
	}
}
