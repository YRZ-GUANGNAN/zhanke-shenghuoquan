package media

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type videoJobMemory struct {
	mu  sync.Mutex
	job VideoReviewJob
}

func (r *videoJobMemory) WithVideoJob(_ context.Context, id string, now time.Time, fn func(*VideoReviewJob) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.job
	if job.AssetPublicID == "" {
		job = VideoReviewJob{AssetPublicID: id, Decision: ProjectionPending, CreatedAt: now, NextPollAt: now}
	}
	if err := fn(&job); err != nil {
		return err
	}
	r.job = job
	return nil
}

type videoProviderFake struct {
	submits, queries int
	err              error
	result           VideoReviewResult
}

func (p *videoProviderFake) SubmitVideo(context.Context, Asset) (string, error) {
	p.submits++
	return "video-job", p.err
}
func (p *videoProviderFake) QueryVideo(context.Context, string) (VideoReviewResult, error) {
	p.queries++
	return p.result, p.err
}
func videoReviewAsset() Asset {
	return Asset{AssetPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Purpose: PurposePost, VerifiedMIME: "video/mp4", TechnicalStatus: TechnicalFinalReady}
}

func TestVideoReviewSerializesSubmissionsAndKeepsTerminalDecision(t *testing.T) {
	asset, repo, provider := videoReviewAsset(), &videoJobMemory{}, &videoProviderFake{}
	provider.result = VideoReviewResult{JobID: "video-job", AssetPublicID: asset.AssetPublicID, State: "Success", Result: 0}
	s, _ := NewVideoReviewService(repo, provider)
	now := time.Now()
	s.now = func() time.Time { return now }
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ReviewVideo(context.Background(), asset); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if provider.submits != 1 || provider.queries != 0 {
		t.Fatalf("submits=%d queries=%d", provider.submits, provider.queries)
	}
	now = now.Add(6 * time.Second)
	result, err := s.ReviewVideo(context.Background(), asset)
	if err != nil || result.Decision != ProjectionApproved {
		t.Fatalf("decision: %+v %v", result, err)
	}
	provider.result.Result = 1
	if result, err = s.ReviewVideo(context.Background(), asset); err != nil || result.Decision != ProjectionApproved || provider.queries != 1 {
		t.Fatal("terminal decision changed or re-polled")
	}
}

func TestVideoReviewFailureRetainsRetryWindowAndNeverApprovesUnknown(t *testing.T) {
	asset, repo, provider := videoReviewAsset(), &videoJobMemory{}, &videoProviderFake{err: errors.New("unavailable")}
	s, _ := NewVideoReviewService(repo, provider)
	now := time.Now()
	s.now = func() time.Time { return now }
	if _, err := s.ReviewVideo(context.Background(), asset); err == nil {
		t.Fatal("submission failure hidden")
	}
	if _, err := s.ReviewVideo(context.Background(), asset); err != nil || provider.submits != 1 {
		t.Fatal("retry backoff ignored")
	}
	now = now.Add(31 * time.Minute)
	job, err := s.ReviewVideo(context.Background(), asset)
	if err != nil || job.Decision != ProjectionReviewFailed || provider.submits != 1 {
		t.Fatal("failed submission did not expire durably")
	}
	for _, v := range []VideoReviewResult{{State: "Unexpected"}, {State: "Success", Result: 7}, {State: "Success", Result: 1}, {State: "Success", Result: 2}} {
		if d, _ := videoDecision(v); d == ProjectionApproved {
			t.Fatalf("unsafe approval for %+v", v)
		}
	}
}

func TestVideoReviewRejectsProviderIdentityMismatch(t *testing.T) {
	asset, repo, provider := videoReviewAsset(), &videoJobMemory{}, &videoProviderFake{result: VideoReviewResult{JobID: "video-job", AssetPublicID: "wrong", State: "Success"}}
	s, _ := NewVideoReviewService(repo, provider)
	now := time.Now()
	s.now = func() time.Time { return now }
	_, _ = s.ReviewVideo(context.Background(), asset)
	now = now.Add(6 * time.Second)
	if _, err := s.ReviewVideo(context.Background(), asset); !errors.Is(err, ErrReviewInvariant) || repo.job.Decision != ProjectionPending {
		t.Fatalf("foreign decision applied: %v", err)
	}
}
