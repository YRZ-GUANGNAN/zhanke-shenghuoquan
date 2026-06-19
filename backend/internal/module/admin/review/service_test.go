package review

import (
	"context"
	"testing"
)

type memoryRepo struct{ task Task }

func (r *memoryRepo) Submit(_ context.Context, v *Task) error { v.ID = 1; r.task = *v; return nil }
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
