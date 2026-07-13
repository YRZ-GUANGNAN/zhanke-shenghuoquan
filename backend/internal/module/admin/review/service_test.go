package review

import (
	"context"
	"testing"
)

type memoryRepo struct {
	task        Task
	ensureCalls int
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
func (r *memoryRepo) Save(_ context.Context, v *Task) error { r.task = *v; return nil }
func (r *memoryRepo) List(context.Context, Status, int, int) ([]Task, int64, error) {
	return []Task{r.task}, 1, nil
}
func (r *memoryRepo) CountPending(context.Context) (int64, error) { return 1, nil }

type memoryTarget struct{ status Status }

func (*memoryTarget) ObjectType() string { return "post" }
func (t *memoryTarget) SetReviewStatus(_ context.Context, _ uint, status Status) error {
	t.status = status
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
	task, err = service.Decide(context.Background(), task.ID, 1, StatusApproved, "ok")
	if err != nil || task.Status != StatusApproved || target.status != StatusApproved {
		t.Fatalf("approve: %v %#v", err, task)
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
