package im

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

func TestUserConversationBothDirectionsReuseStableRelation(t *testing.T) {
	repo, importer := newIMMemoryRepo(), &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")
	first, err := service.EnsureUserConversation(context.Background(), 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	stamp := repo.conversations[0].UpdatedAt
	for _, pair := range [][2]uint{{2, 8}, {8, 2}, {2, 8}} {
		value, err := service.EnsureUserConversation(context.Background(), pair[0], pair[1])
		if err != nil || value.RelationID != first.RelationID || value.ReferenceType != "user" || value.ProductID != 0 {
			t.Fatalf("value=%+v err=%v", value, err)
		}
		if value.ConversationID != "C2C"+value.PeerUserID {
			t.Fatal("invalid C2C")
		}
		if pair[0] == 2 && value.PeerUserID != "campus_8" || pair[0] == 8 && value.PeerUserID != "campus_2" {
			t.Fatal("wrong peer")
		}
	}
	if len(repo.conversations) != 1 || len(importer.users) != 2 || !repo.conversations[0].UpdatedAt.Equal(stamp) {
		t.Fatal("replay mutated relation")
	}
	if _, err := service.Conversation(context.Background(), first.RelationID, 99); !errors.Is(err, ErrConversationNotFound) {
		t.Fatal("third party authorized")
	}
	for _, pair := range [][2]uint{{0, 2}, {2, 0}, {2, 2}} {
		if _, err := service.EnsureUserConversation(context.Background(), pair[0], pair[1]); err == nil {
			t.Fatal("invalid pair accepted")
		}
	}
	broken := newIMMemoryRepo()
	if _, err := NewService(broken, &importerFake{err: errors.New("offline")}, "1", "secret").EnsureUserConversation(context.Background(), 2, 8); err == nil || len(broken.conversations) != 0 {
		t.Fatal("failed import created relation")
	}
}

func performContact(h *Handler, viewer uint, target string) *httptest.ResponseRecorder {
	router := gin.New()
	router.PUT("/im/users/:userId/conversation", func(c *gin.Context) {
		user.SetCurrentUser(c, user.User{ID: viewer, Status: user.StatusActive})
		h.contactUser(c)
	})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodPut, "/im/users/"+target+"/conversation", nil))
	return out
}

func TestDirectContactChecksPermissionBeforeCreatingOrExposing(t *testing.T) {
	featureflags.Configure(false)
	t.Cleanup(func() { featureflags.Configure(false) })
	repo, importer := newIMMemoryRepo(), &importerFake{}
	h := NewHandler(NewService(repo, importer, "1400000000", "secret"), nil)
	checked := 0
	h.contactAllowed = func(_ context.Context, viewer, peer uint) error {
		checked++
		if viewer != 2 || peer != 8 {
			t.Fatal("wrong permission pair")
		}
		return user.ErrPrivateContactUnavailable
	}
	if out := performContact(h, 2, "8"); out.Code != 403 || len(repo.conversations) != 0 || len(importer.users) != 0 {
		t.Fatal(out.Body.String())
	}
	for _, id := range []string{"0", "2", "bad"} {
		if out := performContact(h, 2, id); out.Code != 400 {
			t.Fatal(out.Body.String())
		}
	}
	if checked != 1 {
		t.Fatal("invalid input performed permission query")
	}
	h.contactAllowed = func(context.Context, uint, uint) error { return nil }
	h.profiles = func(context.Context, []uint) (map[uint]user.Profile, error) {
		return map[uint]user.Profile{8: {ID: 8, Nickname: "同学", Avatar: "/avatar.jpg"}}, nil
	}
	out := performContact(h, 2, "8")
	var body struct {
		Data conversationView `json:"data"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if out.Code != 200 || body.Data.Peer == nil || body.Data.Peer.ID != 8 || body.Data.ProductID != 0 || body.Data.ReferenceType != "user" {
		t.Fatal(out.Body.String())
	}
	h.contactAllowed = func(context.Context, uint, uint) error { return errors.New("SELECT private_table") }
	out = performContact(h, 2, "8")
	if out.Code != 503 || strings.Contains(out.Body.String(), "SELECT") {
		t.Fatal(out.Body.String())
	}
	if out := performConversation(h, "/im/conversations/1", 2); out.Code != 503 {
		t.Fatal("reopening bypassed permission")
	}
	if out := performConversation(h, "/im/conversations/1", 99); out.Code != 404 {
		t.Fatal("third party exposed")
	}
}

func TestDirectContactFeaturePolicyIndependentOfMarketplace(t *testing.T) {
	featureflags.Configure(true)
	t.Cleanup(func() { featureflags.Configure(false) })
	p := featureflags.Snapshot()
	p.Flags[featureflags.KeySecondhand] = false
	if _, err := featureflags.Update(1, featureflags.UpdateInput{Flags: p.Flags, PostingMode: p.PostingMode, ExpectedRowVersion: p.RowVersion}); err != nil {
		t.Fatal(err)
	}
	repo := newIMMemoryRepo()
	h := NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil)
	h.contactAllowed = func(context.Context, uint, uint) error { return nil }
	if out := performContact(h, 2, "8"); out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	if out := performConversation(h, "/im/conversations/1", 2); out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	repo.conversations = append(repo.conversations, Conversation{ID: 2, BuyerID: 2, SellerID: 8, ReferenceType: "product", ReferenceID: 9})
	if out := performConversation(h, "/im/conversations/2", 2); out.Code != 403 {
		t.Fatal("product gate bypass")
	}
	page, err := h.service.Conversations(context.Background(), 2, 1, 20, false)
	if err != nil || page.Total != 1 || page.Items[0].ReferenceType != "user" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	p = featureflags.Snapshot()
	p.Flags[featureflags.KeyMessages] = false
	if _, err := featureflags.Update(1, featureflags.UpdateInput{Flags: p.Flags, PostingMode: p.PostingMode, ExpectedRowVersion: p.RowVersion}); err != nil {
		t.Fatal(err)
	}
	if out := performContact(h, 2, "8"); out.Code != 403 {
		t.Fatal("disabled direct contact")
	}
}

func TestDirectContactRouteRequiresAuthentication(t *testing.T) {
	router := gin.New()
	NewHandler(NewService(newIMMemoryRepo(), &importerFake{}, "1", "secret"), user.NewService(nil, "secret", "", "", false)).Register(router.Group("/im"))
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodPut, "/im/users/8/conversation", nil))
	if out.Code != 401 {
		t.Fatal(out.Body.String())
	}
}
