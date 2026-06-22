package mediaconsole

import (
	"context"
	"errors"
	"testing"
	"time"
)

type queryRepositoryStub struct {
	err        error
	lastFilter ListFilter
}

func (r *queryRepositoryStub) Summary(context.Context) (Summary, error) {
	return Summary{Assets: 1}, r.err
}
func (r *queryRepositoryStub) ListAssets(_ context.Context, filter ListFilter) ([]AssetRow, int64, error) {
	r.lastFilter = filter
	return []AssetRow{{AssetPublicID: "01KVMEDIA00000000000000000"}}, 1, r.err
}
func (r *queryRepositoryStub) AssetDetail(context.Context, string) (AssetDetail, error) {
	return AssetDetail{}, r.err
}
func (r *queryRepositoryStub) ListDecisions(_ context.Context, filter ListFilter) ([]DecisionRow, int64, error) {
	r.lastFilter = filter
	return nil, 0, r.err
}
func (r *queryRepositoryStub) ListEvents(_ context.Context, filter ListFilter) ([]EventRow, int64, error) {
	r.lastFilter = filter
	return nil, 0, r.err
}
func (r *queryRepositoryStub) ListGCJobs(_ context.Context, filter ListFilter) ([]GCJobRow, int64, error) {
	r.lastFilter = filter
	return nil, 0, r.err
}

func TestQueryServiceValidatesAndCapsFilters(t *testing.T) {
	repository := &queryRepositoryStub{}
	service, err := NewQueryService(repository)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Assets(context.Background(), ListFilter{PageSize: 500, Purpose: "post", Status: "final_ready"})
	if err != nil || page.Page != 1 || page.PageSize != 100 || repository.lastFilter.PageSize != 100 {
		t.Fatalf("page=%#v filter=%#v err=%v", page, repository.lastFilter, err)
	}
	for _, filter := range []ListFilter{
		{AssetID: "../invalid"}, {Status: "drop table"}, {Purpose: "video"},
		{From: timePointerForTest(time.Now()), To: timePointerForTest(time.Now().Add(-time.Hour))},
	} {
		if _, err := service.Assets(context.Background(), filter); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("invalid filter %#v returned %v", filter, err)
		}
	}
}

func TestQueryServiceContextSpecificStatusAllowlist(t *testing.T) {
	service, _ := NewQueryService(&queryRepositoryStub{})
	if _, err := service.Events(context.Background(), ListFilter{Status: "delete_pending"}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("asset status accepted for event query: %v", err)
	}
	if _, err := service.GCJobs(context.Background(), ListFilter{Kind: "prefix-delete"}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("unsafe GC kind accepted: %v", err)
	}
	if _, err := service.Decisions(context.Background(), ListFilter{Status: "reviewing"}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("non-decision status accepted: %v", err)
	}
}

func timePointerForTest(value time.Time) *time.Time { return &value }

var _ QueryRepository = (*queryRepositoryStub)(nil)
