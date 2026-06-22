package takeout

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type CatalogService struct {
	repository CatalogRepository
	now        func() time.Time
}

func NewCatalogService(repository CatalogRepository, now func() time.Time) (*CatalogService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &CatalogService{repository: repository, now: now}, nil
}

type CreateCategoryInput struct {
	MerchantPublicID string
	Name             string
	SortOrder        int32
}

func (s *CatalogService) CreateCategory(ctx context.Context, input CreateCategoryInput) (DishCategory, error) {
	publicID, err := NewULID()
	if err != nil {
		return DishCategory{}, err
	}
	now := s.now().UTC()
	value := DishCategory{
		CategoryPublicID: publicID, MerchantPublicID: strings.TrimSpace(input.MerchantPublicID),
		Name: strings.TrimSpace(input.Name), SortOrder: input.SortOrder,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return DishCategory{}, err
	}
	if err := s.repository.CreateCategory(ctx, &value); err != nil {
		return DishCategory{}, err
	}
	return value, nil
}

func (s *CatalogService) RenameCategory(
	ctx context.Context,
	categoryPublicID, merchantPublicID string,
	expectedRowVersion uint64,
	name string,
) (DishCategory, error) {
	name = strings.TrimSpace(name)
	if err := validateCatalogIdentity(categoryPublicID, merchantPublicID, expectedRowVersion); err != nil ||
		name == "" || utf8.RuneCountInString(name) > 64 {
		return DishCategory{}, ErrInvalidRecord
	}
	return s.repository.RenameCategory(ctx, categoryPublicID, merchantPublicID, expectedRowVersion, name, s.now().UTC())
}

func (s *CatalogService) ReorderCategory(
	ctx context.Context,
	categoryPublicID, merchantPublicID string,
	expectedRowVersion uint64,
	sortOrder int32,
) (DishCategory, error) {
	if err := validateCatalogIdentity(categoryPublicID, merchantPublicID, expectedRowVersion); err != nil {
		return DishCategory{}, err
	}
	return s.repository.ReorderCategory(ctx, categoryPublicID, merchantPublicID, expectedRowVersion, sortOrder, s.now().UTC())
}

func (s *CatalogService) FindCategory(ctx context.Context, categoryPublicID string) (DishCategory, error) {
	if !ValidULID(categoryPublicID) {
		return DishCategory{}, ErrInvalidULID
	}
	return s.repository.FindCategory(ctx, categoryPublicID)
}

type CreateDishInput struct {
	MerchantPublicID   string
	CategoryPublicID   *string
	Name               string
	Description        string
	PriceCents         uint64
	ImageAssetPublicID *string
	SortOrder          int32
}

type UpdateDishInput struct {
	DishPublicID       string
	MerchantPublicID   string
	ExpectedStatus     DishStatus
	ExpectedRowVersion uint64
	CategoryPublicID   *string
	Name               string
	Description        string
	PriceCents         uint64
	ImageAssetPublicID *string
	SortOrder          int32
}

func (s *CatalogService) CreateDish(ctx context.Context, input CreateDishInput) (Dish, error) {
	publicID, err := NewULID()
	if err != nil {
		return Dish{}, err
	}
	details := normalizeDishDetails(input.CategoryPublicID, input.Name, input.Description, input.PriceCents, input.ImageAssetPublicID, input.SortOrder)
	now := s.now().UTC()
	value := Dish{
		DishPublicID: publicID, MerchantPublicID: strings.TrimSpace(input.MerchantPublicID),
		CategoryPublicID: details.CategoryPublicID, Name: details.Name, Description: details.Description,
		PriceCents: details.PriceCents, ImageAssetPublicID: details.ImageAssetPublicID,
		Status: DishDraft, SortOrder: details.SortOrder, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return Dish{}, err
	}
	if err := s.repository.CreateDish(ctx, &value); err != nil {
		return Dish{}, err
	}
	return value, nil
}

func (s *CatalogService) UpdateDish(ctx context.Context, input UpdateDishInput) (Dish, error) {
	if err := validateCatalogIdentity(input.DishPublicID, input.MerchantPublicID, input.ExpectedRowVersion); err != nil {
		return Dish{}, err
	}
	if !input.ExpectedStatus.Valid() || input.ExpectedStatus.Terminal() {
		return Dish{}, ErrInvalidTransition
	}
	details := normalizeDishDetails(input.CategoryPublicID, input.Name, input.Description, input.PriceCents, input.ImageAssetPublicID, input.SortOrder)
	if !validDishDetails(details) {
		return Dish{}, ErrInvalidRecord
	}
	return s.repository.UpdateDish(ctx, input.DishPublicID, input.MerchantPublicID,
		input.ExpectedRowVersion, input.ExpectedStatus, details, s.now().UTC())
}

func (s *CatalogService) PutOnShelf(ctx context.Context, dishPublicID, merchantPublicID string, expectedStatus DishStatus, expectedRowVersion uint64) (Dish, error) {
	if expectedStatus != DishDraft && expectedStatus != DishOffShelf {
		return Dish{}, ErrInvalidTransition
	}
	return s.transitionDish(ctx, dishPublicID, merchantPublicID, expectedRowVersion, expectedStatus, DishOnShelf)
}

func (s *CatalogService) MarkSoldOut(ctx context.Context, dishPublicID, merchantPublicID string, expectedRowVersion uint64) (Dish, error) {
	return s.transitionDish(ctx, dishPublicID, merchantPublicID, expectedRowVersion, DishOnShelf, DishSoldOut)
}

func (s *CatalogService) Restock(ctx context.Context, dishPublicID, merchantPublicID string, expectedRowVersion uint64) (Dish, error) {
	return s.transitionDish(ctx, dishPublicID, merchantPublicID, expectedRowVersion, DishSoldOut, DishOnShelf)
}

func (s *CatalogService) TakeOffShelf(
	ctx context.Context,
	dishPublicID, merchantPublicID string,
	expectedStatus DishStatus,
	expectedRowVersion uint64,
) (Dish, error) {
	return s.transitionDish(ctx, dishPublicID, merchantPublicID, expectedRowVersion, expectedStatus, DishOffShelf)
}

func (s *CatalogService) RemoveDish(ctx context.Context, dishPublicID, merchantPublicID string, expectedRowVersion uint64) (Dish, error) {
	return s.transitionDish(ctx, dishPublicID, merchantPublicID, expectedRowVersion, DishOffShelf, DishRemoved)
}

func (s *CatalogService) FindDish(ctx context.Context, dishPublicID string) (Dish, error) {
	if !ValidULID(dishPublicID) {
		return Dish{}, ErrInvalidULID
	}
	return s.repository.FindDish(ctx, dishPublicID)
}

func (s *CatalogService) transitionDish(
	ctx context.Context,
	dishPublicID, merchantPublicID string,
	expectedRowVersion uint64,
	from, to DishStatus,
) (Dish, error) {
	if err := validateCatalogIdentity(dishPublicID, merchantPublicID, expectedRowVersion); err != nil {
		return Dish{}, err
	}
	if !from.Valid() || !to.Valid() || !CanTransitionDish(from, to) {
		return Dish{}, ErrInvalidTransition
	}
	return s.repository.TransitionDishStatus(ctx, dishPublicID, merchantPublicID,
		expectedRowVersion, from, to, s.now().UTC())
}

func validateCatalogIdentity(publicID, merchantPublicID string, expectedRowVersion uint64) error {
	if !ValidULID(publicID) || !ValidULID(merchantPublicID) {
		return ErrInvalidULID
	}
	if expectedRowVersion == 0 {
		return ErrInvalidRecord
	}
	return nil
}

func normalizeDishDetails(
	categoryPublicID *string,
	name, description string,
	priceCents uint64,
	imageAssetPublicID *string,
	sortOrder int32,
) dishDetails {
	return dishDetails{
		CategoryPublicID: normalizeCatalogULIDPointer(categoryPublicID),
		Name:             strings.TrimSpace(name), Description: strings.TrimSpace(description), PriceCents: priceCents,
		ImageAssetPublicID: normalizeCatalogULIDPointer(imageAssetPublicID), SortOrder: sortOrder,
	}
}

func normalizeCatalogULIDPointer(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	return &normalized
}
