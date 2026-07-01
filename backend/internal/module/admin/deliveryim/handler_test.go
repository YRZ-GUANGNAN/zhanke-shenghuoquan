package deliveryim

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type conversationQueryStub struct {
	currentErr error
	prepared   delivery.Conversation
	resolved   string
}

func (s *conversationQueryStub) Current(context.Context, uint64, string) (delivery.Conversation, error) {
	return s.prepared, s.currentErr
}
func (s *conversationQueryStub) FindForParticipant(context.Context, uint64, string) (delivery.Conversation, delivery.Order, error) {
	return s.prepared, delivery.Order{}, s.currentErr
}
func (s *conversationQueryStub) ListForParticipant(context.Context, uint64, delivery.Page) ([]delivery.Conversation, int64, error) {
	return []delivery.Conversation{s.prepared}, 1, s.currentErr
}
func (s *conversationQueryStub) PrepareImage(context.Context, uint64, string, string) (uint16, delivery.Conversation, error) {
	return 4, s.prepared, s.currentErr
}
func (s *conversationQueryStub) ResolveImage(context.Context, uint64, string, uint16) (string, error) {
	return s.resolved, s.currentErr
}

func TestHandlerParticipant404AndPrivateImagePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	codec, _ := NewImageTokenCodec("0123456789abcdef0123456789abcdef")
	conversationID := "01J00000000000000000000001"
	stub := &conversationQueryStub{prepared: delivery.Conversation{
		ConversationPublicID: conversationID, DeliveryOrderPublicID: "01J00000000000000000000002",
		AssignmentVersion: 1, TencentGroupID: "d5_" + conversationID,
		Status: delivery.ConversationActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, resolved: "https://signed.example/private?signature=redacted"}
	router := gin.New()
	NewHandler(stub, func(*gin.Context) uint64 { return 7 }, codec, true).Register(router.Group("/delivery"))

	body, _ := json.Marshal(map[string]string{"assetPublicId": "01J00000000000000000000003"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost, "/delivery/conversations/"+conversationID+"/images/prepare", bytes.NewReader(body),
	))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("prepare status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "signed.example") ||
		strings.Contains(recorder.Body.String(), "final_key") ||
		strings.Contains(recorder.Body.String(), "versionId") {
		t.Fatalf("prepare leaked storage detail: %s", recorder.Body.String())
	}

	var envelope struct {
		Data struct {
			ImageID string `json:"imageId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet,
		"/delivery/conversations/"+conversationID+"/images/"+envelope.Data.ImageID+"/resolve", nil,
	))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "expiresInSeconds") {
		t.Fatalf("resolve status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	stub.currentErr = delivery.ErrConversationForbidden
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/delivery/orders/01J00000000000000000000002/conversation", nil,
	))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("participant fence status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerProductionImagePrepareIsFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	codec, _ := NewImageTokenCodec("0123456789abcdef0123456789abcdef")
	router := gin.New()
	NewHandler(&conversationQueryStub{}, func(*gin.Context) uint64 { return 7 }, codec, false).
		Register(router.Group("/delivery"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/delivery/conversations/01J00000000000000000000001/images/prepare",
		bytes.NewBufferString(`{"assetPublicId":"01J00000000000000000000002"}`),
	))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("production image gate status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
