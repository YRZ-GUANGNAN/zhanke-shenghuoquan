package im

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func performConversation(handler *Handler, path string, userID uint) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/im/conversations/:relationId", func(c *gin.Context) {
		c.Set("domainA.user", user.User{ID: userID})
		handler.conversation(c)
	})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
	return out
}

func TestConversationHandlerPeerIsAuthorizedAndMinimal(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[2] = UserMapping{UserID: 2, IMUserID: "campus_2"}
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9}}
	handler := NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil)
	var calls [][]uint
	handler.profiles = func(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
		calls = append(calls, ids)
		return map[uint]user.Profile{
			2: {ID: 2, Nickname: "买家昵称", Avatar: "/buyer.jpg", Cover: "private-cover", School: "private-school"},
			3: {ID: 3, Nickname: "卖家昵称", Avatar: "/seller.jpg", Campus: "private-campus"},
		}, nil
	}
	for _, test := range []struct {
		viewer, peer     uint
		nickname, avatar string
	}{
		{2, 3, "卖家昵称", "/seller.jpg"}, {3, 2, "买家昵称", "/buyer.jpg"},
	} {
		out := performConversation(handler, "/im/conversations/7", test.viewer)
		var body struct {
			Data struct {
				RelationID uint           `json:"relationId"`
				Peer       map[string]any `json:"peer"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"id": float64(test.peer), "nickname": test.nickname, "avatar": test.avatar}
		if out.Code != http.StatusOK || body.Data.RelationID != 7 || !reflect.DeepEqual(body.Data.Peer, want) {
			t.Fatalf("status=%d body=%s", out.Code, out.Body.String())
		}
	}
	if !reflect.DeepEqual(calls, [][]uint{{3}, {2}}) {
		t.Fatalf("profile queries=%v", calls)
	}
	for _, path := range []string{"/im/conversations/7", "/im/conversations/999"} {
		out := performConversation(handler, path, 99)
		if out.Code != http.StatusNotFound || len(calls) != 2 {
			t.Fatalf("unauthorized profile query: %v", calls)
		}
	}
	for _, lookup := range []func(context.Context, []uint) (map[uint]user.Profile, error){
		func(context.Context, []uint) (map[uint]user.Profile, error) {
			return nil, errors.New("private storage error")
		},
		func(context.Context, []uint) (map[uint]user.Profile, error) {
			return map[uint]user.Profile{3: {ID: 999}}, nil
		},
	} {
		handler.profiles = lookup
		out := performConversation(handler, "/im/conversations/7", 2)
		if out.Code != http.StatusOK || strings.Contains(out.Body.String(), `"peer"`) || strings.Contains(out.Body.String(), "private storage") {
			t.Fatalf("profile failure broke conversation: %s", out.Body.String())
		}
	}
}

func TestConversationListBatchesDistinctPeersAndPreservesPage(t *testing.T) {
	repo := newIMMemoryRepo()
	for _, id := range []uint{2, 3, 4} {
		repo.mappings[id] = UserMapping{UserID: id, IMUserID: "campus"}
	}
	repo.conversations = []Conversation{
		{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9},
		{ID: 8, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 10},
		{ID: 9, BuyerID: 4, SellerID: 2, ReferenceType: "product", ReferenceID: 11},
	}
	handler := NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil)
	calls := 0
	handler.profiles = func(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
		calls++
		if !reflect.DeepEqual(ids, []uint{3, 4}) {
			t.Fatalf("ids=%v", ids)
		}
		return map[uint]user.Profile{3: {ID: 3, Nickname: "卖家"}, 4: {ID: 4, Nickname: "账号已注销"}}, nil
	}
	router := gin.New()
	router.GET("/im/conversations", func(c *gin.Context) { c.Set("domainA.user", user.User{ID: 2}); handler.conversations(c) })
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/im/conversations?page=1&pageSize=20", nil))
	var body struct {
		Data conversationPageView `json:"data"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if out.Code != http.StatusOK || calls != 1 || body.Data.Total != 3 || body.Data.Page != 1 || body.Data.PageSize != 20 || len(body.Data.Items) != 3 {
		t.Fatalf("calls=%d body=%s", calls, out.Body.String())
	}
	for i, id := range []uint{3, 3, 4} {
		if body.Data.Items[i].Peer == nil || body.Data.Items[i].Peer.ID != id {
			t.Fatalf("item=%+v", body.Data.Items[i])
		}
	}
	if got := handler.conversationViews(context.Background(), []ConversationContext{{BuyerID: 2, SellerID: 3}}, 99); got[0].Peer != nil || calls != 1 {
		t.Fatal("non-participant queried profiles")
	}
}

func TestConversationHandlerStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9}}
	handler := NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil)
	if out := performConversation(handler, "/im/conversations/7", 2); out.Code != http.StatusOK {
		t.Fatalf("body=%s", out.Body.String())
	}
	for _, out := range []*httptest.ResponseRecorder{
		performConversation(handler, "/im/conversations/7", 99),
		performConversation(handler, "/im/conversations/999", 2),
	} {
		var body response.Body
		_ = json.Unmarshal(out.Body.Bytes(), &body)
		if out.Code != http.StatusNotFound || body.Message != "conversation not found or access denied" {
			t.Fatalf("status=%d body=%s", out.Code, out.Body.String())
		}
	}
	if out := performConversation(handler, "/im/conversations/0", 2); out.Code != http.StatusBadRequest {
		t.Fatalf("body=%s", out.Body.String())
	}
}

func TestConversationHandlerSanitizesStorageFailure(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.conversationLookupErr = errors.New("SELECT secret")
	out := performConversation(NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil), "/im/conversations/7", 2)
	if out.Code != http.StatusInternalServerError || strings.Contains(out.Body.String(), "SELECT") || !strings.Contains(out.Body.String(), "get conversation failed") {
		t.Fatalf("body=%s", out.Body.String())
	}
}

func TestConversationRouteRequiresAuthentication(t *testing.T) {
	auth := user.NewService(nil, "test-secret", "", "", false)
	router := gin.New()
	NewHandler(NewService(newIMMemoryRepo(), &importerFake{}, "1400000000", "secret"), auth).Register(router.Group("/im"))
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/im/conversations/7", nil))
	if out.Code != http.StatusUnauthorized {
		t.Fatalf("body=%s", out.Body.String())
	}
}
