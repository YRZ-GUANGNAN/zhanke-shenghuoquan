package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

var ErrInvalidComment = errors.New("comment must contain 1 to 1000 characters")

type CommentLikeState struct {
	Liked      bool  `json:"liked"`
	LikedCount int64 `json:"likedCount"`
}

type CommentMetadata struct {
	Liked      bool
	ReplyCount int64
}

type commentThreadRepository interface {
	FindThreadComment(context.Context, uint) (Comment, error)
	ListCommentLevel(context.Context, uint, uint, uint, int, int) ([]Comment, int64, error)
	CommentMetadata(context.Context, uint, []uint, uint) (map[uint]CommentMetadata, error)
	SetCommentLiked(context.Context, uint, uint, uint, bool) (CommentLikeState, error)
}

type ThreadCommentView struct {
	CommentView
	ReplyTo    *user.Profile `json:"replyTo,omitempty"`
	Liked      bool          `json:"liked"`
	ReplyCount int64         `json:"replyCount"`
}

func visibleThreadComment(value Comment, viewerID uint) bool {
	return value.ReviewStatus == review.StatusApproved || (viewerID != 0 && value.UserID == viewerID &&
		(value.ReviewStatus == review.StatusSubmitted || value.ReviewStatus == review.StatusReviewing || value.ReviewStatus == review.StatusRejected))
}

func (s *Service) threadRepository() (commentThreadRepository, error) {
	repo, ok := s.repo.(commentThreadRepository)
	if !ok {
		return nil, ErrWriteContractUnavailable
	}
	return repo, nil
}

func (s *Service) CommentThreads(ctx context.Context, postID, viewerID uint, page, size int) (Page[ThreadCommentView], error) {
	return s.commentLevel(ctx, postID, 0, viewerID, page, size)
}

func (s *Service) CommentReplies(ctx context.Context, postID, rootID, viewerID uint, page, size int) (Page[ThreadCommentView], error) {
	if rootID == 0 {
		return Page[ThreadCommentView]{}, ErrNotFound
	}
	return s.commentLevel(ctx, postID, rootID, viewerID, page, size)
}

func (s *Service) commentLevel(ctx context.Context, postID, rootID, viewerID uint, page, size int) (Page[ThreadCommentView], error) {
	repo, err := s.threadRepository()
	if err != nil {
		return Page[ThreadCommentView]{}, err
	}
	post, err := s.repo.FindPost(ctx, postID, true)
	if err != nil {
		return Page[ThreadCommentView]{}, err
	}
	if rootID != 0 {
		root, err := repo.FindThreadComment(ctx, rootID)
		if err != nil {
			return Page[ThreadCommentView]{}, err
		}
		if root.PostID != postID || root.ParentCommentID != 0 || !visibleThreadComment(root, viewerID) {
			return Page[ThreadCommentView]{}, ErrNotFound
		}
	}
	page, size = normalize(page, size)
	values, total, err := repo.ListCommentLevel(ctx, postID, rootID, viewerID, page, size)
	if err != nil {
		return Page[ThreadCommentView]{}, err
	}
	ids, users := make([]uint, 0, len(values)), make([]uint, 0, len(values)*2)
	for _, value := range values {
		ids = append(ids, value.ID)
		users = append(users, value.UserID)
		if value.ReplyToUserID != 0 {
			users = append(users, value.ReplyToUserID)
		}
	}
	profiles, err := s.users.Profiles(ctx, users)
	if err != nil {
		return Page[ThreadCommentView]{}, err
	}
	metadata, err := repo.CommentMetadata(ctx, postID, ids, viewerID)
	if err != nil {
		return Page[ThreadCommentView]{}, err
	}
	views := make([]ThreadCommentView, 0, len(values))
	for _, value := range values {
		item := ThreadCommentView{CommentView: CommentView{Comment: value, Author: profiles[value.UserID]}, Liked: metadata[value.ID].Liked, ReplyCount: metadata[value.ID].ReplyCount}
		if profile, found := profiles[value.ReplyToUserID]; found {
			item.ReplyTo = &profile
		}
		views = append(views, item)
	}
	return Page[ThreadCommentView]{Items: views, Page: page, PageSize: size, Total: total, PublicCount: &post.CommentCount}, nil
}

func (s *Service) ReplyCommentKeyed(ctx context.Context, postID, targetID, userID uint, content, key, risk string) (KeyedCreateResult[Comment], error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 1000 {
		return KeyedCreateResult[Comment]{}, ErrInvalidComment
	}
	if !validWriteContractULID(key) {
		return KeyedCreateResult[Comment]{}, ErrWriteContractInvalid
	}
	repo, err := s.threadRepository()
	if err != nil {
		return KeyedCreateResult[Comment]{}, err
	}
	if s.writeRepo == nil {
		return KeyedCreateResult[Comment]{}, ErrWriteContractUnavailable
	}
	if _, err = s.repo.FindPost(ctx, postID, true); err != nil {
		return KeyedCreateResult[Comment]{}, err
	}
	target, err := repo.FindThreadComment(ctx, targetID)
	if err != nil {
		return KeyedCreateResult[Comment]{}, err
	}
	if target.PostID != postID || target.ReviewStatus != review.StatusApproved {
		return KeyedCreateResult[Comment]{}, ErrNotFound
	}
	rootID := target.ID
	if target.ParentCommentID != 0 {
		rootID = target.ParentCommentID
		root, err := repo.FindThreadComment(ctx, rootID)
		if err != nil {
			return KeyedCreateResult[Comment]{}, err
		}
		if root.PostID != postID || root.ParentCommentID != 0 || root.ReviewStatus != review.StatusApproved {
			return KeyedCreateResult[Comment]{}, ErrNotFound
		}
	}
	hash, err := canonicalCommunityHash(struct {
		Version  string `json:"version"`
		PostID   uint   `json:"postId"`
		RootID   uint   `json:"rootId"`
		TargetID uint   `json:"targetId"`
		Content  string `json:"content"`
	}{"community-comment-reply:v1", postID, rootID, targetID, content})
	if err != nil {
		return KeyedCreateResult[Comment]{}, err
	}
	if old, found, err := s.writeRepo.FindCommentByRequest(ctx, userID, key); err != nil {
		return KeyedCreateResult[Comment]{}, err
	} else if found {
		if !bytes.Equal(old.CreateRequestHash, hash) {
			return KeyedCreateResult[Comment]{}, ErrIdempotencyConflict
		}
		return KeyedCreateResult[Comment]{Value: old, Replayed: true}, nil
	}
	value := Comment{PostID: postID, UserID: userID, ParentCommentID: rootID, ReplyToCommentID: targetID, ReplyToUserID: target.UserID,
		Content: content, ReviewStatus: review.StatusSubmitted, ClientRequestID: &key, CreateRequestHash: hash}
	payload, err := json.Marshal(communityCreationPayload{Version: "community-comment-created:v1", ReferenceIDSource: "aggregateId", OrderedAssetPublicIDs: []string{}, ReviewTargetType: "comment", ReviewRisk: risk})
	if err != nil {
		return KeyedCreateResult[Comment]{}, err
	}
	replayed, err := s.writeRepo.CreateCommentOrReplay(ctx, &value, &WriteOutbox{EventType: "community_comment_created", Payload: string(payload)})
	return KeyedCreateResult[Comment]{Value: value, Replayed: replayed}, err
}

func (s *Service) SetCommentLiked(ctx context.Context, postID, commentID, userID uint, desired bool) (CommentLikeState, error) {
	repo, err := s.threadRepository()
	if err != nil {
		return CommentLikeState{}, err
	}
	return repo.SetCommentLiked(ctx, postID, commentID, userID, desired)
}
