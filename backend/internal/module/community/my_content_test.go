package community

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

func TestMyPostAllowsOwnerToReadSubmittedPost(t *testing.T) {
	repo := &communityMemoryRepo{posts: map[uint]Post{
		8: {ID: 8, UserID: 7, Title: "真实待审核帖子", ReviewStatus: review.StatusSubmitted},
	}}
	service := NewService(repo, profileDirectory{
		7: {ID: 7, Nickname: "发布者"},
	}, nil)

	post, err := service.MyPost(context.Background(), 7, 8)
	if err != nil {
		t.Fatalf("owner read submitted post: %v", err)
	}
	if post.ID != 8 || post.Title != "真实待审核帖子" || post.Author.ID != 7 {
		t.Fatalf("unexpected post: %+v", post)
	}
}

func TestMyPostHidesAnotherUsersPost(t *testing.T) {
	repo := &communityMemoryRepo{posts: map[uint]Post{
		8: {ID: 8, UserID: 7, Title: "私有待审核帖子", ReviewStatus: review.StatusSubmitted},
	}}
	service := NewService(repo, profileDirectory{
		7: {ID: 7, Nickname: "发布者"},
	}, nil)

	_, err := service.MyPost(context.Background(), 9, 8)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner received private post: %v", err)
	}
}
