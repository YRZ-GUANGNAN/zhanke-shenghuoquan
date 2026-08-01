package admin

import (
	"net/http"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

func TestReviewDecisionStrictRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "malformed json", path: "/api/v1/admin/reviews/1/approve", body: `{"reason":`},
		{name: "unknown field", path: "/api/v1/admin/reviews/1/approve", body: `{"reason":"","extra":true}`},
		{name: "trailing json", path: "/api/v1/admin/reviews/1/approve", body: `{"reason":""}{}`},
		{name: "array", path: "/api/v1/admin/reviews/1/approve", body: `[]`},
		{name: "null", path: "/api/v1/admin/reviews/1/approve", body: `null`},
		{name: "blank rejection", path: "/api/v1/admin/reviews/1/reject", body: `{"reason":" \t\n"}`},
		{name: "overlong rejection", path: "/api/v1/admin/reviews/1/reject", body: `{"reason":"` + strings.Repeat("界", 241) + `"}`},
		{name: "approval with reason", path: "/api/v1/admin/reviews/1/approve", body: `{"reason":"looks good"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOverrideReviewFixture(t, review.StatusReviewing)
			got := fixture.perform("admin", test.path, test.body)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
			}
			if len(fixture.target.statuses) != 0 {
				t.Fatalf("invalid request reached review target: %v", fixture.target.statuses)
			}
		})
	}
}

func TestReviewApprovalAcceptsOnlyEmptyReason(t *testing.T) {
	for _, body := range []string{`{}`, `{"reason":""}`, `{"reason":" \t\n"}`} {
		fixture := newOverrideReviewFixture(t, review.StatusReviewing)
		got := fixture.perform("admin", "/api/v1/admin/reviews/1/approve", body)
		if got.Code != http.StatusOK {
			t.Fatalf("body=%s status=%d response=%s", body, got.Code, got.Body.String())
		}
		if fixture.repo.task.Status != review.StatusApproved || fixture.repo.task.Reason != "" {
			t.Fatalf("body=%s task=%#v", body, fixture.repo.task)
		}
	}
}
