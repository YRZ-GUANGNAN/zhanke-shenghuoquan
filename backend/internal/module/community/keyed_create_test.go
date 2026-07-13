package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

const communityTestRequestID = "01J00000000000000000000000"

type communityKeyedMemoryRepo struct {
	*communityMemoryRepo
	mu             sync.Mutex
	postsByRequest map[string]Post
	commentsByKey  map[string]Comment
	postEvent      WriteOutbox
	commentEvent   WriteOutbox
}

func newCommunityKeyedMemoryRepo() *communityKeyedMemoryRepo {
	return &communityKeyedMemoryRepo{
		communityMemoryRepo: &communityMemoryRepo{posts: map[uint]Post{}},
		postsByRequest:      map[string]Post{},
		commentsByKey:       map[string]Comment{},
	}
}

func communityRequestCoordinate(ownerID uint, requestID string) string {
	return fmt.Sprintf("%d:%s", ownerID, requestID)
}

func (r *communityKeyedMemoryRepo) FindPostByRequest(_ context.Context, ownerID uint, requestID string) (Post, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, found := r.postsByRequest[communityRequestCoordinate(ownerID, requestID)]
	return value, found, nil
}

func (r *communityKeyedMemoryRepo) FindCommentByRequest(_ context.Context, ownerID uint, requestID string) (Comment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, found := r.commentsByKey[communityRequestCoordinate(ownerID, requestID)]
	return value, found, nil
}

func (r *communityKeyedMemoryRepo) CreatePostOrReplay(_ context.Context, value *Post, event *WriteOutbox) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	coordinate := communityRequestCoordinate(value.UserID, *value.ClientRequestID)
	if existing, found := r.postsByRequest[coordinate]; found {
		if !bytes.Equal(existing.CreateRequestHash, value.CreateRequestHash) {
			return false, ErrIdempotencyConflict
		}
		*value = existing
		return true, nil
	}
	value.ID = uint(len(r.posts) + 1)
	r.posts[value.ID] = *value
	r.postsByRequest[coordinate] = *value
	r.postEvent = *event
	return false, nil
}

func (r *communityKeyedMemoryRepo) CreateCommentOrReplay(_ context.Context, value *Comment, event *WriteOutbox) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	coordinate := communityRequestCoordinate(value.UserID, *value.ClientRequestID)
	if existing, found := r.commentsByKey[coordinate]; found {
		if !bytes.Equal(existing.CreateRequestHash, value.CreateRequestHash) {
			return false, ErrIdempotencyConflict
		}
		*value = existing
		return true, nil
	}
	value.ID = uint(len(r.comments) + 1)
	r.comments = append(r.comments, *value)
	r.commentsByKey[coordinate] = *value
	r.commentEvent = *event
	return false, nil
}

func TestCommunityKeyedCreateUsesCanonicalHashAndStableOutboxPayload(t *testing.T) {
	repo := newCommunityKeyedMemoryRepo()
	reviews := &submissionRecorder{}
	mediaGateway := &communityMediaFake{}
	service := NewService(repo, profileDirectory{}, reviews, mediaGateway)
	input := CreatePostInput{
		UserID: 7, Title: "title", Content: "content", Topic: "topic",
		ImageAssetIDs: []string{"asset-b", "asset-a"},
	}

	first, err := service.CreatePostKeyed(context.Background(), input, communityTestRequestID, "risk snapshot")
	if err != nil || first.Replayed || first.Value.ID == 0 {
		t.Fatalf("first keyed create failed: result=%+v err=%v", first, err)
	}
	if mediaGateway.committed {
		t.Fatal("keyed create bypassed creation outbox and committed media inline")
	}
	if len(reviews.submissions) != 0 {
		t.Fatalf("keyed create submitted review inline: %+v", reviews.submissions)
	}
	if repo.postEvent.ReservationToken == nil || *repo.postEvent.ReservationToken != "token" {
		t.Fatalf("reservation token missing from outbox: %+v", repo.postEvent)
	}
	var payload communityCreationPayload
	if err := json.Unmarshal([]byte(repo.postEvent.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != "community-post-created:v1" || payload.ReferenceType != "post" ||
		payload.ReferenceIDSource != "aggregateId" || payload.ReviewTargetType != "post" || payload.ReviewRisk != "risk snapshot" ||
		len(payload.OrderedAssetPublicIDs) != 2 || payload.OrderedAssetPublicIDs[0] != "asset-b" || payload.OrderedAssetPublicIDs[1] != "asset-a" {
		t.Fatalf("unstable outbox payload: %+v", payload)
	}

	replay, err := service.CreatePostKeyed(context.Background(), input, communityTestRequestID, "different transient risk")
	if err != nil || !replay.Replayed || replay.Value.ID != first.Value.ID {
		t.Fatalf("same request did not replay: result=%+v err=%v", replay, err)
	}
	changed := input
	changed.ImageAssetIDs = []string{"asset-a", "asset-b"}
	if _, err := service.CreatePostKeyed(context.Background(), changed, communityTestRequestID, "risk snapshot"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("ordered asset change did not conflict: %v", err)
	}
}

func TestCommunityKeyedCommentCreatesOneOutboxAndReplays(t *testing.T) {
	repo := newCommunityKeyedMemoryRepo()
	repo.posts[9] = Post{ID: 9, UserID: 1, ReviewStatus: review.StatusApproved}
	service := NewService(repo, profileDirectory{}, &submissionRecorder{})

	first, err := service.AddCommentKeyed(context.Background(), 9, 7, "hello", communityTestRequestID, "comment risk")
	if err != nil || first.Replayed || first.Value.ID == 0 {
		t.Fatalf("first keyed comment failed: result=%+v err=%v", first, err)
	}
	replay, err := service.AddCommentKeyed(context.Background(), 9, 7, "hello", communityTestRequestID, "changed risk")
	if err != nil || !replay.Replayed || replay.Value.ID != first.Value.ID {
		t.Fatalf("comment replay failed: result=%+v err=%v", replay, err)
	}
	if _, err := service.AddCommentKeyed(context.Background(), 9, 7, "different", communityTestRequestID, "comment risk"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different comment payload did not conflict: %v", err)
	}
	var payload communityCreationPayload
	if err := json.Unmarshal([]byte(repo.commentEvent.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ReviewTargetType != "comment" || payload.ReviewRisk != "comment risk" || payload.ReferenceIDSource != "aggregateId" || len(payload.OrderedAssetPublicIDs) != 0 {
		t.Fatalf("unexpected comment outbox payload: %+v", payload)
	}
}

func TestCommunityIdempotencyKeyResolution(t *testing.T) {
	for _, test := range []struct {
		name, header, body string
		key                string
		keyed              bool
		wantErr            bool
	}{
		{name: "none"},
		{name: "header", header: communityTestRequestID, key: communityTestRequestID, keyed: true},
		{name: "body", body: communityTestRequestID, key: communityTestRequestID, keyed: true},
		{name: "both", header: communityTestRequestID, body: communityTestRequestID, key: communityTestRequestID, keyed: true},
		{name: "mismatch", header: communityTestRequestID, body: "01J00000000000000000000001", wantErr: true},
		{name: "invalid", header: "not-ulid", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			key, keyed, err := resolveCommunityIdempotencyKey(test.header, test.body)
			if (err != nil) != test.wantErr || key != test.key || keyed != test.keyed {
				t.Fatalf("key=%q keyed=%v err=%v", key, keyed, err)
			}
		})
	}
}

func TestCommunityCreateHandlerKeyedHTTPStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newCommunityKeyedMemoryRepo()
	service := NewService(repo, profileDirectory{}, &submissionRecorder{})
	handler := NewHandler(service, nil, review.NewSafetyClient(config.Config{}))
	router := gin.New()
	router.POST("/posts", func(c *gin.Context) {
		c.Set("domainA.user", user.User{ID: 7})
		handler.create(c)
	})

	perform := func(body, header string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if header != "" {
			request.Header.Set("Idempotency-Key", header)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	first := perform(`{"title":"title","content":"content","clientRequestId":"`+communityTestRequestID+`"}`, "")
	if first.Code != http.StatusCreated || first.Header().Get("Idempotency-Replayed") != "" {
		t.Fatalf("unexpected first response: status=%d headers=%v body=%s", first.Code, first.Header(), first.Body.String())
	}
	replay := perform(`{"title":"title","content":"content"}`, communityTestRequestID)
	if replay.Code != http.StatusOK || replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("unexpected replay response: status=%d headers=%v body=%s", replay.Code, replay.Header(), replay.Body.String())
	}
	conflict := perform(`{"title":"changed","content":"content"}`, communityTestRequestID)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("unexpected conflict response: status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	mismatch := perform(`{"title":"title","content":"content","clientRequestId":"01J00000000000000000000001"}`, communityTestRequestID)
	if mismatch.Code != http.StatusBadRequest {
		t.Fatalf("unexpected mismatch response: status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
}

func TestCommunityCommentHandlerKeyedHTTPStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newCommunityKeyedMemoryRepo()
	repo.posts[9] = Post{ID: 9, UserID: 1, ReviewStatus: review.StatusApproved}
	service := NewService(repo, profileDirectory{}, &submissionRecorder{})
	handler := NewHandler(service, nil, review.NewSafetyClient(config.Config{}))
	router := gin.New()
	router.POST("/posts/:id/comments", func(c *gin.Context) {
		c.Set("domainA.user", user.User{ID: 7})
		handler.addComment(c)
	})

	perform := func(body, header string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/posts/9/comments", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if header != "" {
			request.Header.Set("Idempotency-Key", header)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	first := perform(`{"content":"hello"}`, communityTestRequestID)
	if first.Code != http.StatusCreated {
		t.Fatalf("unexpected first comment response: status=%d body=%s", first.Code, first.Body.String())
	}
	replay := perform(`{"content":"hello","clientRequestId":"`+communityTestRequestID+`"}`, "")
	if replay.Code != http.StatusOK || replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("unexpected comment replay response: status=%d headers=%v body=%s", replay.Code, replay.Header(), replay.Body.String())
	}
	conflict := perform(`{"content":"different"}`, communityTestRequestID)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("unexpected comment conflict response: status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}
