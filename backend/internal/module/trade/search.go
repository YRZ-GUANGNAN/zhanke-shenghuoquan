package trade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	meilisearch "github.com/meilisearch/meilisearch-go"
)

var ErrSearchUnavailable = errors.New("product search is unavailable")

type SearchGateway interface {
	Configure(context.Context) error
	Search(context.Context, string, ListQuery) ([]uint, int64, error)
	Upsert(context.Context, Product) error
	Delete(context.Context, uint) error
	Close()
}

type MeiliSearch struct {
	client meilisearch.ServiceManager
	index  meilisearch.IndexManager
	name   string
}

func (m *MeiliSearch) Ready(ctx context.Context) error {
	if m == nil || m.client == nil {
		return ErrSearchUnavailable
	}
	health, err := m.client.HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}
	if health == nil || health.Status != "available" {
		return ErrSearchUnavailable
	}
	return nil
}

func NewMeiliSearch(host, apiKey, indexName string) (*MeiliSearch, error) {
	if host == "" || apiKey == "" {
		return nil, errors.New("MEILI_HOST and MEILI_MASTER_KEY are required")
	}
	if indexName == "" {
		indexName = "products"
	}
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey), meilisearch.WithCustomRetries([]int{429, 500, 502, 503, 504}, 2))
	return &MeiliSearch{client: client, index: client.Index(indexName), name: indexName}, nil
}

func (m *MeiliSearch) Configure(ctx context.Context) error {
	if _, err := m.client.GetIndexWithContext(ctx, m.name); err != nil {
		task, createErr := m.client.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{Uid: m.name, PrimaryKey: "id"})
		if createErr != nil {
			return fmt.Errorf("create Meilisearch index: %w", createErr)
		}
		if err := m.wait(ctx, task); err != nil {
			return err
		}
	}
	searchable := []string{"title", "description"}
	filterable := []interface{}{"status", "reviewStatus", "category", "campus", "sellerId"}
	sortable := []string{"createdAt", "updatedAt", "price", "favoriteCount"}
	for _, update := range []func() (*meilisearch.TaskInfo, error){
		func() (*meilisearch.TaskInfo, error) {
			return m.index.UpdateSearchableAttributesWithContext(ctx, &searchable)
		},
		func() (*meilisearch.TaskInfo, error) {
			return m.index.UpdateFilterableAttributesWithContext(ctx, &filterable)
		},
		func() (*meilisearch.TaskInfo, error) {
			return m.index.UpdateSortableAttributesWithContext(ctx, &sortable)
		},
	} {
		task, err := update()
		if err != nil {
			return fmt.Errorf("configure Meilisearch index: %w", err)
		}
		if err := m.wait(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (m *MeiliSearch) Search(ctx context.Context, keyword string, q ListQuery) ([]uint, int64, error) {
	filters := []string{"reviewStatus = \"approved\""}
	if q.Status != "" {
		filters = append(filters, "status = "+strconv.Quote(string(q.Status)))
	}
	if q.Category != "" {
		filters = append(filters, "category = "+strconv.Quote(q.Category))
	}
	result, err := m.index.SearchWithContext(ctx, keyword, &meilisearch.SearchRequest{
		Offset: int64((q.Page - 1) * q.PageSize), Limit: int64(q.PageSize),
		AttributesToRetrieve: []string{"id"}, AttributesToSearchOn: []string{"title", "description"},
		Filter: filters, Sort: []string{"createdAt:desc"},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}
	ids := make([]uint, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var id uint
		if raw, ok := hit["id"]; ok && json.Unmarshal(raw, &id) == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, result.EstimatedTotalHits, nil
}

func (m *MeiliSearch) Upsert(ctx context.Context, value Product) error {
	document := []map[string]any{{
		"id": value.ID, "title": value.Title, "description": value.Description,
		"category": value.Category, "campus": value.Campus, "condition": value.Condition,
		"status": value.Status, "reviewStatus": value.ReviewStatus, "sellerId": value.SellerID,
		"price": value.Price, "favoriteCount": value.FavoriteCount,
		"createdAt": value.CreatedAt.UnixMilli(), "updatedAt": value.UpdatedAt.UnixMilli(),
	}}
	task, err := m.index.AddDocumentsWithContext(ctx, document, nil)
	if err != nil {
		return err
	}
	return m.wait(ctx, task)
}

func (m *MeiliSearch) Delete(ctx context.Context, id uint) error {
	task, err := m.index.DeleteDocumentWithContext(ctx, strconv.FormatUint(uint64(id), 10), nil)
	if err != nil {
		return err
	}
	return m.wait(ctx, task)
}

func (m *MeiliSearch) wait(ctx context.Context, info *meilisearch.TaskInfo) error {
	task, err := m.client.WaitForTaskWithContext(ctx, info.TaskUID, 25*time.Millisecond)
	if err != nil {
		return err
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		return fmt.Errorf("Meilisearch task %d ended with %s", task.UID, task.Status)
	}
	return nil
}

func (m *MeiliSearch) Close() { m.client.Close() }
