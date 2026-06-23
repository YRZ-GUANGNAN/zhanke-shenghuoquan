package community

import (
	"context"
	"errors"
	"strings"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/media"
)

type UserDirectory interface {
	Profiles(context.Context, []uint) (map[uint]user.Profile, error)
}

type Service struct {
	repo    Repository
	users   UserDirectory
	reviews review.Submitter
	media   media.Gateway
}

func NewService(repo Repository, users UserDirectory, reviews review.Submitter, mediaGateway ...media.Gateway) *Service {
	var gateway media.Gateway
	if len(mediaGateway) > 0 {
		gateway = mediaGateway[0]
	}
	return &Service{repo: repo, users: users, reviews: reviews, media: gateway}
}

type CreatePostInput struct {
	UserID                uint
	Title, Content, Topic string
	ImageAssetIDs         []string
}
type PostView struct {
	Post
	Author user.Profile `json:"author"`
}
type CommentView struct {
	Comment
	Author user.Profile `json:"author"`
}
type NotificationView struct {
	Notification
	Actor user.Profile `json:"actor"`
}
type NotificationPage struct {
	Items    []NotificationView `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
	Unread   int64              `json:"unread"`
}
type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

func (s *Service) CreatePost(ctx context.Context, input CreatePostInput, risk string) (Post, error) {
	if input.Title == "" || input.Content == "" {
		return Post{}, errors.New("title and content are required")
	}
	reservation := media.Reservation{}
	var err error
	if len(input.ImageAssetIDs) > 0 {
		if s.media == nil {
			return Post{}, errors.New("media pipeline is unavailable")
		}
		reservation, err = s.media.Reserve(ctx, input.UserID, media.PurposePostImage, input.ImageAssetIDs)
		if err != nil {
			return Post{}, err
		}
	}
	value := Post{UserID: input.UserID, Title: input.Title, Content: input.Content, Topic: fallback(input.Topic, "校园生活"), ReviewStatus: review.StatusSubmitted}
	for i, ref := range reservation.References {
		assetID := ref.InternalID
		value.Images = append(value.Images, PostImage{AssetID: &assetID, URL: ref.URL, Sort: i})
	}
	if err := s.repo.CreatePost(ctx, &value); err != nil {
		if s.media != nil && reservation.Token != "" {
			_ = s.media.Cancel(ctx, input.UserID, reservation)
		}
		return Post{}, err
	}
	if len(reservation.References) > 0 {
		if err := s.media.Commit(ctx, input.UserID, reservation, "post", uint64(value.ID)); err != nil {
			return Post{}, err
		}
	}
	if _, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "post", ObjectID: value.ID, SubmitterID: input.UserID, Risk: risk}); err != nil {
		return Post{}, err
	}
	return value, nil
}

func (s *Service) Feed(ctx context.Context, q FeedQuery) (Page[PostView], error) {
	q.Page, q.PageSize = normalize(q.Page, q.PageSize)
	values, total, err := s.repo.ListPosts(ctx, q, true)
	if err != nil {
		return Page[PostView]{}, err
	}
	views, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: views, Page: q.Page, PageSize: q.PageSize, Total: total}, err
}
func (s *Service) FollowingFeed(ctx context.Context, userID uint, q FeedQuery) (Page[PostView], error) {
	q.Page, q.PageSize = normalize(q.Page, q.PageSize)
	values, total, err := s.repo.ListFollowingPosts(ctx, userID, q)
	if err != nil {
		return Page[PostView]{}, err
	}
	views, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: views, Page: q.Page, PageSize: q.PageSize, Total: total}, err
}
func (s *Service) Post(ctx context.Context, id uint) (PostView, error) {
	value, err := s.repo.FindPost(ctx, id, true)
	if err != nil {
		return PostView{}, err
	}
	views, err := s.enrichPosts(ctx, []Post{value})
	if err != nil {
		return PostView{}, err
	}
	return views[0], nil
}
func (s *Service) AddComment(ctx context.Context, postID, userID uint, content, risk string) (Comment, error) {
	if content == "" {
		return Comment{}, errors.New("comment is required")
	}
	if _, err := s.repo.FindPost(ctx, postID, true); err != nil {
		return Comment{}, err
	}
	value := Comment{PostID: postID, UserID: userID, Content: content, ReviewStatus: review.StatusSubmitted}
	if err := s.repo.CreateComment(ctx, &value); err != nil {
		return Comment{}, err
	}
	if _, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "comment", ObjectID: value.ID, SubmitterID: userID, Risk: risk}); err != nil {
		return Comment{}, err
	}
	return value, nil
}
func (s *Service) Comments(ctx context.Context, postID uint, page, size int) (Page[CommentView], error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.ListComments(ctx, postID, page, size, true)
	if err != nil {
		return Page[CommentView]{}, err
	}
	ids := make([]uint, 0, len(values))
	for _, v := range values {
		ids = append(ids, v.UserID)
	}
	profiles, err := s.users.Profiles(ctx, ids)
	if err != nil {
		return Page[CommentView]{}, err
	}
	views := make([]CommentView, 0, len(values))
	for _, v := range values {
		views = append(views, CommentView{Comment: v, Author: profiles[v.UserID]})
	}
	return Page[CommentView]{Items: views, Page: page, PageSize: size, Total: total}, nil
}
func (s *Service) ToggleLike(ctx context.Context, postID, userID uint) (bool, error) {
	if _, err := s.repo.FindPost(ctx, postID, true); err != nil {
		return false, err
	}
	return s.repo.ToggleLike(ctx, postID, userID)
}
func (s *Service) ToggleFavorite(ctx context.Context, postID, userID uint) (bool, error) {
	if _, err := s.repo.FindPost(ctx, postID, true); err != nil {
		return false, err
	}
	return s.repo.ToggleFavorite(ctx, postID, userID)
}
func (s *Service) ToggleFollow(ctx context.Context, followerID, followeeID uint) (bool, error) {
	if followerID == followeeID {
		return false, errors.New("cannot follow yourself")
	}
	profiles, err := s.users.Profiles(ctx, []uint{followeeID})
	if err != nil {
		return false, err
	}
	if _, ok := profiles[followeeID]; !ok {
		return false, errors.New("user not found")
	}
	return s.repo.ToggleFollow(ctx, followerID, followeeID)
}
func (s *Service) Notifications(ctx context.Context, userID uint, page, size int) (NotificationPage, error) {
	page, size = normalize(page, size)
	values, total, unread, err := s.repo.ListNotifications(ctx, userID, page, size)
	if err != nil {
		return NotificationPage{}, err
	}
	ids := make([]uint, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ActorID)
	}
	profiles, err := s.users.Profiles(ctx, ids)
	if err != nil {
		return NotificationPage{}, err
	}
	views := make([]NotificationView, 0, len(values))
	for _, value := range values {
		views = append(views, NotificationView{Notification: value, Actor: profiles[value.ActorID]})
	}
	return NotificationPage{Items: views, Page: page, PageSize: size, Total: total, Unread: unread}, nil
}
func (s *Service) MarkNotificationRead(ctx context.Context, userID, id uint) error {
	return s.repo.MarkNotificationRead(ctx, userID, id)
}
func (s *Service) AdminList(ctx context.Context, page, size int) (Page[PostView], error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.ListPosts(ctx, FeedQuery{Page: page, PageSize: size}, false)
	if err != nil {
		return Page[PostView]{}, err
	}
	views, err := s.enrichPosts(ctx, values)
	return Page[PostView]{Items: views, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) AdminTakedown(ctx context.Context, postID, operatorID uint, reason string) error {
	if postID == 0 || operatorID == 0 || strings.TrimSpace(reason) == "" {
		return errors.New("post id, operator and takedown reason are required")
	}
	return s.repo.AdminTakedownPost(ctx, postID)
}

func (s *Service) Stats(ctx context.Context) (int64, int64, error) { return s.repo.Count(ctx) }
func (s *Service) enrichPosts(ctx context.Context, values []Post) ([]PostView, error) {
	ids := make([]uint, 0, len(values))
	for _, v := range values {
		ids = append(ids, v.UserID)
	}
	profiles, err := s.users.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	views := make([]PostView, 0, len(values))
	for _, v := range values {
		views = append(views, PostView{Post: v, Author: profiles[v.UserID]})
	}
	return views, nil
}

type ReviewTarget struct {
	service    *Service
	objectType string
}

func NewReviewTargets(service *Service) []review.Target {
	return []review.Target{ReviewTarget{service, "post"}, ReviewTarget{service, "comment"}}
}
func (t ReviewTarget) ObjectType() string { return t.objectType }
func (t ReviewTarget) SetReviewStatus(ctx context.Context, id uint, status review.Status) error {
	if err := t.service.repo.SetReviewStatus(ctx, t.objectType, id, status); err != nil {
		return err
	}
	if t.objectType == "post" && status == review.StatusRejected && t.service.media != nil {
		return t.service.media.DeleteReference(ctx, "post", uint64(id), nil)
	}
	return nil
}

func fallback(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
func normalize(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
