package im

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
