package featureboard

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type stubRepository struct {
	rows       []SuggestionRecord
	total      int64
	created    *Suggestion
	supportSet struct {
		publicID  string
		userID    uint64
		supported bool
	}
	supportErr   error
	notes        []AuthorNote
	noteCreated  *AuthorNote
	deletedNotes []string
	statusSet    struct {
		publicID string
		status   SuggestionStatus
	}
	statusErr error
	commentSuggestion Suggestion
	commentErr        error
	createdComment    *Comment
	commentRows       []Comment
	commentTotal      int64
}

func (s *stubRepository) ListUser(_ context.Context, _ string, _, _ int, _ uint64) ([]SuggestionRecord, int64, error) {
	return s.rows, s.total, nil
}

func (s *stubRepository) ListAdmin(_ context.Context, _ string, _, _ int) ([]SuggestionRecord, int64, error) {
	return s.rows, s.total, nil
}

func (s *stubRepository) CreateSuggestion(_ context.Context, value *Suggestion) error {
	s.created = value
	return nil
}

func (s *stubRepository) SetSupport(_ context.Context, publicID string, userID uint64, supported bool, _ time.Time) (Suggestion, error) {
	s.supportSet.publicID = publicID
	s.supportSet.userID = userID
	s.supportSet.supported = supported
	if s.supportErr != nil {
		return Suggestion{}, s.supportErr
	}
	return Suggestion{PublicID: publicID, Content: "想要夜间模式", Status: StatusOpen, SupportCount: 3}, nil
}

func (s *stubRepository) ListNotes(_ context.Context, _, _ int) ([]AuthorNote, int64, error) {
	return s.notes, int64(len(s.notes)), nil
}

func (s *stubRepository) CreateNote(_ context.Context, value *AuthorNote) error {
	s.noteCreated = value
	return nil
}

func (s *stubRepository) DeleteNote(_ context.Context, publicID string) (bool, error) {
	s.deletedNotes = append(s.deletedNotes, publicID)
	return true, nil
}

func (s *stubRepository) FindSuggestionByPublicID(_ context.Context, publicID string) (Suggestion, error) {
	if s.commentErr != nil {
		return Suggestion{}, s.commentErr
	}
	suggestion := s.commentSuggestion
	if suggestion.PublicID == "" {
		suggestion = Suggestion{ID: 42, PublicID: publicID, Content: "想要夜间模式", Status: StatusOpen}
	}
	return suggestion, nil
}

func (s *stubRepository) SubmitComment(_ context.Context, publicID string, value *Comment) error {
	if s.commentErr != nil {
		return s.commentErr
	}
	if s.commentSuggestion.Status == StatusClosed {
		return ErrClosed
	}
	value.SuggestionID = 42
	_ = publicID
	s.createdComment = value
	return nil
}

func (s *stubRepository) ListComments(_ context.Context, _ uint64, _, _ int) ([]Comment, int64, error) {
	return s.commentRows, s.commentTotal, nil
}

func (s *stubRepository) SetStatus(_ context.Context, publicID string, status SuggestionStatus, _ time.Time) (Suggestion, error) {
	s.statusSet.publicID = publicID
	s.statusSet.status = status
	if s.statusErr != nil {
		return Suggestion{}, s.statusErr
	}
	return Suggestion{PublicID: publicID, Content: "想要夜间模式", Status: status}, nil
}

var testNow = func() time.Time { return time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC) }

func newTestService(repo *stubRepository) *Service {
	service := NewService(repo, testNow)
	service.rand = bytes.NewReader(bytes.Repeat([]byte{0x42}, 256))
	return service
}

var ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func TestSubmitValidatesContent(t *testing.T) {
	repo := &stubRepository{}
	service := newTestService(repo)
	ctx := context.Background()

	if _, err := service.Submit(ctx, 7, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank content must be rejected, got %v", err)
	}
	if _, err := service.Submit(ctx, 7, strings.Repeat("长", maxContentRunes+1)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized content must be rejected, got %v", err)
	}
	if _, err := service.Submit(ctx, 0, "想要夜间模式"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("anonymous submit must be rejected, got %v", err)
	}

	view, err := service.Submit(ctx, 7, "  想要夜间模式  ")
	if err != nil {
		t.Fatal(err)
	}
	if !ulidPattern.MatchString(view.ID) || view.Content != "想要夜间模式" || view.Status != StatusOpen || view.Supported {
		t.Fatalf("unexpected submit view: %+v", view)
	}
	if repo.created == nil || repo.created.UserID != 7 || !repo.created.CreatedAt.Equal(testNow()) {
		t.Fatalf("unexpected created suggestion: %+v", repo.created)
	}
}

func TestListUserClampsPagingAndSort(t *testing.T) {
	repo := &stubRepository{rows: []SuggestionRecord{{Suggestion: Suggestion{PublicID: "a", Content: "x", Status: StatusOpen}, Supported: true}}, total: 1}
	service := newTestService(repo)
	ctx := context.Background()

	if _, err := service.ListUser(ctx, 7, "hot", 1, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown sort must be rejected, got %v", err)
	}
	view, err := service.ListUser(ctx, 7, "", 0, 999)
	if err != nil {
		t.Fatal(err)
	}
	if view.Page != 1 || view.PageSize != maxPageSize || view.Total != 1 || len(view.Items) != 1 || !view.Items[0].Supported {
		t.Fatalf("unexpected list view: %+v", view)
	}
}

func TestSetSupportMapsRepositoryErrors(t *testing.T) {
	repo := &stubRepository{supportErr: gorm.ErrRecordNotFound}
	service := newTestService(repo)
	ctx := context.Background()

	if _, err := service.SetSupport(ctx, 7, "not-a-ulid", true); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid suggestion id must be rejected, got %v", err)
	}
	validID := "01J8Z3Q4M5K6N7P8R9S0T1V2W3"
	if _, err := service.SetSupport(ctx, 7, validID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("record-not-found must map to ErrNotFound, got %v", err)
	}

	repo.supportErr = nil
	view, err := service.SetSupport(ctx, 7, validID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.supportSet.supported || repo.supportSet.publicID != validID || !view.Supported || view.SupportCount != 3 {
		t.Fatalf("unexpected support result: view=%+v repo=%+v", view, repo.supportSet)
	}
}

func TestNotesAndAdminOperations(t *testing.T) {
	repo := &stubRepository{}
	service := newTestService(repo)
	ctx := context.Background()

	if _, err := service.CreateNote(ctx, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank note must be rejected, got %v", err)
	}
	note, err := service.CreateNote(ctx, " 今天上线了功能建议墙 ")
	if err != nil {
		t.Fatal(err)
	}
	if !ulidPattern.MatchString(note.ID) || note.Content != "今天上线了功能建议墙" || repo.noteCreated == nil {
		t.Fatalf("unexpected note: %+v", note)
	}

	if _, err := service.SetStatus(ctx, "bad-id", "accepted"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid id must be rejected, got %v", err)
	}
	if _, err := service.SetStatus(ctx, note.ID, "unknown"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid status must be rejected, got %v", err)
	}
	updated, err := service.SetStatus(ctx, note.ID, "accepted")
	if err != nil || updated.Status != StatusAccepted || repo.statusSet.status != StatusAccepted {
		t.Fatalf("unexpected status update: %+v err=%v", updated, err)
	}

	if err := service.DeleteNote(ctx, note.ID); err != nil {
		t.Fatal(err)
	}
	if len(repo.deletedNotes) != 1 || repo.deletedNotes[0] != note.ID {
		t.Fatalf("unexpected deleted notes: %v", repo.deletedNotes)
	}
}

func TestSubmitCommentValidatesAndPersists(t *testing.T) {
	repo := &stubRepository{}
	service := newTestService(repo)
	ctx := context.Background()
	suggestionID := "01J8Z3Q4M5K6N7P8R9S0T1V2W3"

	if _, err := service.SubmitComment(ctx, 7, suggestionID, " ", false); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank comment must be rejected, got %v", err)
	}
	if _, err := service.SubmitComment(ctx, 7, suggestionID, strings.Repeat("长", maxCommentRunes+1), false); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized comment must be rejected, got %v", err)
	}

	view, err := service.SubmitComment(ctx, 7, suggestionID, " 支持这个想法 ", false)
	if err != nil {
		t.Fatal(err)
	}
	if !ulidPattern.MatchString(view.ID) || view.Content != "支持这个想法" || view.IsMaintainer {
		t.Fatalf("unexpected comment view: %+v", view)
	}
	if repo.createdComment == nil || repo.createdComment.SuggestionID != 42 || repo.createdComment.UserID != 7 {
		t.Fatalf("unexpected created comment: %+v", repo.createdComment)
	}

	reply, err := service.SubmitComment(ctx, 99, suggestionID, "已排入计划", true)
	if err != nil || !reply.IsMaintainer {
		t.Fatalf("maintainer reply must carry the flag: %+v err=%v", reply, err)
	}
}

func TestSubmitCommentRejectsClosedSuggestion(t *testing.T) {
	repo := &stubRepository{commentSuggestion: Suggestion{PublicID: "01J8Z3Q4M5K6N7P8R9S0T1V2W3", Status: StatusClosed}}
	service := newTestService(repo)
	if _, err := service.SubmitComment(context.Background(), 7, "01J8Z3Q4M5K6N7P8R9S0T1V2W3", "还能说话吗", false); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed suggestion must reject comments, got %v", err)
	}
}

func TestListCommentsMapsMissingSuggestion(t *testing.T) {
	repo := &stubRepository{commentErr: gorm.ErrRecordNotFound}
	service := newTestService(repo)
	if _, err := service.ListComments(context.Background(), "01J8Z3Q4M5K6N7P8R9S0T1V2W3", 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing suggestion must map to ErrNotFound, got %v", err)
	}
	repo.commentErr = nil
	repo.commentRows = []Comment{{PublicID: "01J8Z3Q4M5K6N7P8R9S0T1V2W4", Content: "第一个", IsMaintainer: true}}
	repo.commentTotal = 1
	view, err := service.ListComments(context.Background(), "01J8Z3Q4M5K6N7P8R9S0T1V2W3", 1, 20)
	if err != nil || view.Total != 1 || len(view.Items) != 1 || !view.Items[0].IsMaintainer {
		t.Fatalf("unexpected comment list: %+v err=%v", view, err)
	}
}
