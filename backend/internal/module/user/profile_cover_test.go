package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)

const coverTestID = "01J00000000000000000000001"

func TestProfileProjectionExposesCoverAndRedactsClosedAccount(t *testing.T) {
	value := User{ID: 7, Nickname: "封面用户", Avatar: "avatar-url", Cover: "cover-url", Status: StatusActive}
	if profile := value.Profile(); profile.Cover != value.Cover || profile.Avatar != value.Avatar {
		t.Fatalf("public profile lost saved media: %+v", profile)
	}
	value.Status = StatusClosed
	if profile := value.Profile(); profile.Cover != "" || profile.Avatar != "" || profile.Nickname != "账号已注销" {
		t.Fatalf("closed account exposed personal media: %+v", profile)
	}
}

type coverRepositoryStub struct {
	value User
	calls int
}

func (r *coverRepositoryStub) WithLockedCover(ctx context.Context, id uint, change func(context.Context, User) (ProfileCover, error)) (User, error) {
	r.calls++
	if id != r.value.ID {
		return User{}, ErrNotFound
	}
	cover, err := change(ctx, r.value)
	if err != nil {
		return User{}, err
	}
	r.value.Cover, r.value.CoverAssetID = cover.URL, cover.AssetID
	return r.value, nil
}

type coverBindingsStub struct {
	reserve                                      coremedia.BindingReserveInput
	commit                                       coremedia.BindingCommitInput
	reserveErr, commitErr, publishErr, revokeErr error
	published, revoked                           coremedia.Purpose
	keep                                         []uint64
}

func (b *coverBindingsStub) Reserve(_ context.Context, in coremedia.BindingReserveInput) (coremedia.BindingReservation, error) {
	b.reserve = in
	return coremedia.BindingReservation{Token: in.Token, References: []coremedia.BindingAsset{{InternalID: 83, AssetPublicID: coverTestID}}}, b.reserveErr
}
func (b *coverBindingsStub) Commit(_ context.Context, in coremedia.BindingCommitInput) error {
	b.commit = in
	return b.commitErr
}
func (b *coverBindingsStub) Publish(_ context.Context, purpose coremedia.Purpose, id string) error {
	b.published = purpose
	return b.publishErr
}
func (b *coverBindingsStub) Revoke(_ context.Context, purpose coremedia.Purpose, id string, keep []uint64) error {
	b.revoked, b.keep = purpose, keep
	return b.revokeErr
}

func TestProfileCoverSaveAndClearDoNotChangeOtherProfileFields(t *testing.T) {
	repo := &coverRepositoryStub{value: User{ID: 7, Nickname: "保留昵称", Avatar: "existing-avatar", School: "原学校", Status: StatusActive}}
	bindings := &coverBindingsStub{}
	service, err := NewProfileCoverService(repo, bindings, "https://media.example/")
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.Update(context.Background(), 7, coverTestID)
	if err != nil {
		t.Fatal(err)
	}
	if value.Cover != "https://media.example/api/v1/media/assets/"+coverTestID+"/content" || value.CoverAssetID == nil || *value.CoverAssetID != 83 {
		t.Fatalf("cover not persisted: %+v", value)
	}
	if value.Nickname != "保留昵称" || value.Avatar != "existing-avatar" || value.School != "原学校" {
		t.Fatal("cover overwrote other profile fields")
	}
	if bindings.reserve.OwnerID != 7 || bindings.reserve.Purpose != coremedia.PurposeProfileCover || bindings.reserve.ExistingReferenceID != "7" || bindings.commit.ReferenceType != coremedia.PurposeProfileCover || bindings.commit.ReferenceID != "7" || bindings.published != coremedia.PurposeProfileCover {
		t.Fatal("incorrect cover ownership or media lifecycle")
	}
	value, err = service.Update(context.Background(), 7, "")
	if err != nil || value.Cover != "" || value.CoverAssetID != nil || bindings.revoked != coremedia.PurposeProfileCover || len(bindings.keep) != 0 {
		t.Fatalf("clear failed: %+v %v", value, err)
	}
	if value.Avatar != "existing-avatar" {
		t.Fatal("clear removed avatar")
	}
}

func TestProfileCoverFailuresKeepPreviousCover(t *testing.T) {
	for _, stage := range []string{"reserve", "commit", "publish", "revoke"} {
		t.Run(stage, func(t *testing.T) {
			repo := &coverRepositoryStub{value: User{ID: 7, Cover: "old-cover", Status: StatusActive}}
			b := &coverBindingsStub{}
			failure := errors.New("isolated failure")
			switch stage {
			case "reserve":
				b.reserveErr = failure
			case "commit":
				b.commitErr = failure
			case "publish":
				b.publishErr = failure
			case "revoke":
				b.revokeErr = failure
			}
			s, _ := NewProfileCoverService(repo, b, "")
			if _, err := s.Update(context.Background(), 7, coverTestID); err == nil {
				t.Fatal("failure was ignored")
			}
			if repo.value.Cover != "old-cover" {
				t.Fatal("failed update changed cover")
			}
		})
	}
}

func TestProfileCoverHTTPValidationAndPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		status Status
		want   int
	}{
		{`{}`, StatusActive, 400}, {`{"assetId":null}`, StatusActive, 400}, {`{"assetId":"https://unreviewed.example/x.jpg"}`, StatusActive, 400},
		{`{"assetId":""}`, StatusActive, 200}, {`{"assetId":"` + coverTestID + `"}`, StatusActive, 200},
		{`{"assetId":""}`, StatusMuted, 403}, {`{"assetId":""}`, StatusClosed, 403},
	} {
		repo := &coverRepositoryStub{value: User{ID: 7, Status: tc.status}}
		coverService, _ := NewProfileCoverService(repo, &coverBindingsStub{}, "")
		h := NewHandler(NewService(newMemoryRepository(repo.value), testSecret, "", "", true))
		h.SetProfileCovers(coverService)
		router := gin.New()
		router.PUT("/cover", func(c *gin.Context) { SetCurrentUser(c, repo.value); h.updateCover(c) })
		req := httptest.NewRequest(http.MethodPut, "/cover", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Fatalf("body %s status %s: got %d want %d: %s", tc.body, tc.status, res.Code, tc.want, res.Body.String())
		}
	}
	h := NewHandler(NewService(newMemoryRepository(), testSecret, "", "", true))
	router := gin.New()
	h.Register(router.Group("/auth"))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/auth/me/cover", strings.NewReader(`{"assetId":""}`)))
	if res.Code != 401 {
		t.Fatalf("anonymous cover update: %d", res.Code)
	}
}
