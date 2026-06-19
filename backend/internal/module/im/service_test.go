package im

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type imMemoryRepo struct {
	mappings      map[uint]UserMapping
	conversations []Conversation
}

func newIMMemoryRepo() *imMemoryRepo { return &imMemoryRepo{mappings: map[uint]UserMapping{}} }
func (r *imMemoryRepo) EnsureMapping(_ context.Context, v *UserMapping) error {
	v.ID = v.UserID
	r.mappings[v.UserID] = *v
	return nil
}
func (r *imMemoryRepo) FindMapping(_ context.Context, id uint) (UserMapping, error) {
	v, ok := r.mappings[id]
	if !ok {
		return UserMapping{}, errors.New("not found")
	}
	return v, nil
}
func (r *imMemoryRepo) EnsureConversation(_ context.Context, v *Conversation) error {
	for _, current := range r.conversations {
		if current.BuyerID == v.BuyerID && current.SellerID == v.SellerID && current.ReferenceID == v.ReferenceID {
			*v = current
			return nil
		}
	}
	v.ID = uint(len(r.conversations) + 1)
	r.conversations = append(r.conversations, *v)
	return nil
}
func (r *imMemoryRepo) ListConversations(_ context.Context, userID uint, _, _ int) ([]Conversation, int64, error) {
	values := []Conversation{}
	for _, v := range r.conversations {
		if v.BuyerID == userID || v.SellerID == userID {
			values = append(values, v)
		}
	}
	return values, int64(len(values)), nil
}

type importerFake struct {
	users []string
	err   error
}

func (i *importerFake) ImportAccount(_ context.Context, id string) error {
	i.users = append(i.users, id)
	return i.err
}

func TestRealConversationContextUsesImportedTencentAccounts(t *testing.T) {
	repo := newIMMemoryRepo()
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")
	value, err := service.EnsureProductConversation(context.Background(), 9, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(importer.users) != 2 || importer.users[0] != "campus_2" || importer.users[1] != "campus_3" {
		t.Fatalf("accounts not imported: %v", importer.users)
	}
	if value.ConversationID != "C2Ccampus_3" || value.PeerUserID != "campus_3" {
		t.Fatalf("not a real C2C context: %+v", value)
	}
	sellerPage, err := service.Conversations(context.Background(), 3, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if sellerPage.Items[0].ConversationID != "C2Ccampus_2" {
		t.Fatalf("seller peer mapping wrong: %+v", sellerPage.Items[0])
	}
}

func TestImportFailureDoesNotCreatePseudoConversation(t *testing.T) {
	repo := newIMMemoryRepo()
	service := NewService(repo, &importerFake{err: errors.New("Tencent unavailable")}, "1400000000", "secret")
	if _, err := service.EnsureProductConversation(context.Background(), 9, 2, 3); err == nil {
		t.Fatal("import failure ignored")
	}
	if len(repo.conversations) != 0 {
		t.Fatal("local pseudo conversation was persisted")
	}
}

func TestCredentialsImportAccountAndUseStableIdentity(t *testing.T) {
	repo := newIMMemoryRepo()
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")
	value, err := service.Credentials(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if value.UserID != "campus_42" || value.UserSig == "" || len(importer.users) != 1 {
		t.Fatalf("invalid credentials: %+v imports=%v", value, importer.users)
	}
}

func TestTencentAccountImportRESTRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/im_open_login_svc/account_import" || r.URL.Query().Get("identifier") != "administrator" || r.URL.Query().Get("usersig") == "" {
			t.Errorf("invalid Tencent request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["Identifier"] != "campus_7" {
			t.Errorf("invalid body: %v %v", body, err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ActionStatus": "OK", "ErrorCode": 0, "ErrorInfo": ""})
	}))
	defer server.Close()
	client, err := NewTencentRESTClient("1400000000", "secret", "administrator", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ImportAccount(context.Background(), "campus_7"); err != nil {
		t.Fatal(err)
	}
}
