package community

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

type publicProfileRepoStub struct {
	Repository
	stats PublicProfileView
	posts []Post
}

func (r publicProfileRepoStub) PublicProfileStats(context.Context, uint, uint) (PublicProfileView, error) {
	return r.stats, nil
}

func (r publicProfileRepoStub) ListPublicUserPosts(context.Context, uint, int, int) ([]Post, int64, error) {
	return r.posts, int64(len(r.posts)), nil
}

func TestPublicProfileAndPostsUseRealDirectoryAndRepository(t *testing.T) {
	repo := publicProfileRepoStub{
		stats: PublicProfileView{FollowingCount: 2, FollowerCount: 3, LikedCount: 9, IsFollowing: true},
		posts: []Post{{ID: 11, UserID: 7, Title: "真实帖子", ReviewStatus: review.StatusApproved}},
	}
	service := NewService(repo, profileDirectory{7: {ID: 7, Nickname: "真实用户", School: "湛江科技学院", Cover: "/api/v1/media/assets/test-cover/content"}}, nil)

	profile, err := service.PublicProfile(context.Background(), 5, 7)
	if err != nil || profile.Profile.Nickname != "真实用户" || profile.Profile.Cover != "/api/v1/media/assets/test-cover/content" || profile.FollowerCount != 3 || !profile.IsFollowing {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	posts, err := service.PublicUserPosts(context.Background(), 7, 1, 20)
	if err != nil || posts.Total != 1 || len(posts.Items) != 1 || posts.Items[0].Author.ID != 7 {
		t.Fatalf("posts=%+v err=%v", posts, err)
	}
}

func TestPublicProfileRejectsMissingUser(t *testing.T) {
	service := NewService(publicProfileRepoStub{}, profileDirectory{}, nil)
	_, err := service.PublicProfile(context.Background(), 5, 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
