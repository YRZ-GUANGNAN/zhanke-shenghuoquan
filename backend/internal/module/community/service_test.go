package community

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/media"
)

type communityMemoryRepo struct {
	posts         map[uint]Post
	comments      []Comment
	followed      bool
	notifications []Notification
	readID        uint
}

func (r *communityMemoryRepo) CreatePost(_ context.Context, value *Post) error {
	value.ID = uint(len(r.posts) + 1)
	r.posts[value.ID] = *value
	return nil
}
func (r *communityMemoryRepo) FindPost(_ context.Context, id uint, _ bool) (Post, error) {
	value, ok := r.posts[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	return value, nil
}
func (r *communityMemoryRepo) ListPosts(context.Context, FeedQuery, bool) ([]Post, int64, error) {
	values := make([]Post, 0, len(r.posts))
	for _, v := range r.posts {
		values = append(values, v)
	}
	return values, int64(len(values)), nil
}
func (r *communityMemoryRepo) ListFollowingPosts(ctx context.Context, _ uint, q FeedQuery) ([]Post, int64, error) {
	return r.ListPosts(ctx, q, true)
}
func (r *communityMemoryRepo) CreateComment(_ context.Context, value *Comment) error {
	value.ID = uint(len(r.comments) + 1)
	r.comments = append(r.comments, *value)
	return nil
}
func (r *communityMemoryRepo) ListComments(_ context.Context, postID uint, _, _ int, _ bool) ([]Comment, int64, error) {
	values := []Comment{}
	for _, v := range r.comments {
		if v.PostID == postID {
			values = append(values, v)
		}
	}
	return values, int64(len(values)), nil
}
func (r *communityMemoryRepo) ToggleLike(context.Context, uint, uint) (bool, error) { return true, nil }
func (r *communityMemoryRepo) ToggleFavorite(context.Context, uint, uint) (bool, error) {
	return true, nil
}
func (r *communityMemoryRepo) ToggleFollow(context.Context, uint, uint) (bool, error) {
	r.followed = !r.followed
	return r.followed, nil
}
func (r *communityMemoryRepo) ListNotifications(_ context.Context, userID uint, _, _ int) ([]Notification, int64, int64, error) {
	values := []Notification{}
	for _, v := range r.notifications {
		if v.RecipientID == userID {
			values = append(values, v)
		}
	}
	return values, int64(len(values)), int64(len(values)), nil
}
func (r *communityMemoryRepo) MarkNotificationRead(_ context.Context, _ uint, id uint) error {
	r.readID = id
	return nil
}
func (r *communityMemoryRepo) SetReviewStatus(context.Context, string, uint, review.Status) error {
	return nil
}
func (r *communityMemoryRepo) AdminTakedownPost(_ context.Context, id uint) error {
	value, ok := r.posts[id]
	if !ok {
		return ErrNotFound
	}
	value.ReviewStatus = ReviewStatusTakenDown
	r.posts[id] = value
	return nil
}
func (r *communityMemoryRepo) Count(context.Context) (int64, int64, error) { return 0, 0, nil }

type profileDirectory map[uint]user.Profile

func (d profileDirectory) Profiles(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
	values := map[uint]user.Profile{}
	for _, id := range ids {
		if value, ok := d[id]; ok {
			values[id] = value
		}
	}
	return values, nil
}

type submissionRecorder struct{ submissions []review.Submission }

func (r *submissionRecorder) Submit(_ context.Context, value review.Submission) (review.Task, error) {
	r.submissions = append(r.submissions, value)
	return review.Task{ObjectType: value.ObjectType, ObjectID: value.ObjectID}, nil
}

type communityMediaFake struct{ committed, deleted bool }

func (f *communityMediaFake) Reserve(_ context.Context, _ uint, purpose media.Purpose, ids []string) (media.Reservation, error) {
	if purpose != media.PurposePostImage {
		return media.Reservation{}, errors.New("wrong purpose")
	}
	return media.Reservation{Token: "token", References: []media.Reference{{InternalID: 91, PublicID: ids[0], URL: "/api/v1/media/assets/asset/content"}}}, nil
}
func (f *communityMediaFake) Commit(context.Context, uint, media.Reservation, string, uint64) error {
	f.committed = true
	return nil
}
func (f *communityMediaFake) Cancel(context.Context, uint, media.Reservation) error { return nil }
func (f *communityMediaFake) DeleteReference(context.Context, string, uint64, []uint64) error {
	f.deleted = true
	return nil
}

func TestCommunityPostBindsOnlyMediaReferences(t *testing.T) {
	repo := &communityMemoryRepo{posts: map[uint]Post{}}
	gateway := &communityMediaFake{}
	service := NewService(repo, profileDirectory{}, &submissionRecorder{}, gateway)
	post, err := service.CreatePost(context.Background(), CreatePostInput{UserID: 7, Title: "title", Content: "content", ImageAssetIDs: []string{"asset"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !gateway.committed || len(post.Images) != 1 || post.Images[0].AssetID == nil || *post.Images[0].AssetID != 91 {
		t.Fatalf("media was not bound: %+v", post.Images)
	}
	if err := NewReviewTargets(service)[0].SetReviewStatus(context.Background(), post.ID, review.StatusRejected); err != nil || !gateway.deleted {
		t.Fatalf("rejected media not deleted: %v", err)
	}
}

func TestCommunityCommentFollowAndNotificationLoop(t *testing.T) {
	repo := &communityMemoryRepo{posts: map[uint]Post{1: {ID: 1, UserID: 10, ReviewStatus: review.StatusApproved}}, notifications: []Notification{{ID: 7, RecipientID: 10, ActorID: 20, Type: NotificationComment, PostID: 1}}}
	profiles := profileDirectory{10: {ID: 10, Nickname: "author"}, 20: {ID: 20, Nickname: "actor"}}
	reviews := &submissionRecorder{}
	service := NewService(repo, profiles, reviews)

	comment, err := service.AddComment(context.Background(), 1, 20, "hello", "")
	if err != nil || comment.ReviewStatus != review.StatusSubmitted {
		t.Fatalf("comment not submitted: %+v %v", comment, err)
	}
	if len(reviews.submissions) != 1 || reviews.submissions[0].ObjectType != "comment" {
		t.Fatalf("review not created: %+v", reviews.submissions)
	}
	if _, err := service.ToggleFollow(context.Background(), 20, 20); err == nil {
		t.Fatal("self-follow accepted")
	}
	followed, err := service.ToggleFollow(context.Background(), 20, 10)
	if err != nil || !followed {
		t.Fatalf("follow failed: %v %v", followed, err)
	}
	page, err := service.FollowingFeed(context.Background(), 20, FeedQuery{Page: 1, PageSize: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].Author.Nickname != "author" {
		t.Fatalf("following feed incomplete: %+v %v", page, err)
	}
	notifications, err := service.Notifications(context.Background(), 10, 1, 20)
	if err != nil || notifications.Unread != 1 || notifications.Items[0].Actor.Nickname != "actor" {
		t.Fatalf("notification enrichment failed: %+v %v", notifications, err)
	}
	if err := service.MarkNotificationRead(context.Background(), 10, 7); err != nil || repo.readID != 7 {
		t.Fatalf("mark read failed: %v", err)
	}
}

func TestCommunityLoopRejectsUnknownTargets(t *testing.T) {
	repo := &communityMemoryRepo{posts: map[uint]Post{}}
	service := NewService(repo, profileDirectory{}, &submissionRecorder{})
	if _, err := service.AddComment(context.Background(), 99, 1, "comment", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("comment accepted missing post: %v", err)
	}
	if _, err := service.ToggleFollow(context.Background(), 1, 2); err == nil {
		t.Fatal("follow accepted missing user")
	}
}
