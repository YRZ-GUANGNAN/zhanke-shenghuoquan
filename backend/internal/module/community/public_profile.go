package community

import (
	"context"

	"campus-circle/backend/internal/module/user"
)

type PublicProfileView struct {
	Profile        user.Profile `json:"profile"`
	FollowingCount int64        `json:"followingCount"`
	FollowerCount  int64        `json:"followerCount"`
	LikedCount     int64        `json:"likedCount"`
	IsFollowing    bool         `json:"isFollowing"`
}

type publicProfileRepository interface {
	PublicProfileStats(context.Context, uint, uint) (PublicProfileView, error)
	ListPublicUserPosts(context.Context, uint, int, int) ([]Post, int64, error)
}

func (s *Service) PublicProfile(ctx context.Context, viewerID, userID uint) (PublicProfileView, error) {
	repo, ok := s.repo.(publicProfileRepository)
	if !ok || userID == 0 {
		return PublicProfileView{}, ErrNotFound
	}
	profiles, err := s.users.Profiles(ctx, []uint{userID})
	if err != nil {
		return PublicProfileView{}, err
	}
	profile, exists := profiles[userID]
	if !exists {
		return PublicProfileView{}, ErrNotFound
	}
	value, err := repo.PublicProfileStats(ctx, viewerID, userID)
	if err != nil {
		return PublicProfileView{}, err
	}
	value.Profile = profile
	return value, nil
}

func (s *Service) PublicUserPosts(ctx context.Context, userID uint, page, size int) (Page[PostView], error) {
	repo, ok := s.repo.(publicProfileRepository)
	if !ok || userID == 0 {
		return Page[PostView]{}, ErrNotFound
	}
	page, size = normalize(page, size)
	values, total, err := repo.ListPublicUserPosts(ctx, userID, page, size)
	if err != nil {
		return Page[PostView]{}, err
	}
	items, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: items, Page: page, PageSize: size, Total: total}, err
}
