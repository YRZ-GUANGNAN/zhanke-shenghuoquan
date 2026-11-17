package featureboard

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrInvalidInput = errors.New("feature board: invalid input")
	ErrNotFound     = errors.New("feature board: suggestion not found")
	ErrClosed       = errors.New("feature board: suggestion closed")
)

const (
	minContentRunes = 2
	maxContentRunes = 500
	maxNoteRunes    = 1000
	maxCommentRunes = 300
	defaultPageSize = 10
	maxPageSize     = 20
	notesLimit      = 10
	ulidAlphabet    = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

type Service struct {
	repo Repository
	now  func() time.Time
	rand io.Reader
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now, rand: rand.Reader}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func toView(record SuggestionRecord) SuggestionView {
	return SuggestionView{
		ID:           record.PublicID,
		Content:      record.Content,
		Status:       record.Status,
		SupportCount: record.SupportCount,
		Supported:    record.Supported,
		CommentCount: record.CommentCount,
		CreatedAt:    record.CreatedAt,
	}
}

func toCommentView(comment Comment) CommentView {
	return CommentView{ID: comment.PublicID, Content: comment.Content, IsMaintainer: comment.IsMaintainer, CreatedAt: comment.CreatedAt}
}

func toAdminView(record SuggestionRecord) AdminSuggestionView {
	return AdminSuggestionView{
		ID:           record.PublicID,
		UserID:       record.UserID,
		Content:      record.Content,
		Status:       record.Status,
		SupportCount: record.SupportCount,
		CreatedAt:    record.CreatedAt,
	}
}

func toNoteView(note AuthorNote) NoteView {
	return NoteView{ID: note.PublicID, Content: note.Content, CreatedAt: note.CreatedAt}
}

// ListUser lists open/accepted suggestions. sort is "" or "supports" (default)
// and "latest".
func (s *Service) ListUser(ctx context.Context, viewerID uint64, sort string, page, pageSize int) (ListView, error) {
	sort = strings.TrimSpace(sort)
	if sort != "" && sort != "supports" && sort != "latest" {
		return ListView{}, ErrInvalidInput
	}
	if viewerID == 0 {
		return ListView{}, ErrInvalidInput
	}
	page, pageSize = normalizePage(page, pageSize)
	rows, total, err := s.repo.ListUser(ctx, sort, (page-1)*pageSize, pageSize, viewerID)
	if err != nil {
		return ListView{}, err
	}
	items := make([]SuggestionView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toView(row))
	}
	return ListView{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) Submit(ctx context.Context, userID uint64, content string) (SuggestionView, error) {
	content = strings.TrimSpace(content)
	length := utf8.RuneCountInString(content)
	if userID == 0 || length < minContentRunes || length > maxContentRunes {
		return SuggestionView{}, ErrInvalidInput
	}
	at := s.now().UTC()
	publicID, err := newULIDAt(at, s.rand)
	if err != nil {
		return SuggestionView{}, err
	}
	suggestion := &Suggestion{
		PublicID: publicID, UserID: userID, Content: content,
		Status: StatusOpen, CreatedAt: at, UpdatedAt: at,
	}
	if err := s.repo.CreateSuggestion(ctx, suggestion); err != nil {
		return SuggestionView{}, err
	}
	return toView(SuggestionRecord{Suggestion: *suggestion}), nil
}

func (s *Service) SetSupport(ctx context.Context, userID uint64, publicID string, supported bool) (SuggestionView, error) {
	publicID = strings.TrimSpace(publicID)
	if userID == 0 || !validULID(publicID) {
		return SuggestionView{}, ErrInvalidInput
	}
	suggestion, err := s.repo.SetSupport(ctx, publicID, userID, supported, s.now().UTC())
	if err != nil {
		return SuggestionView{}, normalizeRepoError(err)
	}
	return SuggestionView{
		ID: suggestion.PublicID, Content: suggestion.Content, Status: suggestion.Status,
		SupportCount: suggestion.SupportCount, Supported: supported, CreatedAt: suggestion.CreatedAt,
	}, nil
}

func (s *Service) Notes(ctx context.Context) ([]NoteView, error) {
	notes, err := s.repo.ListNotes(ctx, notesLimit)
	if err != nil {
		return nil, err
	}
	views := make([]NoteView, 0, len(notes))
	for _, note := range notes {
		views = append(views, toNoteView(note))
	}
	return views, nil
}

func (s *Service) ListAdmin(ctx context.Context, status string, page, pageSize int) (AdminListView, error) {
	status = strings.TrimSpace(status)
	if status != "" && !SuggestionStatus(status).Valid() {
		return AdminListView{}, ErrInvalidInput
	}
	page, pageSize = normalizePage(page, pageSize)
	rows, total, err := s.repo.ListAdmin(ctx, status, (page-1)*pageSize, pageSize)
	if err != nil {
		return AdminListView{}, err
	}
	items := make([]AdminSuggestionView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAdminView(row))
	}
	return AdminListView{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) SetStatus(ctx context.Context, publicID, status string) (AdminSuggestionView, error) {
	publicID = strings.TrimSpace(publicID)
	status = strings.TrimSpace(status)
	if !validULID(publicID) || !SuggestionStatus(status).Valid() {
		return AdminSuggestionView{}, ErrInvalidInput
	}
	suggestion, err := s.repo.SetStatus(ctx, publicID, SuggestionStatus(status), s.now().UTC())
	if err != nil {
		return AdminSuggestionView{}, normalizeRepoError(err)
	}
	return toAdminView(SuggestionRecord{Suggestion: suggestion}), nil
}

func (s *Service) CreateNote(ctx context.Context, content string) (NoteView, error) {
	content = strings.TrimSpace(content)
	length := utf8.RuneCountInString(content)
	if length < minContentRunes || length > maxNoteRunes {
		return NoteView{}, ErrInvalidInput
	}
	at := s.now().UTC()
	publicID, err := newULIDAt(at, s.rand)
	if err != nil {
		return NoteView{}, err
	}
	note := &AuthorNote{PublicID: publicID, Content: content, CreatedAt: at}
	if err := s.repo.CreateNote(ctx, note); err != nil {
		return NoteView{}, err
	}
	return toNoteView(*note), nil
}

func (s *Service) ListNotesAdmin(ctx context.Context) ([]NoteView, error) {
	notes, err := s.repo.ListNotes(ctx, notesLimit)
	if err != nil {
		return nil, err
	}
	views := make([]NoteView, 0, len(notes))
	for _, note := range notes {
		views = append(views, toNoteView(note))
	}
	return views, nil
}

func (s *Service) DeleteNote(ctx context.Context, publicID string) error {
	publicID = strings.TrimSpace(publicID)
	if !validULID(publicID) {
		return ErrInvalidInput
	}
	deleted, err := s.repo.DeleteNote(ctx, publicID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func normalizeRepoError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func validULID(value string) bool {
	if len(value) != 26 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(ulidAlphabet, character) {
			return false
		}
	}
	return true
}

func newULIDAt(at time.Time, randomness io.Reader) (string, error) {
	if randomness == nil {
		return "", ErrInvalidInput
	}
	milliseconds := at.UnixMilli()
	if milliseconds < 0 || milliseconds > (1<<48)-1 {
		return "", ErrInvalidInput
	}
	raw := make([]byte, 16)
	for index := 5; index >= 0; index-- {
		raw[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	if _, err := io.ReadFull(randomness, raw[6:]); err != nil {
		return "", err
	}
	value := new(big.Int).SetBytes(raw)
	base := big.NewInt(32)
	remainder := new(big.Int)
	encoded := make([]byte, 26)
	for index := len(encoded) - 1; index >= 0; index-- {
		value.QuoRem(value, base, remainder)
		encoded[index] = ulidAlphabet[remainder.Int64()]
	}
	return string(encoded), nil
}

// SubmitComment appends one message to a suggestion's conversation. Maintainer
// replies are created through the same path with maintainer=true.
func (s *Service) SubmitComment(ctx context.Context, userID uint64, publicID, content string, maintainer bool) (CommentView, error) {
	publicID = strings.TrimSpace(publicID)
	content = strings.TrimSpace(content)
	length := utf8.RuneCountInString(content)
	if userID == 0 || !validULID(publicID) || length < 1 || length > maxCommentRunes {
		return CommentView{}, ErrInvalidInput
	}
	suggestion, err := s.repo.FindSuggestionByPublicID(ctx, publicID)
	if err != nil {
		return CommentView{}, normalizeRepoError(err)
	}
	if suggestion.Status == StatusClosed {
		return CommentView{}, ErrClosed
	}
	at := s.now().UTC()
	commentID, err := newULIDAt(at, s.rand)
	if err != nil {
		return CommentView{}, err
	}
	comment := &Comment{
		PublicID: commentID, SuggestionID: suggestion.ID, UserID: userID,
		IsMaintainer: maintainer, Content: content, CreatedAt: at,
	}
	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return CommentView{}, err
	}
	return toCommentView(*comment), nil
}

// ListComments returns the conversation of one suggestion, oldest first.
func (s *Service) ListComments(ctx context.Context, publicID string, page, pageSize int) (CommentListView, error) {
	publicID = strings.TrimSpace(publicID)
	if !validULID(publicID) {
		return CommentListView{}, ErrInvalidInput
	}
	suggestion, err := s.repo.FindSuggestionByPublicID(ctx, publicID)
	if err != nil {
		return CommentListView{}, normalizeRepoError(err)
	}
	page, pageSize = normalizePage(page, pageSize)
	rows, total, err := s.repo.ListComments(ctx, suggestion.ID, (page-1)*pageSize, pageSize)
	if err != nil {
		return CommentListView{}, err
	}
	items := make([]CommentView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toCommentView(row))
	}
	return CommentListView{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}
