package im

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type imMemoryRepo struct {
	mappings              map[uint]UserMapping
	conversations         []Conversation
	conversationLookupErr error
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
func (r *imMemoryRepo) FindConversation(_ context.Context, buyerID, sellerID uint, referenceType string, referenceID uint) (Conversation, bool, error) {
	for _, current := range r.conversations {
		if current.BuyerID == buyerID && current.SellerID == sellerID && current.ReferenceType == referenceType && current.ReferenceID == referenceID {
			return current, true, nil
		}
	}
	return Conversation{}, false, nil
}
func (r *imMemoryRepo) FindConversationForParticipant(_ context.Context, relationID, userID uint) (Conversation, bool, error) {
	if r.conversationLookupErr != nil {
		return Conversation{}, false, r.conversationLookupErr
	}
	for _, value := range r.conversations {
		if value.ID == relationID && value.ReferenceType == "product" && (value.BuyerID == userID || value.SellerID == userID) {
			return value, true, nil
		}
	}
	return Conversation{}, false, nil
}
func (r *imMemoryRepo) EnsureConversationStable(ctx context.Context, v *Conversation) error {
	if current, found, err := r.FindConversation(ctx, v.BuyerID, v.SellerID, v.ReferenceType, v.ReferenceID); err != nil {
		return err
	} else if found {
		*v = current
		return nil
	}
	now := time.Now().UTC()
	v.ID = uint(len(r.conversations) + 1)
	v.CreatedAt = now
	v.UpdatedAt = now
	r.conversations = append(r.conversations, *v)
	return nil
}
func (r *imMemoryRepo) ListConversations(_ context.Context, userID uint, _, _ int) ([]Conversation, int64, error) {
	values := []Conversation{}
	for _, v := range r.conversations {
		if v.ReferenceType == "product" && (v.BuyerID == userID || v.SellerID == userID) {
			values = append(values, v)
		}
	}
	return values, int64(len(values)), nil
}

func TestConversationForParticipantUsesCallerSpecificPeer(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[2] = UserMapping{UserID: 2, IMUserID: "campus_2"}
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, ConversationID: "stale", BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9}}
	service := NewService(repo, &importerFake{}, "1400000000", "secret")
	buyer, err := service.Conversation(context.Background(), 7, 2)
	if err != nil || buyer.PeerUserID != "campus_3" || buyer.ConversationID != "C2Ccampus_3" {
		t.Fatalf("buyer=%+v err=%v", buyer, err)
	}
	seller, err := service.Conversation(context.Background(), 7, 3)
	if err != nil || seller.PeerUserID != "campus_2" || seller.ConversationID != "C2Ccampus_2" {
		t.Fatalf("seller=%+v err=%v", seller, err)
	}
}

func TestConversationForParticipantHidesMissingUnauthorizedAndNonProduct(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.conversations = []Conversation{
		{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9},
		{ID: 8, BuyerID: 2, SellerID: 3, ReferenceType: "delivery", ReferenceID: 10},
	}
	service := NewService(repo, &importerFake{}, "1400000000", "secret")
	for _, input := range [][2]uint{{7, 99}, {999, 2}, {8, 2}} {
		if _, err := service.Conversation(context.Background(), input[0], input[1]); !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("input=%v err=%v", input, err)
		}
	}
}

func TestConversationForParticipantIsReadOnly(t *testing.T) {
	stamp := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9, CreatedAt: stamp, UpdatedAt: stamp}}
	before := append([]Conversation(nil), repo.conversations...)
	importer := &importerFake{}
	if _, err := NewService(repo, importer, "1400000000", "secret").Conversation(context.Background(), 7, 2); err != nil {
		t.Fatal(err)
	}
	if len(importer.users) != 0 || !reflect.DeepEqual(before, repo.conversations) {
		t.Fatalf("imports=%v before=%+v after=%+v", importer.users, before, repo.conversations)
	}
}

func TestConversationsOnlyReturnsProductRelations(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{
		{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9},
		{ID: 8, BuyerID: 2, SellerID: 3, ReferenceType: "delivery", ReferenceID: 10},
	}
	page, err := NewService(repo, &importerFake{}, "1400000000", "secret").Conversations(context.Background(), 2, 1, 20)
	if err != nil || len(page.Items) != 1 || page.Items[0].RelationID != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
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

func TestFindProductConversationIsReadOnly(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{ID: 3, UserID: 3, IMUserID: "campus_3"}
	stamp := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	repo.conversations = append(repo.conversations, Conversation{
		ID: 7, ConversationID: "C2Ccampus_3", BuyerID: 2, SellerID: 3,
		ReferenceType: "product", ReferenceID: 9, CreatedAt: stamp, UpdatedAt: stamp,
	})
	before := append([]Conversation(nil), repo.conversations...)
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")

	value, found, err := service.FindProductConversation(context.Background(), 9, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !found || value.RelationID != 7 || value.ConversationID != "C2Ccampus_3" {
		t.Fatalf("unexpected stable lookup: found=%v value=%+v", found, value)
	}
	if len(importer.users) != 0 {
		t.Fatalf("read-only lookup imported accounts: %v", importer.users)
	}
	if !reflect.DeepEqual(before, repo.conversations) {
		t.Fatalf("read-only lookup mutated relations: before=%+v after=%+v", before, repo.conversations)
	}
}

func TestFindProductConversationMissingDoesNotWrite(t *testing.T) {
	repo := newIMMemoryRepo()
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")

	value, found, err := service.FindProductConversation(context.Background(), 9, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if found || value != (ConversationContext{}) {
		t.Fatalf("missing relation reported as found: found=%v value=%+v", found, value)
	}
	if len(importer.users) != 0 || len(repo.mappings) != 0 || len(repo.conversations) != 0 {
		t.Fatalf("missing lookup produced writes: imports=%v mappings=%v conversations=%v", importer.users, repo.mappings, repo.conversations)
	}
}

func TestEnsureProductConversationStableReplayDoesNotRefreshRelation(t *testing.T) {
	repo := newIMMemoryRepo()
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")

	first, err := service.EnsureProductConversationStable(context.Background(), 9, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	canonical := repo.conversations[0]
	time.Sleep(time.Millisecond)
	second, err := service.EnsureProductConversationStable(context.Background(), 9, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(repo.conversations) != 1 {
		t.Fatalf("replay changed canonical relation: first=%+v second=%+v rows=%+v", first, second, repo.conversations)
	}
	if len(importer.users) != 2 {
		t.Fatalf("replay imported Tencent accounts again: %v", importer.users)
	}
	if repo.conversations[0].CreatedAt != canonical.CreatedAt || repo.conversations[0].UpdatedAt != canonical.UpdatedAt {
		t.Fatalf("replay refreshed relation timestamps: before=%+v after=%+v", canonical, repo.conversations[0])
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
