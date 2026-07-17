package trade

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

func TestResolveCreateRequestID(t *testing.T) {
	key := "01J00000000000000000000000"
	for _, test := range []struct {
		name, header, body string
		want               string
		wantErr            bool
	}{
		{name: "no key keeps legacy path"},
		{name: "header", header: key, want: key},
		{name: "body", body: key, want: key},
		{name: "matching header and body", header: key, body: key, want: key},
		{name: "mismatch", header: key, body: "01J00000000000000000000001", wantErr: true},
		{name: "invalid ULID", header: "not-a-ulid", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveCreateRequestID(test.header, test.body)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("got key=%q err=%v, want key=%q wantErr=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestMediaCreateStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"keyed", "non-keyed"} {
		for _, test := range []struct {
			name string
			err  error
			want int
		}{
			{name: "invalid media", err: ErrMediaInvalid, want: http.StatusConflict},
			{name: "media conflict", err: ErrMediaConflict, want: http.StatusConflict},
			{name: "deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
			{name: "internal", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
		} {
			t.Run(path+"/"+test.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				ginContext, _ := gin.CreateTestContext(recorder)
				writeCreateError(ginContext, test.err)
				if recorder.Code != test.want {
					t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
				}
			})
		}
	}
}

func TestMediaWriteStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid media", err: ErrMediaInvalid, want: http.StatusConflict},
		{name: "media conflict", err: ErrMediaConflict, want: http.StatusConflict},
		{name: "deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "internal", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			write(ginContext, nil, test.err)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestWriteCreateResultFirstAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstRecorder := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(firstRecorder)
	writeCreateResult(firstContext, Product{ID: 7}, false)
	if firstRecorder.Code != http.StatusCreated || firstRecorder.Header().Get("Idempotency-Replayed") != "" {
		t.Fatalf("unexpected first response status=%d headers=%v", firstRecorder.Code, firstRecorder.Header())
	}

	replayRecorder := httptest.NewRecorder()
	replayContext, _ := gin.CreateTestContext(replayRecorder)
	writeCreateResult(replayContext, Product{ID: 7}, true)
	if replayRecorder.Code != http.StatusOK || replayRecorder.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("unexpected replay response status=%d headers=%v", replayRecorder.Code, replayRecorder.Header())
	}
}

type tradeSafetyCall struct {
	ctx             context.Context
	openID, content string
}

type tradeSafetyRecorder struct {
	risk  string
	calls []tradeSafetyCall
}

func (r *tradeSafetyRecorder) CheckText(ctx context.Context, openID, content string) string {
	r.calls = append(r.calls, tradeSafetyCall{ctx: ctx, openID: openID, content: content})
	return r.risk
}

type tradeReviewRecorder struct {
	submissions []review.Submission
}

func (r *tradeReviewRecorder) Submit(_ context.Context, value review.Submission) (review.Task, error) {
	r.submissions = append(r.submissions, value)
	return review.Task{ObjectType: value.ObjectType, ObjectID: value.ObjectID}, nil
}

func TestTradeHandlersUseAuthenticatedOpenID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTradeMemoryRepo(Product{ID: 9, SellerID: 7, Title: "old", Status: StatusSelling, ReviewStatus: review.StatusApproved})
	reviews := &tradeReviewRecorder{}
	checker := &tradeSafetyRecorder{risk: "manual:wechat_request_failed"}
	handler := NewHandler(NewService(repo, tradeUsers{}, reviews, &tradeConversations{}, &tradeSearchFake{}), nil, checker)
	router := gin.New()
	current := user.User{ID: 7, OpenID: "openid-trade-secret"}
	router.POST("/products", func(c *gin.Context) {
		c.Set("domainA.user", current)
		handler.create(c)
	})
	router.PUT("/products/:id", func(c *gin.Context) {
		c.Set("domainA.user", current)
		handler.update(c)
	})
	tests := []struct {
		method, path, body, content string
	}{
		{http.MethodPost, "/products", `{"title":"new","description":"create","price":100}`, "new\ncreate"},
		{http.MethodPut, "/products/9", `{"title":"updated","description":"replace","price":200}`, "updated\nreplace"},
	}
	for _, test := range tests {
		requestContext := context.WithValue(context.Background(), struct{}{}, test.path)
		req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)).WithContext(requestContext)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		beforeCalls, beforeReviews := len(checker.calls), len(reviews.submissions)
		router.ServeHTTP(response, req)
		wantStatus := http.StatusOK
		if test.method == http.MethodPost {
			wantStatus = http.StatusCreated
		}
		if response.Code != wantStatus {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		call := checker.calls[beforeCalls]
		if call.ctx != requestContext || call.openID != current.OpenID || call.content != test.content {
			t.Fatalf("%s call=%#v", test.path, call)
		}
		submission := reviews.submissions[beforeReviews]
		if submission.ObjectType != "product" || submission.SubmitterID != current.ID || submission.Risk != checker.risk {
			t.Fatalf("%s submission=%#v", test.path, submission)
		}
		if strings.Contains(response.Body.String(), current.OpenID) {
			t.Fatalf("%s response leaked openid", test.path)
		}
	}
	if repo.products[9].ReviewStatus != review.StatusSubmitted {
		t.Fatalf("update review status=%s", repo.products[9].ReviewStatus)
	}
	if _, err := repo.Find(context.Background(), 9, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("updated product became public: %v", err)
	}
}

func TestWriteMapsContactAvailabilityAndDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "contact state unavailable", err: ErrContactStateUnavailable, want: http.StatusServiceUnavailable},
		{name: "upstream deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			write(ginContext, nil, test.err)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}
