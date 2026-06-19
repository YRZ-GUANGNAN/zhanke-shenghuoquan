package review

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Submitter interface {
	Submit(context.Context, Submission) (Task, error)
}

type Service struct {
	repo    Repository
	mu      sync.RWMutex
	targets map[string]Target
}

func NewService(repo Repository) *Service { return &Service{repo: repo, targets: map[string]Target{}} }
func (s *Service) RegisterTarget(target Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[target.ObjectType()] = target
}
func (s *Service) Submit(ctx context.Context, input Submission) (Task, error) {
	if input.ObjectType == "" || input.ObjectID == 0 {
		return Task{}, errors.New("invalid review submission")
	}
	value := Task{ObjectType: input.ObjectType, ObjectID: input.ObjectID, SubmitterID: input.SubmitterID, Risk: input.Risk, Status: StatusSubmitted}
	if err := s.repo.Submit(ctx, &value); err != nil {
		return Task{}, fmt.Errorf("submit review: %w", err)
	}
	return value, nil
}
func (s *Service) Start(ctx context.Context, id, operatorID uint) (Task, error) {
	value, err := s.repo.Find(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if value.Status != StatusSubmitted {
		return Task{}, errors.New("only submitted tasks can enter review")
	}
	if err := s.setTarget(ctx, value, StatusReviewing); err != nil {
		return Task{}, err
	}
	value.Status, value.OperatorID = StatusReviewing, operatorID
	if err := s.repo.Save(ctx, &value); err != nil {
		return Task{}, err
	}
	return value, nil
}
func (s *Service) Decide(ctx context.Context, id, operatorID uint, status Status, reason string) (Task, error) {
	if status != StatusApproved && status != StatusRejected {
		return Task{}, errors.New("invalid review decision")
	}
	value, err := s.repo.Find(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if value.Status != StatusReviewing {
		return Task{}, errors.New("task must be reviewing before decision")
	}
	if err := s.setTarget(ctx, value, status); err != nil {
		return Task{}, err
	}
	value.Status, value.OperatorID, value.Reason = status, operatorID, reason
	if err := s.repo.Save(ctx, &value); err != nil {
		return Task{}, err
	}
	return value, nil
}
func (s *Service) setTarget(ctx context.Context, task Task, status Status) error {
	s.mu.RLock()
	target := s.targets[task.ObjectType]
	s.mu.RUnlock()
	if target == nil {
		return fmt.Errorf("review target %q is not registered", task.ObjectType)
	}
	return target.SetReviewStatus(ctx, task.ObjectID, status)
}
func (s *Service) List(ctx context.Context, status Status, page, size int) ([]Task, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.repo.List(ctx, status, page, size)
}
func (s *Service) PendingCount(ctx context.Context) (int64, error) { return s.repo.CountPending(ctx) }
