package review

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type memoryRepo struct {
	mu          sync.Mutex
	task        Task
	ensureCalls int
	saveErr     error
	projection  Status
}

func (r *memoryRepo) Submit(_ context.Context, v *Task) error { v.ID = 1; r.task = *v; return nil }
func (r *memoryRepo) EnsureSubmitted(_ context.Context, v *Task) error {
	r.ensureCalls++
	if r.task.ID != 0 {
		*v = r.task
		return nil
	}
	v.ID = 1
	r.task = *v
	return nil
}
func (r *memoryRepo) Find(_ context.Context, id uint) (Task, error) {
	if id != r.task.ID {
		return Task{}, ErrNotFound
	}
	return r.task, nil
}
func (r *memoryRepo) FindForUpdate(ctx context.Context, id uint) (Task, error) {
	return r.Find(ctx, id)
}
func (r *memoryRepo) Save(_ context.Context, v *Task) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.task = *v
	return nil
}
func (r *memoryRepo) List(context.Context, Status, int, int) ([]Task, int64, error) {
	return []Task{r.task}, 1, nil
}
func (r *memoryRepo) CountPending(context.Context) (int64, error) { return 1, nil }
func (r *memoryRepo) WithinTransaction(ctx context.Context, fn func(context.Context, Repository) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := r.task
	projectionSnapshot := r.projection
	if err := fn(ctx, r); err != nil {
		r.task = snapshot
		r.projection = projectionSnapshot
		return err
	}
	return nil
}

type memoryTarget struct{ status Status }

func (*memoryTarget) ObjectType() string { return "post" }
func (t *memoryTarget) SetReviewStatus(_ context.Context, _ uint, status Status) error {
	t.status = status
	return nil
}

type recordingTarget struct {
	objectType string
	statuses   []Status
}

func (t *recordingTarget) ObjectType() string { return t.objectType }
func (t *recordingTarget) SetReviewStatus(_ context.Context, _ uint, status Status) error {
	t.statuses = append(t.statuses, status)
	return nil
}

func TestReviewWorkflow(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	target := &memoryTarget{}
	service.RegisterTarget(target)
	task, err := service.Submit(context.Background(), Submission{ObjectType: "post", ObjectID: 9, SubmitterID: 2})
	if err != nil || task.Status != StatusSubmitted {
		t.Fatalf("submit: %v %#v", err, task)
	}
	if _, err = service.Decide(context.Background(), task.ID, 1, StatusApproved, ""); err == nil {
		t.Fatal("decision before start must fail")
	}
	task, err = service.Start(context.Background(), task.ID, 1)
	if err != nil || task.Status != StatusReviewing || target.status != StatusReviewing {
		t.Fatalf("start: %v %#v", err, task)
	}
	task, err = service.Decide(context.Background(), task.ID, 1, StatusApproved, "")
	if err != nil || task.Status != StatusApproved || target.status != StatusApproved {
		t.Fatalf("approve: %v %#v", err, task)
	}
}

func TestDecisionReasonValidation(t *testing.T) {
	repo := &memoryRepo{task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusReviewing}}
	service := NewService(repo)
	service.RegisterTarget(&memoryTarget{})

	tests := []struct {
		name   string
		status Status
		reason string
	}{
		{name: "blank rejection", status: StatusRejected, reason: " \t\n"},
		{name: "approval reason", status: StatusApproved, reason: "looks good"},
		{name: "overlong rejection", status: StatusRejected, reason: strings.Repeat("界", 241)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Decide(context.Background(), 1, 7, test.status, test.reason); err == nil {
				t.Fatal("invalid decision reason must fail")
			}
		})
	}
}

type transactionalStatusTarget struct {
	repo *memoryRepo
	seen Status
}

func (*transactionalStatusTarget) ObjectType() string { return "post" }
func (t *transactionalStatusTarget) SetReviewStatus(_ context.Context, _ uint, status Status) error {
	t.seen = status
	t.repo.projection = status
	return nil
}

func TestDecisionSaveFailureRollsBackTask(t *testing.T) {
	wantErr := errors.New("save failed")
	repo := &memoryRepo{
		task:    Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusReviewing},
		saveErr: wantErr,
	}
	service := NewService(repo)
	target := &transactionalStatusTarget{repo: repo}
	service.RegisterTarget(target)

	if _, err := service.Decide(context.Background(), 1, 7, StatusApproved, ""); !errors.Is(err, wantErr) {
		t.Fatalf("error=%v want=%v", err, wantErr)
	}
	if repo.task.Status != StatusReviewing {
		t.Fatalf("task status=%s want=%s", repo.task.Status, StatusReviewing)
	}
	if repo.projection != "" {
		t.Fatalf("target projection was not rolled back: %s", repo.projection)
	}
}

func TestConcurrentDecisionHasSingleWinner(t *testing.T) {
	repo := &memoryRepo{task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusReviewing}}
	service := NewService(repo)
	service.RegisterTarget(&memoryTarget{})

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, decision := range []struct {
		status Status
		reason string
	}{{StatusApproved, ""}, {StatusRejected, "policy violation"}} {
		wg.Add(1)
		go func(status Status, reason string) {
			defer wg.Done()
			_, err := service.Decide(context.Background(), 1, 7, status, reason)
			errs <- err
		}(decision.status, decision.reason)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful decisions=%d want=1", successes)
	}
}

func TestEnsureSubmittedCreatesOnceAndReturnsExistingStateUnchanged(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	input := Submission{ObjectType: "post", ObjectID: 9, SubmitterID: 2, Risk: "initial"}

	created, err := service.EnsureSubmitted(context.Background(), input)
	if err != nil || created.ID != 1 || created.Status != StatusSubmitted {
		t.Fatalf("create ensure: task=%#v err=%v", created, err)
	}
	existing := created
	existing.Status = StatusApproved
	existing.Reason = "approved once"
	existing.OperatorID = 7
	repo.task = existing

	replayed, err := service.EnsureSubmitted(context.Background(), Submission{
		ObjectType: "post", ObjectID: 9, SubmitterID: 99, Risk: "must not overwrite",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed != existing || repo.task != existing {
		t.Fatalf("replay changed existing task: replayed=%#v stored=%#v want=%#v", replayed, repo.task, existing)
	}
	if repo.ensureCalls != 2 {
		t.Fatalf("ensure calls=%d want=2", repo.ensureCalls)
	}
}

func TestEnsureSubmittedFailsClosedForInvalidStoredStatus(t *testing.T) {
	repo := &memoryRepo{task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: Status("corrupt")}}
	service := NewService(repo)
	if _, err := service.EnsureSubmitted(context.Background(), Submission{ObjectType: "post", ObjectID: 9}); err == nil {
		t.Fatal("invalid stored status must fail closed")
	}
}

func TestEnsureSubmittedPreservesEveryLegalExistingState(t *testing.T) {
	statuses := []Status{StatusSubmitted, StatusReviewing, StatusApproved, StatusRejected}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			existing := Task{
				ID: 4, ObjectType: "product", ObjectID: 12, SubmitterID: 8,
				Risk: "original", Status: status, Reason: "keep", OperatorID: 6,
			}
			repo := &memoryRepo{task: existing}
			service := NewService(repo)
			got, err := service.EnsureSubmitted(context.Background(), Submission{
				ObjectType: "product", ObjectID: 12, SubmitterID: 99, Risk: "replacement",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got != existing || repo.task != existing {
				t.Fatalf("status %s changed: got=%#v stored=%#v", status, got, repo.task)
			}
		})
	}
}

func TestEnsureSubmittedAutomaticallyApprovesTrustedResultOnce(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	target := &recordingTarget{objectType: "product"}
	service.RegisterTarget(target)
	input := Submission{
		ObjectType: "product", ObjectID: 12, SubmitterID: 8, Risk: "wechat pass",
	}

	created, err := service.EnsureSubmitted(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != StatusApproved || created.DecisionSource != DecisionSourceAutomatic {
		t.Fatalf("automatic task=%#v", created)
	}
	if len(target.statuses) != 1 || target.statuses[0] != StatusApproved {
		t.Fatalf("target statuses=%v", target.statuses)
	}

	replayed, err := service.EnsureSubmitted(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed != created {
		t.Fatalf("replayed=%#v want=%#v", replayed, created)
	}
	if len(target.statuses) != 1 {
		t.Fatalf("terminal replay repeated target side effect: %v", target.statuses)
	}
}

func TestSubmitAutomaticallyRejectsRiskyResult(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	target := &recordingTarget{objectType: "comment"}
	service.RegisterTarget(target)

	task, err := service.Submit(context.Background(), Submission{
		ObjectType: "comment", ObjectID: 5, SubmitterID: 3, Risk: "wechat risky label=100",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusRejected || task.DecisionSource != DecisionSourceAutomatic {
		t.Fatalf("automatic task=%#v", task)
	}
	if len(target.statuses) != 1 || target.statuses[0] != StatusRejected {
		t.Fatalf("target statuses=%v", target.statuses)
	}
}

func TestEnsureSubmittedKeepsAmbiguousResultForManualReview(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	task, err := service.EnsureSubmitted(context.Background(), Submission{
		ObjectType: "product", ObjectID: 12, SubmitterID: 8, Risk: "wechat review label=20001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusSubmitted || task.DecisionSource != DecisionSourceManual {
		t.Fatalf("manual task=%#v", task)
	}
}

func TestOverrideAutomaticDecision(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	target := &recordingTarget{objectType: "product"}
	service.RegisterTarget(target)
	automatic, err := service.EnsureSubmitted(context.Background(), Submission{
		ObjectType: "product", ObjectID: 12, SubmitterID: 8, Risk: "wechat risky label=100",
	})
	if err != nil {
		t.Fatal(err)
	}

	overridden, err := service.Override(context.Background(), automatic.ID, 27, StatusApproved, " 人工复核确认无违规 ")
	if err != nil {
		t.Fatal(err)
	}
	if overridden.Status != StatusApproved ||
		overridden.DecisionSource != DecisionSourceManualOverride ||
		overridden.OperatorID != 27 ||
		overridden.Reason != "人工复核确认无违规" {
		t.Fatalf("overridden=%#v", overridden)
	}
	if len(target.statuses) != 2 || target.statuses[0] != StatusRejected || target.statuses[1] != StatusApproved {
		t.Fatalf("target statuses=%v", target.statuses)
	}
}

func TestOverrideRequiresTerminalTaskOperatorAndReason(t *testing.T) {
	tests := []struct {
		name       string
		task       Task
		operatorID uint
		status     Status
		reason     string
	}{
		{name: "not terminal", task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusReviewing}, operatorID: 1, status: StatusApproved, reason: "ok"},
		{name: "missing operator", task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusRejected}, status: StatusApproved, reason: "ok"},
		{name: "missing reason", task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusRejected}, operatorID: 1, status: StatusApproved, reason: " "},
		{name: "invalid status", task: Task{ID: 1, ObjectType: "post", ObjectID: 9, Status: StatusRejected}, operatorID: 1, status: StatusReviewing, reason: "ok"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &memoryRepo{task: test.task}
			service := NewService(repo)
			service.RegisterTarget(&memoryTarget{})
			if _, err := service.Override(context.Background(), test.task.ID, test.operatorID, test.status, test.reason); err == nil {
				t.Fatal("override must fail")
			}
		})
	}
}
