package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

type communitySafetyCall struct {
	ctx             context.Context
	openID, content string
}

type communitySafetyRecorder struct {
	risk  string
	calls []communitySafetyCall
}

func (r *communitySafetyRecorder) CheckText(ctx context.Context, openID, content string) string {
	r.calls = append(r.calls, communitySafetyCall{ctx: ctx, openID: openID, content: content})
	return r.risk
}

func TestCommunityHandlersUseAuthenticatedOpenID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &communityMemoryRepo{posts: map[uint]Post{9: {ID: 9, UserID: 1, ReviewStatus: review.StatusApproved}}}
	reviews := &submissionRecorder{}
	checker := &communitySafetyRecorder{risk: "manual:wechat_request_failed"}
	handler := NewHandler(NewService(repo, profileDirectory{}, reviews), nil, checker)
	router := gin.New()
	current := user.User{ID: 7, OpenID: "openid-community-secret"}
	router.POST("/posts", func(c *gin.Context) {
		c.Set("domainA.user", current)
		handler.create(c)
	})
	router.POST("/posts/:id/comments", func(c *gin.Context) {
		c.Set("domainA.user", current)
		handler.addComment(c)
	})
	tests := []struct {
		path, body, content, objectType string
	}{
		{"/posts", `{"title":"title","content":"content"}`, "title\ncontent", "post"},
		{"/posts/9/comments", `{"content":"comment"}`, "comment", "comment"},
	}
	for _, test := range tests {
		requestContext := context.WithValue(context.Background(), struct{}{}, test.objectType)
		req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)).WithContext(requestContext)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		beforeCalls, beforeReviews := len(checker.calls), len(reviews.submissions)
		router.ServeHTTP(response, req)
		if response.Code != http.StatusCreated {
			t.Fatalf("%s status=%d body=%s", test.objectType, response.Code, response.Body.String())
		}
		call := checker.calls[beforeCalls]
		if call.ctx != requestContext || call.openID != current.OpenID || call.content != test.content {
			t.Fatalf("%s safety call=%#v", test.objectType, call)
		}
		submission := reviews.submissions[beforeReviews]
		if submission.ObjectType != test.objectType || submission.SubmitterID != current.ID || submission.Risk != checker.risk {
			t.Fatalf("%s submission=%#v", test.objectType, submission)
		}
		if strings.Contains(response.Body.String(), current.OpenID) {
			t.Fatalf("%s response leaked openid", test.objectType)
		}
	}
}
