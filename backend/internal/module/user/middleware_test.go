package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIdentityRequiredRejectsUnapprovedOrInactiveIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		user     User
		merchant IdentityEligibilityResult
		want     int
	}{
		{
			name:     "merchant approval missing",
			user:     User{ID: 1, CurrentIdentity: IdentityStudent, Status: StatusActive},
			merchant: IdentityEligibilityResult{Available: false, Status: "missing"},
			want:     http.StatusForbidden,
		},
		{
			name:     "approved but student identity is active",
			user:     User{ID: 1, CurrentIdentity: IdentityStudent, Status: StatusActive},
			merchant: IdentityEligibilityResult{Available: true, Status: "active"},
			want:     http.StatusForbidden,
		},
		{
			name:     "approved merchant identity is active",
			user:     User{ID: 1, CurrentIdentity: IdentityMerchant, Status: StatusActive},
			merchant: IdentityEligibilityResult{Available: true, Status: "active"},
			want:     http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryRepository(tt.user)
			service := NewService(repo, testSecret, "", "", true, WithIdentityEligibility(identityEligibilityStub{
				merchant: tt.merchant,
			}))
			router := gin.New()
			router.GET(
				"/merchant",
				func(c *gin.Context) {
					c.Set(contextUser, repo.users[1])
					c.Next()
				},
				service.IdentityRequired(IdentityMerchant),
				func(c *gin.Context) {
					c.Status(http.StatusOK)
				},
			)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/merchant", nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.want {
				t.Fatalf("status=%d body=%s want=%d", recorder.Code, recorder.Body.String(), tt.want)
			}
		})
	}
}
