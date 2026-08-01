package admin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

type reportMemoryRepository struct {
	mu      sync.Mutex
	nextID  uint
	reports map[string]Report
}

func newReportMemoryRepository() *reportMemoryRepository {
	return &reportMemoryRepository{nextID: 1, reports: map[string]Report{}}
}

func reportMemoryKey(value Report) string {
	if value.ClientRequestID == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", value.ReporterID, *value.ClientRequestID)
}

func (r *reportMemoryRepository) CreateReportOrReplay(_ context.Context, value *Report) (bool, error) {
	key := reportMemoryKey(*value)
	if existing, ok := r.reports[key]; ok {
		*value = existing
		return false, nil
	}
	value.ID = r.nextID
	r.nextID++
	r.reports[key] = *value
	return true, nil
}

func (r *reportMemoryRepository) ListReports(context.Context, int, int) ([]Report, int64, error) {
	values := make([]Report, 0, len(r.reports))
	for _, value := range r.reports {
		values = append(values, value)
	}
	return values, int64(len(values)), nil
}

func (r *reportMemoryRepository) SetReportStatus(_ context.Context, id uint, status review.Status) error {
	for key, value := range r.reports {
		if value.ID == id {
			value.ReviewStatus = status
			r.reports[key] = value
			return nil
		}
	}
	return ErrReportNotFound
}

func (r *reportMemoryRepository) WithinTransaction(ctx context.Context, fn func(context.Context, Repository) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := make(map[string]Report, len(r.reports))
	for key, value := range r.reports {
		snapshot[key] = value
	}
	nextID := r.nextID
	if err := fn(ctx, r); err != nil {
		r.reports = snapshot
		r.nextID = nextID
		return err
	}
	return nil
}

type reportReviewMemoryRepository struct {
	mu        sync.Mutex
	nextID    uint
	tasks     map[string]review.Task
	ensureErr error
}

func newReportReviewMemoryRepository() *reportReviewMemoryRepository {
	return &reportReviewMemoryRepository{nextID: 1, tasks: map[string]review.Task{}}
}

func reportReviewKey(value review.Task) string {
	return fmt.Sprintf("%s:%d", value.ObjectType, value.ObjectID)
}

func (r *reportReviewMemoryRepository) Submit(ctx context.Context, value *review.Task) error {
	return r.EnsureSubmitted(ctx, value)
}

func (r *reportReviewMemoryRepository) EnsureSubmitted(_ context.Context, value *review.Task) error {
	if r.ensureErr != nil {
		return r.ensureErr
	}
	key := reportReviewKey(*value)
	if existing, ok := r.tasks[key]; ok {
		*value = existing
		return nil
	}
	value.ID = r.nextID
	r.nextID++
	r.tasks[key] = *value
	return nil
}

func (r *reportReviewMemoryRepository) Find(_ context.Context, id uint) (review.Task, error) {
	for _, value := range r.tasks {
		if value.ID == id {
			return value, nil
		}
	}
	return review.Task{}, review.ErrNotFound
}

func (r *reportReviewMemoryRepository) FindForUpdate(ctx context.Context, id uint) (review.Task, error) {
	return r.Find(ctx, id)
}

func (r *reportReviewMemoryRepository) Save(_ context.Context, value *review.Task) error {
	r.tasks[reportReviewKey(*value)] = *value
	return nil
}

func (r *reportReviewMemoryRepository) List(context.Context, review.Status, int, int) ([]review.Task, int64, error) {
	values := make([]review.Task, 0, len(r.tasks))
	for _, value := range r.tasks {
		values = append(values, value)
	}
	return values, int64(len(values)), nil
}

func (r *reportReviewMemoryRepository) CountPending(context.Context) (int64, error) {
	return int64(len(r.tasks)), nil
}

func (r *reportReviewMemoryRepository) WithinTransaction(ctx context.Context, fn func(context.Context, review.Repository) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fn(ctx, r)
}

func newReportServiceForTest(reportRepo *reportMemoryRepository, reviewRepo *reportReviewMemoryRepository) *Service {
	return NewService(reportRepo, review.NewService(reviewRepo))
}

func TestSubmitReportRollsBackWhenReviewCreationFails(t *testing.T) {
	reportRepo := newReportMemoryRepository()
	reviewRepo := newReportReviewMemoryRepository()
	reviewRepo.ensureErr = errors.New("review unavailable")
	service := newReportServiceForTest(reportRepo, reviewRepo)

	_, err := service.SubmitReport(context.Background(), 7, "post", 9, "spam", "details", "report-request-1")
	if err == nil {
		t.Fatal("review creation failure must be returned")
	}
	if len(reportRepo.reports) != 0 {
		t.Fatalf("orphan reports=%v", reportRepo.reports)
	}
}

func TestSubmitReportIdempotentReplayCreatesOneReportAndReview(t *testing.T) {
	reportRepo := newReportMemoryRepository()
	reviewRepo := newReportReviewMemoryRepository()
	service := newReportServiceForTest(reportRepo, reviewRepo)

	first, err := service.SubmitReport(context.Background(), 7, "post", 9, "spam", "details", "report-request-1")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.SubmitReport(context.Background(), 7, "post", 9, "spam", "details", "report-request-1")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != first.ID || len(reportRepo.reports) != 1 || len(reviewRepo.tasks) != 1 {
		t.Fatalf("first=%#v replayed=%#v reports=%d reviews=%d", first, replayed, len(reportRepo.reports), len(reviewRepo.tasks))
	}
}

func TestSubmitReportIdempotencyPayloadConflict(t *testing.T) {
	reportRepo := newReportMemoryRepository()
	reviewRepo := newReportReviewMemoryRepository()
	service := newReportServiceForTest(reportRepo, reviewRepo)

	if _, err := service.SubmitReport(context.Background(), 7, "post", 9, "spam", "details", "report-request-1"); err != nil {
		t.Fatal(err)
	}
	_, err := service.SubmitReport(context.Background(), 7, "post", 10, "spam", "details", "report-request-1")
	if !errors.Is(err, ErrReportIdempotencyConflict) {
		t.Fatalf("error=%v want=%v", err, ErrReportIdempotencyConflict)
	}
	if len(reportRepo.reports) != 1 || len(reviewRepo.tasks) != 1 {
		t.Fatalf("reports=%d reviews=%d", len(reportRepo.reports), len(reviewRepo.tasks))
	}
}

func TestSubmitReportConcurrentReplayCreatesOneReportAndReview(t *testing.T) {
	reportRepo := newReportMemoryRepository()
	reviewRepo := newReportReviewMemoryRepository()
	service := newReportServiceForTest(reportRepo, reviewRepo)

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	ids := make(chan uint, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := service.SubmitReport(context.Background(), 7, "post", 9, "spam", "details", "report-request-concurrent")
			errs <- err
			ids <- value.ID
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id uint
	for got := range ids {
		if id == 0 {
			id = got
		}
		if got != id {
			t.Fatalf("report ids differ: first=%d got=%d", id, got)
		}
	}
	if len(reportRepo.reports) != 1 || len(reviewRepo.tasks) != 1 {
		t.Fatalf("reports=%d reviews=%d", len(reportRepo.reports), len(reviewRepo.tasks))
	}
}
