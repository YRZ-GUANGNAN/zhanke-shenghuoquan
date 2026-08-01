package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

type overrideReviewRepository struct {
	task review.Task
}

func (r *overrideReviewRepository) Submit(_ context.Context, value *review.Task) error {
	value.ID = 1
	r.task = *value
	return nil
}
func (r *overrideReviewRepository) EnsureSubmitted(_ context.Context, value *review.Task) error {
	if r.task.ID != 0 {
		*value = r.task
		return nil
	}
	return r.Submit(context.Background(), value)
}
func (r *overrideReviewRepository) Find(_ context.Context, id uint) (review.Task, error) {
	if id == 0 || id != r.task.ID {
		return review.Task{}, review.ErrNotFound
	}
	return r.task, nil
}
func (r *overrideReviewRepository) FindForUpdate(ctx context.Context, id uint) (review.Task, error) {
	return r.Find(ctx, id)
}
func (r *overrideReviewRepository) Save(_ context.Context, value *review.Task) error {
	r.task = *value
	return nil
}
func (r *overrideReviewRepository) List(context.Context, review.Status, int, int) ([]review.Task, int64, error) {
	return []review.Task{r.task}, 1, nil
}
func (r *overrideReviewRepository) CountPending(context.Context) (int64, error) { return 0, nil }
func (r *overrideReviewRepository) WithinTransaction(ctx context.Context, fn func(context.Context, review.Repository) error) error {
	snapshot := r.task
	if err := fn(ctx, r); err != nil {
		r.task = snapshot
		return err
	}
	return nil
}

type overrideReviewTarget struct {
	statuses []review.Status
}

func (*overrideReviewTarget) ObjectType() string { return "product" }
func (t *overrideReviewTarget) SetReviewStatus(_ context.Context, _ uint, status review.Status) error {
	t.statuses = append(t.statuses, status)
	return nil
}

type overrideReviewFixture struct {
	router *gin.Engine
	repo   *overrideReviewRepository
	target *overrideReviewTarget
}

func newOverrideReviewFixture(t *testing.T, status review.Status) *overrideReviewFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &overrideReviewRepository{task: review.Task{
		ID: 1, ObjectType: "product", ObjectID: 9, Status: status,
		DecisionSource: review.DecisionSourceAutomatic,
	}}
	reviews := review.NewService(repo)
	target := &overrideReviewTarget{}
	reviews.RegisterTarget(target)
	users := user.NewService(nil, "review-handler-test-secret-longer-than-32-bytes", "", "", true)
	auth := func(c *gin.Context) {
		switch c.GetHeader("X-Test-Role") {
		case "admin":
			user.SetCurrentUser(c, user.User{ID: 7, Role: user.RoleAdmin, Status: user.StatusActive})
		case "ordinary":
			user.SetCurrentUser(c, user.User{ID: 8, Role: user.RoleUser, Status: user.StatusActive})
		default:
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
	router := gin.New()
	NewHandler(nil, reviews, users, nil, nil, nil, auth).Register(router.Group("/api/v1"))
	return &overrideReviewFixture{router: router, repo: repo, target: target}
}

func (f *overrideReviewFixture) perform(role, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if role != "" {
		request.Header.Set("X-Test-Role", role)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	return response
}

func TestReviewOverrideHandler(t *testing.T) {
	const path = "/api/v1/admin/reviews/1/override"

	t.Run("secured route requires administrator", func(t *testing.T) {
		fixture := newOverrideReviewFixture(t, review.StatusRejected)
		body := `{"status":"approved","reason":"人工复核确认无违规"}`
		if got := fixture.perform("", path, body); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthorized status=%d body=%s", got.Code, got.Body.String())
		}
		if got := fixture.perform("ordinary", path, body); got.Code != http.StatusForbidden {
			t.Fatalf("ordinary status=%d body=%s", got.Code, got.Body.String())
		}
		if fixture.repo.task.Status != review.StatusRejected || len(fixture.target.statuses) != 0 {
			t.Fatalf("unauthorized request changed review: task=%#v target=%v", fixture.repo.task, fixture.target.statuses)
		}
	})

	t.Run("reason and target decision must be terminal", func(t *testing.T) {
		tests := []string{
			`{"status":"approved","reason":" "}`,
			`{"status":"reviewing","reason":"人工复核"}`,
		}
		for _, body := range tests {
			fixture := newOverrideReviewFixture(t, review.StatusRejected)
			got := fixture.perform("admin", path, body)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("body=%s status=%d response=%s", body, got.Code, got.Body.String())
			}
			if len(fixture.target.statuses) != 0 {
				t.Fatalf("invalid request reached target: %v", fixture.target.statuses)
			}
		}

		fixture := newOverrideReviewFixture(t, review.StatusReviewing)
		got := fixture.perform("admin", path, `{"status":"approved","reason":"人工复核"}`)
		if got.Code != http.StatusBadRequest || len(fixture.target.statuses) != 0 {
			t.Fatalf("non-terminal task status=%d body=%s target=%v", got.Code, got.Body.String(), fixture.target.statuses)
		}
	})

	t.Run("administrator overrides terminal decision", func(t *testing.T) {
		fixture := newOverrideReviewFixture(t, review.StatusRejected)
		got := fixture.perform("admin", path, `{"status":"approved","reason":" 人工复核确认无违规 "}`)
		if got.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
		}
		if fixture.repo.task.Status != review.StatusApproved ||
			fixture.repo.task.DecisionSource != review.DecisionSourceManualOverride ||
			fixture.repo.task.OperatorID != 7 ||
			fixture.repo.task.Reason != "人工复核确认无违规" {
			t.Fatalf("stored task=%#v", fixture.repo.task)
		}
		if len(fixture.target.statuses) != 1 || fixture.target.statuses[0] != review.StatusApproved {
			t.Fatalf("target statuses=%v", fixture.target.statuses)
		}
		var envelope struct {
			Data review.Task `json:"data"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode response: %v body=%s", err, got.Body.String())
		}
		if envelope.Data.DecisionSource != review.DecisionSourceManualOverride {
			t.Fatalf("response task=%#v", envelope.Data)
		}
	})

	t.Run("missing task returns not found", func(t *testing.T) {
		fixture := newOverrideReviewFixture(t, review.StatusRejected)
		got := fixture.perform("admin", "/api/v1/admin/reviews/999/override", `{"status":"approved","reason":"人工复核"}`)
		if got.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
		}
	})
}
