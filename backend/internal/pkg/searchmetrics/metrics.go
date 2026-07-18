package searchmetrics

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidKeyword = errors.New("invalid search keyword")
	tableNamePattern  = regexp.MustCompile(`^[a-z_]+$`)
)

type Tables struct {
	History string
	Events  string
}

type HotKeyword struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
	Score   int    `json:"score"`
}

type Metrics interface {
	Confirm(context.Context, uint, string) (string, error)
	History(context.Context, uint) ([]string, error)
	ClearHistory(context.Context, uint) error
	Hot(context.Context) ([]HotKeyword, error)
}

type Store struct {
	db     *gorm.DB
	tables Tables
	now    func() time.Time
}

type historyRow struct {
	ID                uint64
	UserID            uint
	NormalizedKeyword string
	Keyword           string
	UpdatedAt         time.Time
}

type eventRow struct {
	ID                uint64
	UserID            uint
	NormalizedKeyword string
	Keyword           string
	CreatedAt         time.Time
}

func NewStore(db *gorm.DB, tables Tables, now func() time.Time) (*Store, error) {
	if db == nil || !tableNamePattern.MatchString(tables.History) || !tableNamePattern.MatchString(tables.Events) {
		return nil, errors.New("invalid search metrics store configuration")
	}
	if now == nil {
		now = time.Now
	}
	return &Store{db: db, tables: tables, now: now}, nil
}

func Normalize(value string) (display string, normalized string, err error) {
	display = strings.Join(strings.Fields(value), " ")
	length := utf8.RuneCountInString(display)
	if length < 1 || length > 50 {
		return "", "", ErrInvalidKeyword
	}
	return display, strings.ToLower(display), nil
}

func (s *Store) Confirm(ctx context.Context, userID uint, keyword string) (string, error) {
	if userID == 0 {
		return "", ErrInvalidKeyword
	}
	display, normalized, err := Normalize(keyword)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		history := historyRow{UserID: userID, NormalizedKeyword: normalized, Keyword: display, UpdatedAt: now}
		if err := tx.Table(s.tables.History).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "normalized_keyword"}},
			DoUpdates: clause.Assignments(map[string]any{"keyword": display, "updated_at": now}),
		}).Create(&history).Error; err != nil {
			return err
		}

		var recent int64
		if err := tx.Table(s.tables.Events).Where(
			"user_id = ? AND normalized_keyword = ? AND created_at >= ?",
			userID, normalized, now.Add(-30*time.Second),
		).Count(&recent).Error; err != nil {
			return err
		}
		if recent > 0 {
			return nil
		}

		var daily int64
		if err := tx.Table(s.tables.Events).Where(
			"user_id = ? AND normalized_keyword = ? AND created_at >= ?",
			userID, normalized, shanghaiDayStart(now).UTC(),
		).Count(&daily).Error; err != nil {
			return err
		}
		if daily >= 5 {
			return nil
		}
		return tx.Table(s.tables.Events).Create(&eventRow{
			UserID: userID, NormalizedKeyword: normalized, Keyword: display, CreatedAt: now,
		}).Error
	})
	return display, err
}

func (s *Store) History(ctx context.Context, userID uint) ([]string, error) {
	rows := make([]historyRow, 0, 20)
	err := s.db.WithContext(ctx).Table(s.tables.History).
		Where("user_id = ?", userID).Order("updated_at DESC, id DESC").Limit(20).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Keyword)
	}
	return values, nil
}

func (s *Store) ClearHistory(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).Table(s.tables.History).Where("user_id = ?", userID).Delete(&historyRow{}).Error
}

func (s *Store) Hot(ctx context.Context) ([]HotKeyword, error) {
	now := s.now().UTC()
	rows := make([]eventRow, 0)
	start := shanghaiDayStart(now).AddDate(0, 0, -6).UTC()
	if err := s.db.WithContext(ctx).Table(s.tables.Events).
		Where("created_at >= ?", start).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rankHot(rows, now, 10), nil
}

type hotAggregate struct {
	normalized string
	display    string
	count      int
	score      int
	latest     time.Time
	users      map[uint]struct{}
}

func rankHot(rows []eventRow, now time.Time, limit int) []HotKeyword {
	nowDay := shanghaiDayStart(now)
	values := make(map[string]*hotAggregate)
	for _, row := range rows {
		eventDay := shanghaiDayStart(row.CreatedAt)
		days := int(nowDay.Sub(eventDay).Hours() / 24)
		if days < 0 || days > 6 {
			continue
		}
		item := values[row.NormalizedKeyword]
		if item == nil {
			item = &hotAggregate{normalized: row.NormalizedKeyword, display: row.Keyword, users: map[uint]struct{}{}}
			values[row.NormalizedKeyword] = item
		}
		item.count++
		item.score += 7 - days
		item.users[row.UserID] = struct{}{}
		if row.CreatedAt.After(item.latest) {
			item.latest = row.CreatedAt
			item.display = row.Keyword
		}
	}

	eligible := make([]*hotAggregate, 0, len(values))
	for _, item := range values {
		if item.count >= 3 && len(item.users) >= 2 {
			eligible = append(eligible, item)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		left, right := eligible[i], eligible[j]
		if left.score != right.score {
			return left.score > right.score
		}
		if left.count != right.count {
			return left.count > right.count
		}
		if !left.latest.Equal(right.latest) {
			return left.latest.After(right.latest)
		}
		return left.normalized < right.normalized
	})
	if limit < 1 || limit > 20 {
		limit = 10
	}
	if len(eligible) > limit {
		eligible = eligible[:limit]
	}
	result := make([]HotKeyword, 0, len(eligible))
	for _, item := range eligible {
		result = append(result, HotKeyword{Keyword: item.display, Count: item.count, Score: item.score})
	}
	return result
}

func shanghaiDayStart(value time.Time) time.Time {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
