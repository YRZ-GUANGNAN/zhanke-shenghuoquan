package media

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestPostVideoCOSSubmissionCoversFullClipAndAudio(t *testing.T) {
	asset := videoReviewAsset()
	key, version := "final/test-video.mp4", "immutable-version"
	asset.FinalKey, asset.FinalVersionID, asset.DurationMS = &key, &version, 60000
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/video/auditing" {
			t.Errorf("unexpected COS request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var request cos.PutVideoAuditingJobOptions
		if err := xml.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Conf == nil || request.Conf.Snapshot == nil || request.Conf.Snapshot.Count < 60 || request.Conf.Snapshot.TimeInterval != 1 || request.Conf.Snapshot.Mode != "Interval" || request.Conf.DetectContent != 1 {
			t.Errorf("video review must include the required frame count, entire clip and audio: %+v", request.Conf)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mediaURL, err := url.Parse(request.InputUrl)
		if err != nil || mediaURL.Query().Get("versionId") != version || request.InputDataId != asset.AssetPublicID || request.Conf.BizType != "verified-policy" {
			t.Error("video review lost immutable media identity or configured policy")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte("<Response><JobsDetail><JobId>real-provider-job</JobId><State>Submitted</State></JobsDetail></Response>"))
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	store := &TencentCompletionStore{secretID: "test-id", secretKey: "test-key", cosClient: cos.NewClient(&cos.BaseURL{BucketURL: base, CIURL: base}, server.Client())}
	provider, err := NewTencentVideoReview(store, "verified-policy")
	if err != nil {
		t.Fatal(err)
	}
	job, err := provider.SubmitVideo(context.Background(), asset)
	if err != nil || job != "real-provider-job" {
		t.Fatalf("COS submission: job=%q error=%v", job, err)
	}
}
