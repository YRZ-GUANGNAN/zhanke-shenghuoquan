package media

import (
	"context"
	"errors"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	"math"
	"strconv"
	"strings"
	"time"
)

var ErrVideoUnavailable = errors.New("video media service is unavailable")

type VideoInfo struct {
	Width, Height, DurationMS uint32
	Size                      uint64
	Codec                     string
}
type VideoInspector interface {
	InspectVideo(context.Context, string, string) (VideoInfo, error)
}
type VideoReviewResult struct {
	JobID, AssetPublicID, State string
	Result                      int
}
type VideoReviewProvider interface {
	SubmitVideo(context.Context, Asset) (string, error)
	QueryVideo(context.Context, string) (VideoReviewResult, error)
}

func (s *TencentCompletionStore) InspectVideo(ctx context.Context, key, version string) (VideoInfo, error) {
	if key == "" || version == "" {
		return VideoInfo{}, ErrCompletionInvariant
	}
	r, _, err := s.cosClient.CI.GetMediaInfo(ctx, key, nil, version)
	if err != nil {
		return VideoInfo{}, err
	}
	if r == nil || r.MediaInfo == nil || len(r.MediaInfo.Stream.Video) != 1 {
		return VideoInfo{}, ErrCompletionValidation
	}
	m := r.MediaInfo
	v := m.Stream.Video[0]
	duration, err := strconv.ParseFloat(m.Format.Duration, 64)
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration < .5 || duration > 60 {
		return VideoInfo{}, ErrCompletionValidation
	}
	width, e1 := strconv.ParseUint(v.Width, 10, 32)
	height, e2 := strconv.ParseUint(v.Height, 10, 32)
	size, e3 := strconv.ParseUint(m.Format.Size, 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || !strings.Contains(m.Format.FormatName, "mp4") {
		return VideoInfo{}, ErrCompletionValidation
	}
	for _, a := range m.Stream.Audio {
		if a.CodecName != "aac" {
			return VideoInfo{}, ErrCompletionValidation
		}
	}
	return VideoInfo{Width: uint32(width), Height: uint32(height), Size: size, Codec: v.CodecName, DurationMS: uint32(math.Ceil(duration * 1000))}, nil
}

type TencentVideoReview struct {
	store   *TencentCompletionStore
	bizType string
}

func NewTencentVideoReview(store *TencentCompletionStore, bizType string) (*TencentVideoReview, error) {
	if store == nil {
		return nil, ErrVideoUnavailable
	}
	return &TencentVideoReview{store: store, bizType: strings.TrimSpace(bizType)}, nil
}
func (s *TencentVideoReview) SubmitVideo(ctx context.Context, asset Asset) (string, error) {
	if asset.Purpose != PurposePost || asset.VerifiedMIME != "video/mp4" || asset.FinalKey == nil || asset.FinalVersionID == nil {
		return "", ErrReviewInvariant
	}
	signed, err := s.store.SignedGetURL(ctx, *asset.FinalKey, *asset.FinalVersionID, time.Hour)
	if err != nil {
		return "", err
	}
	r, _, err := s.store.cosClient.CI.PutVideoAuditingJob(ctx, &cos.PutVideoAuditingJobOptions{
		InputUrl: signed, InputDataId: asset.AssetPublicID,
		Conf: &cos.VideoAuditingJobConf{BizType: s.bizType, DetectContent: 1,
			// CI requires Count even in Interval mode. Cover the complete allowed
			// 60-second clip, including its initial frame, rather than a preview.
			Snapshot: &cos.PutVideoAuditingJobSnapshot{Mode: "Interval", TimeInterval: 1, Count: 61}},
	})
	if err != nil {
		return "", err
	}
	if r == nil || r.JobsDetail.JobId == "" {
		return "", ErrReviewInvariant
	}
	return r.JobsDetail.JobId, nil
}
func (s *TencentVideoReview) QueryVideo(ctx context.Context, id string) (VideoReviewResult, error) {
	r, _, err := s.store.cosClient.CI.GetVideoAuditingJob(ctx, id)
	if err != nil {
		return VideoReviewResult{}, err
	}
	if r == nil || r.JobsDetail == nil || r.JobsDetail.JobId != id {
		return VideoReviewResult{}, ErrReviewInvariant
	}
	d := r.JobsDetail
	return VideoReviewResult{JobID: d.JobId, AssetPublicID: d.DataId, State: d.State, Result: d.Result}, nil
}
