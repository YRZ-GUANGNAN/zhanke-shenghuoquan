package trade

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

func TestProductCommentHandlerCreateUsesAuthenticatedCoordinatesAndSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newProductCommentMemoryRepo()
	service := newProductCommentTestService(repo, Product{
		ID: 9, SellerID: 21, Status: StatusSelling, ReviewStatus: review.StatusApproved,
	})
	safety := &tradeSafetyRecorder{risk: "wechat pass"}
	handler := NewProductCommentHandler(service, nil, safety)
	router := productCommentTestRouter(handler, user.User{ID: 31, OpenID: "openid-comment-secret"})

	recorder := performProductCommentRequest(
		router,
		http.MethodPost,
		"/products/9/comments",
		`{"content":"  留言内容  ","clientRequestId":"`+productCommentTestRequestID+`","productId":99,"parentCommentId":88,"replyToUserId":77,"isSeller":true}`,
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastCreated.ProductID != 9 || repo.lastCreated.ParentCommentID != 0 ||
		repo.lastCreated.ReplyToUserID != 0 || repo.lastCreated.UserID != 31 {
		t.Fatalf("client coordinates leaked into create: %#v", repo.lastCreated)
	}
	if len(safety.calls) != 1 || safety.calls[0].openID != "openid-comment-secret" ||
		safety.calls[0].content != "  留言内容  " {
		t.Fatalf("unexpected safety call: %#v", safety.calls)
	}
	if strings.Contains(recorder.Body.String(), "openid-comment-secret") {
		t.Fatalf("response leaked authenticated openid: %s", recorder.Body.String())
	}
}

func TestProductCommentHandlerReplyInfersURLAndDatabaseCoordinates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := ProductComment{
		ID: 10, ProductID: 9, UserID: 41, ReviewStatus: review.StatusApproved,
	}
	repo := newProductCommentMemoryRepo(root)
	service := newProductCommentTestService(repo, Product{
		ID: 9, SellerID: 21, Status: StatusReserved, ReviewStatus: review.StatusApproved,
	})
	handler := NewProductCommentHandler(service, nil, &tradeSafetyRecorder{risk: "wechat pass"})
	router := productCommentTestRouter(handler, user.User{ID: 31, OpenID: "openid-reply"})

	recorder := performProductCommentRequest(
		router,
		http.MethodPost,
		"/products/9/comments/10/replies",
		`{"content":"回复内容","clientRequestId":"`+productCommentTestRequestID+`","productId":99,"parentCommentId":88,"replyToUserId":77,"isSeller":true}`,
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastCreated.ProductID != 9 || repo.lastCreated.ParentCommentID != root.ID ||
		repo.lastCreated.ReplyToUserID != root.UserID || repo.lastCreated.UserID != 31 {
		t.Fatalf("reply coordinates were not server-derived: %#v", repo.lastCreated)
	}
}

func TestProductCommentHandlerReplayReturnsHeaderAndOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newProductCommentMemoryRepo()
	service := newProductCommentTestService(repo, Product{
		ID: 9, Status: StatusSelling, ReviewStatus: review.StatusApproved,
	})
	handler := NewProductCommentHandler(service, nil, &tradeSafetyRecorder{risk: "wechat pass"})
	router := productCommentTestRouter(handler, user.User{ID: 31, OpenID: "openid-replay"})
	body := `{"content":"same payload","clientRequestId":"` + productCommentTestRequestID + `"}`

	first := performProductCommentRequest(router, http.MethodPost, "/products/9/comments", body)
	second := performProductCommentRequest(router, http.MethodPost, "/products/9/comments", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusOK || second.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay status=%d header=%q body=%s",
			second.Code, second.Header().Get("Idempotency-Replayed"), second.Body.String())
	}
}

func TestProductCommentHandlerListsAuthenticatedVisibleComments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newProductCommentMemoryRepo(ProductComment{
		ID: 10, ProductID: 9, UserID: 31, Content: "visible", ReviewStatus: review.StatusSubmitted,
	})
	service := newProductCommentTestService(repo, Product{
		ID: 9, Status: StatusSold, ReviewStatus: review.StatusApproved,
	})
	handler := NewProductCommentHandler(service, nil, &tradeSafetyRecorder{})
	router := productCommentTestRouter(handler, user.User{ID: 31, OpenID: "openid-list"})

	recorder := performProductCommentRequest(router, http.MethodGet, "/products/9/comments?page=1&pageSize=20", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data ProductCommentPage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Total != 1 || len(body.Data.Items) != 1 || body.Data.Items[0].Content != "visible" {
		t.Fatalf("unexpected page: %#v", body.Data)
	}
}

func TestProductCommentHandlerStrictStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid", err: ErrInvalidProductComment, want: http.StatusBadRequest},
		{name: "invalid write contract", err: ErrWriteContractInvalid, want: http.StatusBadRequest},
		{name: "not found", err: ErrNotFound, want: http.StatusNotFound},
		{name: "unavailable product", err: ErrConflict, want: http.StatusConflict},
		{name: "idempotency conflict", err: ErrIdempotencyConflict, want: http.StatusConflict},
		{name: "duplicate content", err: ErrProductCommentDuplicate, want: http.StatusConflict},
		{name: "rate limited", err: ErrProductCommentRateLimited, want: http.StatusTooManyRequests},
		{name: "write unavailable", err: ErrWriteContractUnavailable, want: http.StatusServiceUnavailable},
		{name: "internal", err: errors.New("database failed"), want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			writeProductCommentError(context, test.err)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestProductCommentHandlerRejectsUnauthenticatedAndInvalidCoordinates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newProductCommentMemoryRepo()
	service := newProductCommentTestService(repo, Product{
		ID: 9, Status: StatusSelling, ReviewStatus: review.StatusApproved,
	})
	handler := NewProductCommentHandler(service, nil, &tradeSafetyRecorder{})

	unauthenticated := productCommentTestRouter(handler, user.User{})
	response := performProductCommentRequest(unauthenticated, http.MethodGet, "/products/9/comments", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d body=%s", response.Code, response.Body.String())
	}

	authenticated := productCommentTestRouter(handler, user.User{ID: 31, OpenID: "openid-valid"})
	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/products/0/comments", `{"content":"hello","clientRequestId":"` + productCommentTestRequestID + `"}`},
		{http.MethodPost, "/products/9/comments/not-a-number/replies", `{"content":"hello","clientRequestId":"` + productCommentTestRequestID + `"}`},
		{http.MethodPost, "/products/9/comments", `{"content":"hello"}`},
		{http.MethodPost, "/products/9/comments", `{"content":"hello","clientRequestId":"invalid"}`},
	} {
		response = performProductCommentRequest(authenticated, request.method, request.path, request.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, response.Code, response.Body.String())
		}
	}
}

func productCommentTestRouter(handler *ProductCommentHandler, current user.User) *gin.Engine {
	router := gin.New()
	authenticate := func(c *gin.Context) {
		if current.ID != 0 {
			user.SetCurrentUser(c, current)
		}
		c.Next()
	}
	router.GET("/products/:id/comments", authenticate, handler.list)
	router.POST("/products/:id/comments", authenticate, handler.create)
	router.POST("/products/:id/comments/:commentId/replies", authenticate, handler.reply)
	return router
}

func performProductCommentRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}
