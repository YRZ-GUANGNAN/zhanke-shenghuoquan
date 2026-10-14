package community

import (
	"context"
	"errors"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

type threadMemoryRepo struct {
	*communityKeyedMemoryRepo
}

func (r *threadMemoryRepo) FindThreadComment(_ context.Context, id uint) (Comment, error) {
	for _, value := range r.comments {
		if value.ID == id {
			return value, nil
		}
	}
	return Comment{}, ErrNotFound
}
func (r *threadMemoryRepo) ListCommentLevel(_ context.Context, postID, rootID, viewerID uint, page, size int) ([]Comment, int64, error) {
	values := []Comment{}
	for _, value := range r.comments {
		if value.PostID == postID && value.ParentCommentID == rootID && visibleThreadComment(value, viewerID) {
			values = append(values, value)
		}
	}
	return values, int64(len(values)), nil
}
func (r *threadMemoryRepo) CommentMetadata(_ context.Context, _ uint, ids []uint, viewerID uint) (map[uint]CommentMetadata, error) {
	return map[uint]CommentMetadata{}, nil
}
func (r *threadMemoryRepo) SetCommentLiked(context.Context, uint, uint, uint, bool) (CommentLikeState, error) {
	return CommentLikeState{}, nil
}

func TestCommentReplyCoordinatesIdempotencyAndModeration(t *testing.T) {
	repo := &threadMemoryRepo{newCommunityKeyedMemoryRepo()}
	repo.posts[10] = Post{ID: 10, ReviewStatus: review.StatusApproved}
	repo.comments = []Comment{
		{ID: 1, PostID: 10, UserID: 2, Content: "根评论", ReviewStatus: review.StatusApproved},
		{ID: 2, PostID: 10, UserID: 3, ParentCommentID: 1, Content: "楼内回复", ReviewStatus: review.StatusApproved},
		{ID: 3, PostID: 11, UserID: 4, Content: "其他帖子", ReviewStatus: review.StatusApproved},
		{ID: 4, PostID: 10, UserID: 4, Content: "待审核", ReviewStatus: review.StatusSubmitted},
	}
	service := NewService(repo, profileDirectory{2: {ID: 2, Nickname: "原作者"}, 3: {ID: 3, Nickname: "回复作者"}, 7: {ID: 7, Nickname: "本人"}}, &submissionRecorder{})
	ctx := context.Background()
	result, err := service.ReplyCommentKeyed(ctx, 10, 2, 7, "  新回复  ", communityTestRequestID, "manual:test")
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.ParentCommentID != 1 || result.Value.ReplyToCommentID != 2 || result.Value.ReplyToUserID != 3 || result.Value.Content != "新回复" || result.Value.ReviewStatus != review.StatusSubmitted {
		t.Fatalf("wrong reply: %+v", result)
	}
	if !strings.Contains(repo.commentEvent.Payload, "manual:test") {
		t.Fatal("reply lost review risk/outbox")
	}
	again, err := service.ReplyCommentKeyed(ctx, 10, 2, 7, "新回复", communityTestRequestID, "changed-risk")
	if err != nil || !again.Replayed || again.Value.ID != result.Value.ID {
		t.Fatalf("retry duplicated reply: %+v %v", again, err)
	}
	if _, err = service.ReplyCommentKeyed(ctx, 10, 1, 7, "新回复", communityTestRequestID, ""); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("same key changed target: %v", err)
	}
	for _, target := range []uint{3, 4, 999} {
		if _, err = service.ReplyCommentKeyed(ctx, 10, target, 7, "新回复", communityTestRequestID, ""); !errors.Is(err, ErrNotFound) {
			t.Fatalf("unsafe target %d: %v", target, err)
		}
	}
	if _, err = service.ReplyCommentKeyed(ctx, 10, 1, 7, strings.Repeat("字", 1001), communityTestRequestID, ""); !errors.Is(err, ErrInvalidComment) {
		t.Fatalf("oversized reply: %v", err)
	}
	repo.comments[0].ReviewStatus = ReviewStatusTakenDown
	if _, err = service.ReplyCommentKeyed(ctx, 10, 2, 7, "新回复", communityTestRequestID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden root remained replyable: %v", err)
	}
}

func TestThreadVisibilityAndReplyEnrichment(t *testing.T) {
	for _, status := range []review.Status{review.StatusSubmitted, review.StatusReviewing, review.StatusRejected} {
		value := Comment{UserID: 7, ReviewStatus: status}
		if !visibleThreadComment(value, 7) || visibleThreadComment(value, 8) || visibleThreadComment(value, 0) {
			t.Fatalf("private moderation state leaked: %s", status)
		}
	}
	if visibleThreadComment(Comment{UserID: 7, ReviewStatus: ReviewStatusTakenDown}, 7) {
		t.Fatal("removed comment was exposed to author")
	}
	repo := &threadMemoryRepo{newCommunityKeyedMemoryRepo()}
	repo.posts[1] = Post{ID: 1, ReviewStatus: review.StatusApproved}
	repo.comments = []Comment{{ID: 1, PostID: 1, UserID: 2, ReviewStatus: review.StatusApproved}, {ID: 2, PostID: 1, UserID: 3, ParentCommentID: 1, ReplyToCommentID: 1, ReplyToUserID: 2, ReviewStatus: review.StatusApproved}}
	service := NewService(repo, profileDirectory{2: {ID: 2, Nickname: "原作者"}, 3: {ID: 3, Nickname: "回复者"}}, &submissionRecorder{})
	page, err := service.CommentReplies(context.Background(), 1, 1, 7, 1, 20)
	if err != nil || len(page.Items) != 1 || page.Items[0].ReplyTo == nil || page.Items[0].ReplyTo.Nickname != "原作者" {
		t.Fatalf("reply attribution: %+v %v", page, err)
	}
	if _, err = service.CommentReplies(context.Background(), 1, 2, 7, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatal("reply was accepted as root")
	}
}
