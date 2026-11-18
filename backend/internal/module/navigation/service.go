package navigation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidNavigationInput      = errors.New("invalid navigation input")
	ErrInvalidExternalTakeoutInput = errors.New("invalid external takeout merchant")
	ErrSystemManaged               = errors.New("navigation entry is system managed")
)

var iconPathPattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png)$`)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) PublicTree(ctx context.Context) ([]PublicCategory, error) {
	values, err := s.repo.Tree(ctx, true)
	if err != nil {
		return nil, err
	}
	return toPublicCategories(values), nil
}

func (s *Service) AdminTree(ctx context.Context) ([]Category, error) {
	return s.repo.Tree(ctx, false)
}

func (s *Service) CreateCategory(ctx context.Context, value Category) (Category, error) {
	if strings.TrimSpace(value.Name) == "" {
		return Category{}, invalidNavigationInput("category name is required")
	}
	value.ID = 0
	value.SystemKey = nil
	value.Items = nil
	if err := s.repo.CreateCategory(ctx, &value); err != nil {
		return Category{}, err
	}
	return value, nil
}

func (s *Service) UpdateCategory(ctx context.Context, value Category) error {
	if value.ID == 0 || strings.TrimSpace(value.Name) == "" {
		return invalidNavigationInput("valid category id and name are required")
	}
	existing, found, err := s.repo.FindCategory(ctx, value.ID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if systemManagedCategory(existing) {
		return ErrSystemManaged
	}
	return s.repo.UpdateCategory(ctx, value.ID, map[string]any{
		"name":    value.Name,
		"sort":    value.Sort,
		"enabled": value.Enabled,
	})
}

func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	if id == 0 {
		return invalidNavigationInput("category id is required")
	}
	existing, found, err := s.repo.FindCategory(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if systemManagedCategory(existing) {
		return ErrSystemManaged
	}
	return s.repo.DeleteCategory(ctx, id)
}

func (s *Service) CreateItem(ctx context.Context, value Item) (Item, error) {
	if err := validateItem(value); err != nil {
		return Item{}, invalidNavigationInput(err.Error())
	}
	targetCategory, found, err := s.repo.FindCategory(ctx, value.CategoryID)
	if err != nil {
		return Item{}, err
	}
	if !found {
		return Item{}, ErrNotFound
	}
	if itemCategoryLocked(targetCategory) {
		return Item{}, ErrSystemManaged
	}
	value.ID = 0
	value.OperatingStatus = OperatingStatusOpen
	if err := s.repo.CreateItem(ctx, &value); err != nil {
		return Item{}, err
	}
	return value, nil
}

func (s *Service) UpdateItem(ctx context.Context, value Item) error {
	if value.ID == 0 {
		return invalidNavigationInput("item id is required")
	}
	if err := validateItem(value); err != nil {
		return invalidNavigationInput(err.Error())
	}
	existing, found, err := s.repo.FindItem(ctx, value.ID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	existingCategory, found, err := s.repo.FindCategory(ctx, existing.CategoryID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if itemCategoryLocked(existingCategory) {
		return ErrSystemManaged
	}
	targetCategory, found, err := s.repo.FindCategory(ctx, value.CategoryID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if itemCategoryLocked(targetCategory) {
		return ErrSystemManaged
	}
	return s.repo.UpdateItem(ctx, value.ID, map[string]any{
		"category_id": value.CategoryID,
		"title":       value.Title,
		"subtitle":    value.Subtitle,
		"icon_text":   value.IconText,
		"app_id":      value.AppID,
		"path":        value.Path,
		"color":       value.Color,
		"sort":        value.Sort,
		"enabled":     value.Enabled,
	})
}

// SetItemIconPath binds (or clears when empty) the custom icon of one item.
// Icon files themselves are managed by the upload endpoints.
func (s *Service) SetItemIconPath(ctx context.Context, id uint, iconPath string) (Item, error) {
	if id == 0 {
		return Item{}, invalidNavigationInput("item id is required")
	}
	iconPath = strings.TrimSpace(iconPath)
	if iconPath != "" && !iconPathPattern.MatchString(iconPath) {
		return Item{}, invalidNavigationInput("invalid icon path")
	}
	existing, found, err := s.repo.FindItem(ctx, id)
	if err != nil {
		return Item{}, err
	}
	if !found {
		return Item{}, ErrNotFound
	}
	if err := s.repo.UpdateItem(ctx, id, map[string]any{"icon_path": iconPath}); err != nil {
		return Item{}, err
	}
	existing.IconPath = iconPath
	return existing, nil
}

func (s *Service) DeleteItem(ctx context.Context, id uint) error {
	if id == 0 {
		return invalidNavigationInput("item id is required")
	}
	existing, found, err := s.repo.FindItem(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	category, found, err := s.repo.FindCategory(ctx, existing.CategoryID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if itemCategoryLocked(category) {
		return ErrSystemManaged
	}
	return s.repo.DeleteItemInCategory(ctx, id, existing.CategoryID)
}

func (s *Service) ExternalTakeoutDirectory(ctx context.Context) (ExternalTakeoutDirectory, error) {
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return ExternalTakeoutDirectory{}, err
	}
	items, err := s.repo.ListItemsByCategory(ctx, category.ID)
	if err != nil {
		return ExternalTakeoutDirectory{}, err
	}
	values := make([]ExternalTakeoutMerchant, 0, len(items))
	for _, item := range items {
		values = append(values, toExternalTakeoutMerchant(item))
	}
	return ExternalTakeoutDirectory{CategoryEnabled: category.Enabled, Items: values}, nil
}

func (s *Service) CreateExternalTakeoutMerchant(ctx context.Context, input ExternalTakeoutMerchantInput) (ExternalTakeoutMerchant, error) {
	normalized, err := normalizeExternalTakeoutMerchantInput(input)
	if err != nil {
		return ExternalTakeoutMerchant{}, invalidExternalTakeoutInput(err.Error())
	}
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	duplicate, err := s.repo.DuplicateTarget(ctx, category.ID, normalized.AppID, normalized.Path, 0)
	if err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	if duplicate {
		return ExternalTakeoutMerchant{}, ErrDuplicateTarget
	}
	item := Item{
		CategoryID:      category.ID,
		Title:           normalized.Title,
		Subtitle:        normalized.Subtitle,
		IconText:        normalized.IconText,
		AppID:           normalized.AppID,
		Path:            normalized.Path,
		Color:           normalized.Color,
		Sort:            normalized.Sort,
		Enabled:         true,
		OperatingStatus: OperatingStatusOpen,
	}
	if err := s.repo.CreateItem(ctx, &item); err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	return toExternalTakeoutMerchant(item), nil
}

func (s *Service) UpdateExternalTakeoutMerchant(ctx context.Context, id uint, input ExternalTakeoutMerchantInput) (ExternalTakeoutMerchant, error) {
	if id == 0 {
		return ExternalTakeoutMerchant{}, invalidExternalTakeoutInput("merchant id is required")
	}
	normalized, err := normalizeExternalTakeoutMerchantInput(input)
	if err != nil {
		return ExternalTakeoutMerchant{}, invalidExternalTakeoutInput(err.Error())
	}
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	item, err := s.externalTakeoutItem(ctx, category.ID, id)
	if err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	duplicate, err := s.repo.DuplicateTarget(ctx, category.ID, normalized.AppID, normalized.Path, id)
	if err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	if duplicate {
		return ExternalTakeoutMerchant{}, ErrDuplicateTarget
	}
	changes := map[string]any{
		"title":     normalized.Title,
		"subtitle":  normalized.Subtitle,
		"icon_text": normalized.IconText,
		"app_id":    normalized.AppID,
		"path":      normalized.Path,
		"color":     normalized.Color,
		"sort":      normalized.Sort,
	}
	if err := s.repo.UpdateItemInCategory(ctx, id, category.ID, changes); err != nil {
		return ExternalTakeoutMerchant{}, err
	}
	item.Title = normalized.Title
	item.Subtitle = normalized.Subtitle
	item.IconText = normalized.IconText
	item.AppID = normalized.AppID
	item.Path = normalized.Path
	item.Color = normalized.Color
	item.Sort = normalized.Sort
	return toExternalTakeoutMerchant(item), nil
}

func (s *Service) DeleteExternalTakeoutMerchant(ctx context.Context, id uint) error {
	if id == 0 {
		return invalidExternalTakeoutInput("merchant id is required")
	}
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return err
	}
	if _, err := s.externalTakeoutItem(ctx, category.ID, id); err != nil {
		return err
	}
	return s.repo.DeleteItemInCategory(ctx, id, category.ID)
}

func (s *Service) SetExternalTakeoutOperatingStatus(ctx context.Context, id uint, status OperatingStatus) error {
	if id == 0 {
		return invalidExternalTakeoutInput("merchant id is required")
	}
	if !validOperatingStatus(status) {
		return invalidExternalTakeoutInput("invalid operating status")
	}
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return err
	}
	if _, err := s.externalTakeoutItem(ctx, category.ID, id); err != nil {
		return err
	}
	return s.repo.UpdateItemInCategory(ctx, id, category.ID, map[string]any{
		"operating_status": status,
	})
}

func (s *Service) SetExternalTakeoutVisibility(ctx context.Context, id uint, enabled bool) error {
	if id == 0 {
		return invalidExternalTakeoutInput("merchant id is required")
	}
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return err
	}
	if _, err := s.externalTakeoutItem(ctx, category.ID, id); err != nil {
		return err
	}
	return s.repo.UpdateItemInCategory(ctx, id, category.ID, map[string]any{"enabled": enabled})
}

func (s *Service) SetExternalTakeoutCategoryVisibility(ctx context.Context, enabled bool) error {
	category, err := s.externalTakeoutCategory(ctx)
	if err != nil {
		return err
	}
	return s.repo.UpdateCategory(ctx, category.ID, map[string]any{"enabled": enabled})
}

func (s *Service) externalTakeoutCategory(ctx context.Context) (Category, error) {
	category, found, err := s.repo.FindCategoryBySystemKey(ctx, SystemKeyExternalTakeoutMerchants)
	if err != nil {
		return Category{}, err
	}
	if !found || category.ID == 0 || category.SystemKey == nil ||
		*category.SystemKey != SystemKeyExternalTakeoutMerchants {
		return Category{}, ErrNotFound
	}
	return category, nil
}

func (s *Service) externalTakeoutItem(ctx context.Context, categoryID, id uint) (Item, error) {
	item, found, err := s.repo.FindItem(ctx, id)
	if err != nil {
		return Item{}, err
	}
	if !found || item.ID == 0 || item.CategoryID != categoryID {
		return Item{}, ErrNotFound
	}
	return item, nil
}

func toExternalTakeoutMerchant(item Item) ExternalTakeoutMerchant {
	status := item.OperatingStatus
	if status == "" {
		status = OperatingStatusOpen
	}
	return ExternalTakeoutMerchant{
		ID: item.ID, Title: item.Title, Subtitle: item.Subtitle, IconText: item.IconText,
		AppID: item.AppID, Path: item.Path, Color: item.Color, Sort: item.Sort,
		Enabled: item.Enabled, OperatingStatus: status,
	}
}

func toPublicCategories(values []Category) []PublicCategory {
	result := make([]PublicCategory, 0, len(values))
	for _, category := range values {
		if !category.Enabled {
			continue
		}
		items := make([]PublicItem, 0, len(category.Items))
		for _, item := range category.Items {
			if !item.Enabled {
				continue
			}
			status := item.OperatingStatus
			if status == "" {
				status = OperatingStatusOpen
			}
			items = append(items, PublicItem{
				ID: item.ID, Title: item.Title, Subtitle: item.Subtitle, IconText: item.IconText,
				AppID: item.AppID, Path: item.Path, Color: item.Color, Sort: item.Sort,
				OperatingStatus: status, IconPath: item.IconPath,
			})
		}
		var systemKey *string
		if category.SystemKey != nil {
			key := *category.SystemKey
			systemKey = &key
		}
		result = append(result, PublicCategory{
			ID: category.ID, Name: category.Name, SystemKey: systemKey, Sort: category.Sort, Items: items,
		})
	}
	return result
}

func systemManagedCategory(value Category) bool { return value.SystemKey != nil }

// itemCategoryLocked blocks generic item writes for system categories except
// the campus_maker channel, whose entries are maintained through the same
// admin API as ordinary categories.
func itemCategoryLocked(value Category) bool {
	return value.SystemKey != nil && *value.SystemKey != SystemKeyCampusMaker
}

func invalidNavigationInput(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidNavigationInput, detail)
}

func invalidExternalTakeoutInput(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidExternalTakeoutInput, detail)
}

func validateItem(value Item) error {
	if value.CategoryID == 0 || strings.TrimSpace(value.Title) == "" {
		return errors.New("category and title are required")
	}
	if strings.TrimSpace(value.AppID) == "" || strings.TrimSpace(value.Path) == "" {
		return errors.New("appid and path are required")
	}
	return nil
}
