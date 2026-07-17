package navigation

import (
	"context"
	"errors"
	"strings"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service                             { return &Service{repo: repo} }
func (s *Service) PublicTree(ctx context.Context) ([]Category, error) { return s.repo.Tree(ctx, true) }
func (s *Service) AdminTree(ctx context.Context) ([]Category, error)  { return s.repo.Tree(ctx, false) }
func (s *Service) CreateCategory(ctx context.Context, v Category) (Category, error) {
	if strings.TrimSpace(v.Name) == "" {
		return Category{}, errors.New("category name is required")
	}
	v.ID = 0
	v.SystemKey = nil
	if err := s.repo.CreateCategory(ctx, &v); err != nil {
		return Category{}, err
	}
	return v, nil
}
func (s *Service) UpdateCategory(ctx context.Context, v Category) error {
	if v.ID == 0 || strings.TrimSpace(v.Name) == "" {
		return errors.New("invalid category")
	}
	return s.repo.UpdateCategory(ctx, v.ID, map[string]any{
		"name":    v.Name,
		"sort":    v.Sort,
		"enabled": v.Enabled,
	})
}
func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	return s.repo.DeleteCategory(ctx, id)
}
func (s *Service) CreateItem(ctx context.Context, v Item) (Item, error) {
	if err := validateItem(v); err != nil {
		return Item{}, err
	}
	v.ID = 0
	v.OperatingStatus = OperatingStatusOpen
	if err := s.repo.CreateItem(ctx, &v); err != nil {
		return Item{}, err
	}
	return v, nil
}
func (s *Service) UpdateItem(ctx context.Context, v Item) error {
	if v.ID == 0 {
		return errors.New("item id is required")
	}
	if err := validateItem(v); err != nil {
		return err
	}
	return s.repo.UpdateItem(ctx, v.ID, map[string]any{
		"category_id": v.CategoryID,
		"title":       v.Title,
		"subtitle":    v.Subtitle,
		"icon_text":   v.IconText,
		"app_id":      v.AppID,
		"path":        v.Path,
		"color":       v.Color,
		"sort":        v.Sort,
		"enabled":     v.Enabled,
	})
}
func (s *Service) DeleteItem(ctx context.Context, id uint) error { return s.repo.DeleteItem(ctx, id) }
func validateItem(v Item) error {
	if v.CategoryID == 0 || strings.TrimSpace(v.Title) == "" {
		return errors.New("category and title are required")
	}
	if strings.TrimSpace(v.AppID) == "" || strings.TrimSpace(v.Path) == "" {
		return errors.New("appid and path are required")
	}
	return nil
}
