package community

import (
	"context"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

type MyContentCounts struct {
	Favorites int64 `json:"favorites"`
	Posts     int64 `json:"posts"`
	Comments  int64 `json:"comments"`
}

type MyCommentView struct {
	ID           uint          `json:"id"`
	PostID       uint          `json:"postId"`
	PostTitle    string        `json:"postTitle"`
	Content      string        `json:"content"`
	ReviewStatus review.Status `json:"reviewStatus"`
	CreatedAt    time.Time     `json:"createdAt"`
}

type myContentRepository interface {
	ListOwnedPosts(context.Context, uint, int, int) ([]Post, int64, error)
	ListFavoritePosts(context.Context, uint, int, int) ([]Post, int64, error)
	ListOwnedComments(context.Context, uint, int, int) ([]MyCommentView, int64, error)
	MyContentCounts(context.Context, uint) (MyContentCounts, error)
}

func (s *Service) MyPosts(ctx context.Context, userID uint, page, size int) (Page[PostView], error) {
	repo, ok := s.repo.(myContentRepository)
	if !ok {
		return Page[PostView]{}, ErrNotFound
	}
	page, size = normalize(page, size)
	values, total, err := repo.ListOwnedPosts(ctx, userID, page, size)
	if err != nil {
		return Page[PostView]{}, err
	}
	items, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: items, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) MyFavoritePosts(ctx context.Context, userID uint, page, size int) (Page[PostView], error) {
	repo, ok := s.repo.(myContentRepository)
	if !ok {
		return Page[PostView]{}, ErrNotFound
	}
	page, size = normalize(page, size)
	values, total, err := repo.ListFavoritePosts(ctx, userID, page, size)
	if err != nil {
		return Page[PostView]{}, err
	}
	items, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: items, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) MyComments(ctx context.Context, userID uint, page, size int) (Page[MyCommentView], error) {
	repo, ok := s.repo.(myContentRepository)
	if !ok {
		return Page[MyCommentView]{}, ErrNotFound
	}
	page, size = normalize(page, size)
	values, total, err := repo.ListOwnedComments(ctx, userID, page, size)
	return Page[MyCommentView]{Items: values, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) MySummary(ctx context.Context, userID uint) (MyContentCounts, error) {
	repo, ok := s.repo.(myContentRepository)
	if !ok {
		return MyContentCounts{}, ErrNotFound
	}
	return repo.MyContentCounts(ctx, userID)
}
