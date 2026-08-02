package user

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"campus-circle/backend/internal/pkg/media"
	"github.com/gin-gonic/gin"
)

type avatarMediaGatewayStub struct {
	reservation          media.Reservation
	commitErr            error
	reserveCalls         int
	commitCalls          int
	cancelCalls          int
	deleteReferenceCalls int
}

func (s *avatarMediaGatewayStub) Reserve(_ context.Context, userID uint, purpose media.Purpose, assetIDs []string) (media.Reservation, error) {
	s.reserveCalls++
	if userID == 0 || purpose != media.PurposeAvatar || len(assetIDs) != 1 || assetIDs[0] == "" {
		return media.Reservation{}, errors.New("invalid avatar reservation")
	}
	return s.reservation, nil
}

func (s *avatarMediaGatewayStub) Commit(_ context.Context, _ uint, _ media.Reservation, ownerType string, ownerID uint64) error {
	s.commitCalls++
	if ownerType != "user" || ownerID == 0 {
		return errors.New("invalid avatar owner")
	}
	return s.commitErr
}

func (s *avatarMediaGatewayStub) Cancel(_ context.Context, _ uint, _ media.Reservation) error {
	s.cancelCalls++
	return nil
}

func (s *avatarMediaGatewayStub) DeleteReference(_ context.Context, ownerType string, ownerID uint64, keep []uint64) error {
	s.deleteReferenceCalls++
	if ownerType != "user" || ownerID == 0 || len(keep) != 1 || keep[0] == 0 {
		return errors.New("invalid avatar reference cleanup")
	}
	return nil
}

func performAvatarUpdate(t *testing.T, current User, gateway media.Gateway) *httptest.ResponseRecorder {
	t.Helper()
	repo := newMemoryRepository(current)
	handler := NewHandler(NewService(repo, testSecret, "", "", true), gateway)
	router := gin.New()
	router.PATCH("/me", func(c *gin.Context) {
		SetCurrentUser(c, current)
		handler.updateMe(c)
	})

	request := httptest.NewRequest(http.MethodPatch, "/me", bytes.NewBufferString(`{
		"nickname":"新昵称",
		"school":"湛江科技学院",
		"campus":"新湖校区",
		"avatarAssetId":"01AVATARASSET000000000000"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestUpdateMeCommitsReviewedAvatarAndReturnsFormalProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	current := User{ID: 7, Nickname: "旧昵称", Avatar: "https://old.example/avatar.jpg", Status: StatusActive}
	gateway := &avatarMediaGatewayStub{reservation: media.Reservation{
		Token:      "avatar-reservation",
		References: []media.Reference{{InternalID: 91, PublicID: "01AVATARASSET000000000000", URL: "https://media.example/avatar.jpg"}},
	}}

	response := performAvatarUpdate(t, current, gateway)
	if response.Code != http.StatusOK {
		t.Fatalf("update avatar status = %d, body = %s", response.Code, response.Body.String())
	}
	if gateway.reserveCalls != 1 || gateway.commitCalls != 1 || gateway.cancelCalls != 0 || gateway.deleteReferenceCalls != 1 {
		t.Fatalf("unexpected media calls: reserve=%d commit=%d cancel=%d cleanup=%d", gateway.reserveCalls, gateway.commitCalls, gateway.cancelCalls, gateway.deleteReferenceCalls)
	}
	if body := response.Body.String(); !bytes.Contains([]byte(body), []byte("https://media.example/avatar.jpg")) {
		t.Fatalf("response does not contain formal avatar URL: %s", body)
	}
}

func TestUpdateMeRestoresPreviousAvatarWhenMediaCommitFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldAssetID := uint64(17)
	current := User{
		ID: 9, Nickname: "原昵称", Avatar: "https://old.example/avatar.jpg", AvatarAssetID: &oldAssetID,
		School: "原学校", Campus: "原校区", Status: StatusActive,
	}
	gateway := &avatarMediaGatewayStub{
		reservation: media.Reservation{
			Token:      "avatar-reservation",
			References: []media.Reference{{InternalID: 92, PublicID: "01AVATARASSET000000000000", URL: "https://media.example/new-avatar.jpg"}},
		},
		commitErr: errors.New("commit failed"),
	}

	repo := newMemoryRepository(current)
	handler := NewHandler(NewService(repo, testSecret, "", "", true), gateway)
	router := gin.New()
	router.PATCH("/me", func(c *gin.Context) {
		SetCurrentUser(c, current)
		handler.updateMe(c)
	})
	request := httptest.NewRequest(http.MethodPatch, "/me", bytes.NewBufferString(`{
		"nickname":"不应保留的新昵称",
		"school":"新学校",
		"campus":"新校区",
		"avatarAssetId":"01AVATARASSET000000000000"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("commit failure status = %d, body = %s", response.Code, response.Body.String())
	}
	stored, err := repo.FindByID(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("load restored user: %v", err)
	}
	if stored.Nickname != current.Nickname || stored.Avatar != current.Avatar || stored.School != current.School || stored.Campus != current.Campus {
		t.Fatalf("profile was not restored: %#v", stored.Profile())
	}
	if stored.AvatarAssetID == nil || *stored.AvatarAssetID != oldAssetID {
		t.Fatalf("avatar asset was not restored: %#v", stored.AvatarAssetID)
	}
	if gateway.reserveCalls != 1 || gateway.commitCalls != 1 || gateway.cancelCalls != 1 || gateway.deleteReferenceCalls != 0 {
		t.Fatalf("unexpected media calls: reserve=%d commit=%d cancel=%d cleanup=%d", gateway.reserveCalls, gateway.commitCalls, gateway.cancelCalls, gateway.deleteReferenceCalls)
	}
}
