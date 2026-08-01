package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

type Submitter interface {
	Submit(context.Context, Submission) (Task, error)
}

type EnsuringSubmitter interface {
	EnsureSubmitted(context.Context, Submission) (Task, error)
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
	value := Task{
		ObjectType: input.ObjectType, ObjectID: input.ObjectID, SubmitterID: input.SubmitterID,
		Risk: input.Risk, Status: StatusSubmitted, DecisionSource: DecisionSourceManual,
	}
	if err := s.repo.Submit(ctx, &value); err != nil {
		return Task{}, fmt.Errorf("submit review: %w", err)
	}
	return s.applyAutomaticDecision(ctx, value)
}

// EnsureSubmitted creates a submitted task only when the target has no task.
// A replay returns the existing task exactly as stored, including terminal
// decisions, and never resets status, reason, operator, risk, or submitter.
func (s *Service) EnsureSubmitted(ctx context.Context, input Submission) (Task, error) {
	if input.ObjectType == "" || input.ObjectID == 0 {
		return Task{}, errors.New("invalid review submission")
	}
	value := Task{
		ObjectType: input.ObjectType, ObjectID: input.ObjectID, SubmitterID: input.SubmitterID,
		Risk: input.Risk, Status: StatusSubmitted, DecisionSource: DecisionSourceManual,
	}
	if err := s.repo.EnsureSubmitted(ctx, &value); err != nil {
		return Task{}, fmt.Errorf("ensure review submitted: %w", err)
	}
	if !validStatus(value.Status) {
		return Task{}, fmt.Errorf("ensure review submitted: invalid existing status %q", value.Status)
	}
	return s.applyAutomaticDecision(ctx, value)
}

func (s *Service) applyAutomaticDecision(ctx context.Context, task Task) (Task, error) {
	if task.Status != StatusSubmitted {
		return task, nil
	}
	decision := ClassifyAutomaticDecision(task.ObjectType, task.Risk)
	if !decision.Apply {
		return task, nil
	}
	return s.mutateTask(ctx, task.ID, func(txCtx context.Context, txRepo Repository, current *Task) error {
		if current.Status != StatusSubmitted {
			return nil
		}
		currentDecision := ClassifyAutomaticDecision(current.ObjectType, current.Risk)
		if !currentDecision.Apply {
			return nil
		}
		if err := s.setTarget(txCtx, *current, currentDecision.Status); err != nil {
			return err
		}
		current.Status = currentDecision.Status
		current.DecisionSource = DecisionSourceAutomatic
		current.Reason = currentDecision.Reason
		current.OperatorID = 0
		return txRepo.Save(txCtx, current)
	})
}

func validStatus(status Status) bool {
	switch status {
	case StatusSubmitted, StatusReviewing, StatusApproved, StatusRejected:
		return true
	default:
		return false
	}
}
func (s *Service) Start(ctx context.Context, id, operatorID uint) (Task, error) {
	return s.mutateTask(ctx, id, func(txCtx context.Context, txRepo Repository, value *Task) error {
		if value.Status != StatusSubmitted {
			return errors.New("only submitted tasks can enter review")
		}
		if err := s.setTarget(txCtx, *value, StatusReviewing); err != nil {
			return err
		}
		value.Status, value.OperatorID, value.DecisionSource = StatusReviewing, operatorID, DecisionSourceManual
		return txRepo.Save(txCtx, value)
	})
}
func (s *Service) Decide(ctx context.Context, id, operatorID uint, status Status, reason string) (Task, error) {
	reason = strings.TrimSpace(reason)
	if status != StatusApproved && status != StatusRejected {
		return Task{}, errors.New("invalid review decision")
	}
	if utf8.RuneCountInString(reason) > 240 {
		return Task{}, errors.New("review reason exceeds 240 characters")
	}
	if status == StatusRejected && reason == "" {
		return Task{}, errors.New("rejected review requires a reason")
	}
	if status == StatusApproved && reason != "" {
		return Task{}, errors.New("approved review must not include a reason")
	}
	return s.mutateTask(ctx, id, func(txCtx context.Context, txRepo Repository, value *Task) error {
		if value.Status != StatusReviewing {
			return errors.New("task must be reviewing before decision")
		}
		if err := s.setTarget(txCtx, *value, status); err != nil {
			return err
		}
		value.Status, value.OperatorID, value.Reason, value.DecisionSource = status, operatorID, reason, DecisionSourceManual
		return txRepo.Save(txCtx, value)
	})
}

func (s *Service) Override(ctx context.Context, id, operatorID uint, status Status, reason string) (Task, error) {
	reason = strings.TrimSpace(reason)
	if status != StatusApproved && status != StatusRejected {
		return Task{}, errors.New("invalid review decision")
	}
	if operatorID == 0 {
		return Task{}, errors.New("review override requires an operator")
	}
	if reason == "" {
		return Task{}, errors.New("review override requires a reason")
	}
	if utf8.RuneCountInString(reason) > 240 {
		return Task{}, errors.New("review override reason exceeds 240 characters")
	}
	return s.mutateTask(ctx, id, func(txCtx context.Context, txRepo Repository, value *Task) error {
		if value.Status != StatusApproved && value.Status != StatusRejected {
			return errors.New("only terminal review tasks can be overridden")
		}
		if err := s.setTarget(txCtx, *value, status); err != nil {
			return err
		}
		value.Status = status
		value.OperatorID = operatorID
		value.Reason = reason
		value.DecisionSource = DecisionSourceManualOverride
		return txRepo.Save(txCtx, value)
	})
}

func (s *Service) mutateTask(
	ctx context.Context,
	id uint,
	mutate func(context.Context, Repository, *Task) error,
) (Task, error) {
	var result Task
	err := s.repo.WithinTransaction(ctx, func(txCtx context.Context, txRepo Repository) error {
		value, err := txRepo.FindForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if err := mutate(txCtx, txRepo, &value); err != nil {
			return err
		}
		result = value
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return result, nil
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
