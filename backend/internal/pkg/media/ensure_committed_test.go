package media

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type ensureCommittedRepo struct {
	Repository
	commitErr   error
	readErr     error
	values      []Asset
	commitCalls int
	readCalls   int
	mutations   int
	audits      int
	committed   bool
}

func (r *ensureCommittedRepo) Commit(_ context.Context, ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) error {
	r.commitCalls++
	if r.committed {
		if r.commitErr != nil {
			return r.commitErr
		}
		return ErrInvalidState
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.values = make([]Asset, len(orderedIDs))
	for i, publicID := range orderedIDs {
		r.values[i] = Asset{
			PublicID:      publicID,
			OwnerUserID:   ownerID,
			Status:        StatusApproved,
			ReferenceType: referenceType,
			ReferenceID:   referenceID,
			ReferenceSort: i,
		}
	}
	r.committed = true
	r.mutations += len(orderedIDs)
	r.audits += len(orderedIDs)
	return nil
}

func (r *ensureCommittedRepo) FindActiveReferenceSet(_ context.Context, _ uint, _ string, _ uint64) ([]Asset, error) {
	r.readCalls++
	if r.readErr != nil {
		return nil, r.readErr
	}
	return append([]Asset(nil), r.values...), nil
}

type commitOnlyRepo struct {
	Repository
	err   error
	calls int
}

func (r *commitOnlyRepo) Commit(context.Context, uint, string, string, uint64, []string) error {
	r.calls++
	return r.err
}

func newEnsureCommittedService(repo Repository) *Service {
	return NewService(repo, nil, nil, nil, nil, "", "", "")
}

func TestEnsureCommittedFirstSuccess(t *testing.T) {
	repo := &ensureCommittedRepo{}
	service := newEnsureCommittedService(repo)
	ids := []string{"asset-a", "asset-b"}

	if err := service.EnsureCommitted(context.Background(), 7, "token", "post", 91, ids); err != nil {
		t.Fatalf("EnsureCommitted() error = %v", err)
	}
	if repo.commitCalls != 1 || repo.readCalls != 0 || repo.mutations != 2 || repo.audits != 2 {
		t.Fatalf("unexpected calls/mutations: commit=%d read=%d mutations=%d audits=%d", repo.commitCalls, repo.readCalls, repo.mutations, repo.audits)
	}
	if !exactCommittedReferenceSet(repo.values, 7, "post", 91, ids) {
		t.Fatal("first commit did not persist the exact reference set")
	}
}

func TestEnsureCommittedExactReplayIsIdempotent(t *testing.T) {
	repo := &ensureCommittedRepo{}
	service := newEnsureCommittedService(repo)
	ids := []string{"asset-a", "asset-b"}

	if err := service.EnsureCommitted(context.Background(), 7, "token", "product", 92, ids); err != nil {
		t.Fatalf("first EnsureCommitted() error = %v", err)
	}
	before := append([]Asset(nil), repo.values...)
	beforeMutations, beforeAudits := repo.mutations, repo.audits
	if err := service.EnsureCommitted(context.Background(), 7, "token", "product", 92, ids); err != nil {
		t.Fatalf("replay EnsureCommitted() error = %v", err)
	}
	if repo.commitCalls != 2 || repo.readCalls != 1 {
		t.Fatalf("unexpected calls: commit=%d read=%d", repo.commitCalls, repo.readCalls)
	}
	if repo.mutations != beforeMutations || repo.audits != beforeAudits || !reflect.DeepEqual(repo.values, before) {
		t.Fatal("exact replay changed persisted state or audit count")
	}
}

func TestEnsureCommittedMismatchPreservesCommitError(t *testing.T) {
	repo := &ensureCommittedRepo{
		committed: true,
		commitErr: ErrInvalidState,
		values: []Asset{{
			PublicID: "asset-b", OwnerUserID: 7, Status: StatusApproved,
			ReferenceType: "post", ReferenceID: 93, ReferenceSort: 0,
		}},
	}
	service := newEnsureCommittedService(repo)

	err := service.EnsureCommitted(context.Background(), 7, "token", "post", 93, []string{"asset-a"})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("EnsureCommitted() error = %v, want ErrInvalidState", err)
	}
}

func TestEnsureCommittedValidationDoesNotTouchRepository(t *testing.T) {
	tests := []struct {
		name          string
		ownerID       uint
		token         string
		referenceType string
		referenceID   uint64
		ids           []string
	}{
		{name: "zero owner", token: "token", referenceType: "post", referenceID: 1, ids: []string{"asset"}},
		{name: "blank token", ownerID: 1, token: " \t", referenceType: "post", referenceID: 1, ids: []string{"asset"}},
		{name: "invalid reference type", ownerID: 1, token: "token", referenceType: "dish", referenceID: 1, ids: []string{"asset"}},
		{name: "zero reference id", ownerID: 1, token: "token", referenceType: "post", ids: []string{"asset"}},
		{name: "empty ids", ownerID: 1, token: "token", referenceType: "post", referenceID: 1},
		{name: "blank id", ownerID: 1, token: "token", referenceType: "post", referenceID: 1, ids: []string{"  "}},
		{name: "duplicate id", ownerID: 1, token: "token", referenceType: "post", referenceID: 1, ids: []string{"asset", "asset"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &ensureCommittedRepo{}
			err := newEnsureCommittedService(repo).EnsureCommitted(context.Background(), tt.ownerID, tt.token, tt.referenceType, tt.referenceID, tt.ids)
			if !errors.Is(err, ErrInvalidAsset) {
				t.Fatalf("EnsureCommitted() error = %v, want ErrInvalidAsset", err)
			}
			if repo.commitCalls != 0 || repo.readCalls != 0 {
				t.Fatalf("repository touched during validation: commit=%d read=%d", repo.commitCalls, repo.readCalls)
			}
		})
	}
}

func TestEnsureCommittedWithoutReaderReturnsCommitError(t *testing.T) {
	want := errors.New("commit failed")
	repo := &commitOnlyRepo{err: want}
	err := newEnsureCommittedService(repo).EnsureCommitted(context.Background(), 1, "token", "user", 1, []string{"asset"})
	if !errors.Is(err, want) {
		t.Fatalf("EnsureCommitted() error = %v, want %v", err, want)
	}
	if repo.calls != 1 {
		t.Fatalf("Commit() calls = %d, want 1", repo.calls)
	}
}

func TestEnsureCommittedReturnsReaderError(t *testing.T) {
	readErr := errors.New("read failed")
	repo := &ensureCommittedRepo{committed: true, commitErr: ErrInvalidState, readErr: readErr}
	err := newEnsureCommittedService(repo).EnsureCommitted(context.Background(), 1, "token", "post", 1, []string{"asset"})
	if !errors.Is(err, readErr) {
		t.Fatalf("EnsureCommitted() error = %v, want %v", err, readErr)
	}
}
