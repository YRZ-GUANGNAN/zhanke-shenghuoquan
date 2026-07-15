package media

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormReferenceBindingRepository struct{ db *gorm.DB }

func NewReferenceBindingRepository(db *gorm.DB) *GormReferenceBindingRepository {
	return &GormReferenceBindingRepository{db: db}
}

type bindingAssetRow struct {
	ID                   uint64           `gorm:"column:id"`
	AssetPublicID        string           `gorm:"column:asset_public_id"`
	LegacyPublicID       string           `gorm:"column:legacy_public_id"`
	OwnerID              uint64           `gorm:"column:owner_id"`
	Purpose              Purpose          `gorm:"column:purpose"`
	StorageStage         StorageStage     `gorm:"column:storage_stage"`
	TechnicalStatus      TechnicalStatus  `gorm:"column:technical_status"`
	FinalKey             string           `gorm:"column:final_key"`
	FinalVersionID       string           `gorm:"column:final_version_id"`
	DeleteRequestedAt    *time.Time       `gorm:"column:delete_requested_at"`
	DeletedAt            *time.Time       `gorm:"column:deleted_at"`
	ReservationToken     string           `gorm:"column:reservation_token"`
	ReservationExpiresAt *time.Time       `gorm:"column:reservation_expires_at"`
	HasProjection        bool             `gorm:"column:has_projection"`
	ProjectionStatus     ProjectionStatus `gorm:"column:projection_status"`
}

const bindingAssetSelect = `
	a.id AS id,
	a.asset_public_id AS asset_public_id,
	COALESCE(a.public_id, '') AS legacy_public_id,
	a.owner_id AS owner_id,
	a.purpose AS purpose,
	a.storage_stage AS storage_stage,
	a.technical_status AS technical_status,
	COALESCE(a.final_key, '') AS final_key,
	COALESCE(a.final_version_id, '') AS final_version_id,
	a.delete_requested_at AS delete_requested_at,
	a.deleted_at AS deleted_at,
	COALESCE(a.reservation_token, '') AS reservation_token,
	a.reservation_expires_at AS reservation_expires_at,
	(p.asset_id IS NOT NULL) AS has_projection,
	COALESCE(p.status, '') AS projection_status`

func (r *GormReferenceBindingRepository) ReserveBinding(ctx context.Context, input BindingReserveInput) ([]BindingAsset, error) {
	var result []BindingAsset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := loadBindingAssetRows(tx, input.OrderedAssetPublicIDs, true)
		if err != nil {
			return err
		}
		ordered, legacyOnly, err := classifyBindingAssetRows(rows, input.OrderedAssetPublicIDs)
		if err != nil {
			return err
		}
		if legacyOnly {
			return ErrBindingLegacySet
		}

		assetIDs := bindingRowIDs(ordered)
		var references []AssetReference
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("asset_id IN ? AND status <> ?", assetIDs, ReferenceRevoked).
			Order("asset_id ASC, id ASC").Find(&references).Error; err != nil {
			return err
		}
		byAsset := groupBindingReferences(references)
		now := time.Now().UTC()
		result = make([]BindingAsset, 0, len(ordered))
		for index, row := range ordered {
			if !validCoreBindingAsset(row, input.OwnerID, input.Purpose) {
				return ErrBindingInvalidAsset
			}
			kept := false
			for _, reference := range byAsset[row.ID] {
				if input.ExistingReferenceID != "" && reference.ReferenceType == input.Purpose &&
					reference.ReferenceID == input.ExistingReferenceID && reference.RevocationFence == nil &&
					(reference.Status == ReferencePending || reference.Status == ReferenceActive) && !kept {
					kept = true
					continue
				}
				return ErrBindingConflict
			}
			if row.ReservationToken != "" && row.ReservationToken != input.Token &&
				(row.ReservationExpiresAt == nil || row.ReservationExpiresAt.After(now)) {
				return ErrBindingConflict
			}
			result = append(result, BindingAsset{
				InternalID: row.ID, AssetPublicID: row.AssetPublicID, Slot: uint16(index), Kept: kept,
			})
		}

		updated := tx.Model(&Asset{}).
			Where("id IN ? AND owner_id = ?", assetIDs, input.OwnerID).
			Where("reservation_token = '' OR reservation_token = ? OR reservation_expires_at IS NULL OR reservation_expires_at <= ?", input.Token, now).
			Updates(map[string]any{"reservation_token": input.Token, "reservation_expires_at": input.ExpiresAt})
		if updated.Error != nil {
			return updated.Error
		}
		var reserved int64
		if err := tx.Model(&Asset{}).Where("id IN ? AND owner_id = ? AND reservation_token = ?",
			assetIDs, input.OwnerID, input.Token).Count(&reserved).Error; err != nil {
			return err
		}
		if reserved != int64(len(assetIDs)) {
			return ErrBindingConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *GormReferenceBindingRepository) CommitBinding(ctx context.Context, input BindingCommitInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := loadBindingAssetRows(tx, input.OrderedAssetPublicIDs, true)
		if err != nil {
			return err
		}
		ordered, legacyOnly, err := classifyBindingAssetRows(rows, input.OrderedAssetPublicIDs)
		if err != nil {
			return err
		}
		if legacyOnly {
			return ErrBindingMismatch
		}
		assetIDs := bindingRowIDs(ordered)
		for _, row := range ordered {
			if !validCoreBindingAsset(row, input.OwnerID, input.ReferenceType) {
				return ErrBindingInvalidAsset
			}
		}

		var parentReferences []AssetReference
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference_type = ? AND reference_id = ?", input.ReferenceType, input.ReferenceID).
			Order("id ASC").Find(&parentReferences).Error; err != nil {
			return err
		}
		if exactBindingReferences(parentReferences, ordered) {
			return clearBindingReservation(tx, input.OwnerID, input.Token)
		}

		var assetReferences []AssetReference
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("asset_id IN ? AND status <> ?", assetIDs, ReferenceRevoked).
			Order("asset_id ASC, id ASC").Find(&assetReferences).Error; err != nil {
			return err
		}
		byAsset := groupBindingReferences(assetReferences)
		for _, row := range ordered {
			if row.ReservationToken != input.Token || row.ReservationExpiresAt == nil || row.ReservationExpiresAt.Before(time.Now().UTC()) {
				return ErrBindingMismatch
			}
			for _, reference := range byAsset[row.ID] {
				if reference.ReferenceType != input.ReferenceType || reference.ReferenceID != input.ReferenceID ||
					reference.RevocationFence != nil || (reference.Status != ReferencePending && reference.Status != ReferenceActive) {
					return ErrBindingConflict
				}
			}
		}

		if err := writeBindingReferences(tx, input, ordered, parentReferences); err != nil {
			return err
		}
		return clearBindingReservation(tx, input.OwnerID, input.Token)
	})
}

func (r *GormReferenceBindingRepository) ExactCommittedBinding(ctx context.Context, input BindingCommitInput) (bool, error) {
	rows, err := loadBindingAssetRows(r.db.WithContext(ctx), input.OrderedAssetPublicIDs, false)
	if err != nil {
		return false, err
	}
	ordered, legacyOnly, err := classifyBindingAssetRows(rows, input.OrderedAssetPublicIDs)
	if err != nil || legacyOnly {
		return false, err
	}
	for _, row := range ordered {
		if row.OwnerID != input.OwnerID {
			return false, nil
		}
	}
	var references []AssetReference
	if err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id = ?", input.ReferenceType, input.ReferenceID).
		Order("slot ASC, id ASC").Find(&references).Error; err != nil {
		return false, err
	}
	return exactBindingReferences(references, ordered), nil
}

func (r *GormReferenceBindingRepository) CancelBinding(ctx context.Context, ownerID uint64, token string) error {
	return r.db.WithContext(ctx).Model(&Asset{}).
		Where("owner_id = ? AND reservation_token = ?", ownerID, token).
		Updates(map[string]any{"reservation_token": "", "reservation_expires_at": nil}).Error
}

func (r *GormReferenceBindingRepository) PublishBinding(ctx context.Context, referenceType Purpose, referenceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		references, err := lockBindingParentReferences(tx, referenceType, referenceID)
		if err != nil {
			return err
		}
		if len(references) == 0 {
			return ErrNotFound
		}
		now := time.Now().UTC()
		assetIDs := make([]uint64, 0, len(references))
		publishable := 0
		for _, reference := range references {
			if reference.Status == ReferenceRevoked {
				continue
			}
			if reference.RevocationFence != nil || (reference.Status != ReferencePending && reference.Status != ReferenceActive) ||
				(reference.Status == ReferencePending && reference.ParentVisibility != ParentHidden) {
				return ErrBindingMismatch
			}
			publishable++
			assetIDs = append(assetIDs, reference.AssetID)
			if reference.Status == ReferencePending {
				updated := tx.Model(&AssetReference{}).Where("id = ? AND status = ?", reference.ID, ReferencePending).
					Updates(map[string]any{"status": ReferenceActive, "aggregate_version": gorm.Expr("aggregate_version + 1"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
				if updated.Error != nil || updated.RowsAffected != 1 {
					return bindingMutationError(updated)
				}
			}
		}
		if publishable == 0 {
			return ErrNotFound
		}
		for _, reference := range references {
			if reference.Status == ReferenceRevoked {
				continue
			}
			if reference.ParentVisibility != ParentHidden {
				continue
			}
			updated := tx.Model(&AssetReference{}).Where("id = ? AND parent_visibility = ?", reference.ID, ParentHidden).
				Updates(map[string]any{"parent_visibility": ParentVisible, "aggregate_version": gorm.Expr("aggregate_version + 1"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return bindingMutationError(updated)
			}
		}
		return rebuildBindingActiveRefCounts(tx, assetIDs, now)
	})
}

func (r *GormReferenceBindingRepository) RevokeBinding(ctx context.Context, referenceType Purpose, referenceID string, keep []uint64, fence string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		references, err := lockBindingParentReferences(tx, referenceType, referenceID)
		if err != nil {
			return err
		}
		if len(references) == 0 {
			return nil
		}

		now := time.Now().UTC()
		assetIDs := make([]uint64, 0, len(references))
		for _, reference := range references {
			assetIDs = append(assetIDs, reference.AssetID)
			if containsBindingID(keep, reference.AssetID) || reference.Status == ReferenceRevoked {
				continue
			}
			if reference.ParentVisibility == ParentVisible {
				if err := updateBindingReference(tx, reference.ID, map[string]any{"parent_visibility": ParentHidden}, now); err != nil {
					return err
				}
			}
			switch reference.Status {
			case ReferencePending:
				if err := updateBindingReference(tx, reference.ID, map[string]any{"status": ReferenceRevoked, "revocation_fence": fence}, now); err != nil {
					return err
				}
			case ReferenceActive:
				if err := updateBindingReference(tx, reference.ID, map[string]any{"status": ReferenceRevoking, "revocation_fence": fence}, now); err != nil {
					return err
				}
				if err := updateBindingReference(tx, reference.ID, map[string]any{"status": ReferenceRevoked}, now); err != nil {
					return err
				}
			case ReferenceRevoking:
				if err := updateBindingReference(tx, reference.ID, map[string]any{"status": ReferenceRevoked, "revocation_fence": fence}, now); err != nil {
					return err
				}
			default:
				return ErrBindingMismatch
			}
		}
		return rebuildBindingActiveRefCounts(tx, assetIDs, now)
	})
}

func loadBindingAssetRows(db *gorm.DB, lookupIDs []string, lock bool) ([]bindingAssetRow, error) {
	ids := append([]string(nil), lookupIDs...)
	sort.Strings(ids)
	query := db.Table("media_assets AS a").Select(bindingAssetSelect).
		Joins("LEFT JOIN media_review_projections AS p ON p.asset_id = a.id").
		Where("a.asset_public_id IN ? OR a.public_id IN ?", ids, ids).
		Order("a.id ASC")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var rows []bindingAssetRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func classifyBindingAssetRows(rows []bindingAssetRow, lookupIDs []string) ([]bindingAssetRow, bool, error) {
	ordered := make([]bindingAssetRow, 0, len(lookupIDs))
	coreCount, legacyCount := 0, 0
	for _, lookupID := range lookupIDs {
		match := -1
		for index, row := range rows {
			if row.AssetPublicID == lookupID || row.LegacyPublicID == lookupID {
				if match != -1 {
					return nil, false, ErrBindingInvalidAsset
				}
				match = index
			}
		}
		if match == -1 {
			return nil, false, ErrBindingInvalidAsset
		}
		row := rows[match]
		switch {
		case row.HasProjection && row.AssetPublicID == lookupID:
			coreCount++
		case !row.HasProjection && row.LegacyPublicID != "" && row.LegacyPublicID == lookupID:
			legacyCount++
		default:
			return nil, false, ErrBindingInvalidAsset
		}
		ordered = append(ordered, row)
	}
	if legacyCount == len(lookupIDs) {
		return ordered, true, nil
	}
	if coreCount != len(lookupIDs) || legacyCount != 0 {
		return nil, false, ErrBindingInvalidAsset
	}
	return ordered, false, nil
}

func validCoreBindingAsset(row bindingAssetRow, ownerID uint64, purpose Purpose) bool {
	return row.OwnerID == ownerID && row.Purpose == purpose && row.StorageStage == StorageStageFinal &&
		row.TechnicalStatus == TechnicalFinalReady && row.FinalKey != "" && row.FinalVersionID != "" &&
		row.HasProjection && row.ProjectionStatus == ProjectionApproved && row.DeleteRequestedAt == nil && row.DeletedAt == nil
}

func bindingRowIDs(rows []bindingAssetRow) []uint64 {
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func groupBindingReferences(references []AssetReference) map[uint64][]AssetReference {
	grouped := make(map[uint64][]AssetReference, len(references))
	for _, reference := range references {
		grouped[reference.AssetID] = append(grouped[reference.AssetID], reference)
	}
	return grouped
}

func exactBindingReferences(references []AssetReference, ordered []bindingAssetRow) bool {
	active := make([]AssetReference, 0, len(references))
	for _, reference := range references {
		if reference.Status == ReferenceRevoked {
			continue
		}
		if reference.Status != ReferencePending && reference.Status != ReferenceActive || reference.RevocationFence != nil {
			return false
		}
		active = append(active, reference)
	}
	if len(active) != len(ordered) {
		return false
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].Slot == active[j].Slot {
			return active[i].ID < active[j].ID
		}
		return active[i].Slot < active[j].Slot
	})
	for index, reference := range active {
		if reference.AssetID != ordered[index].ID || reference.Slot != uint16(index) {
			return false
		}
	}
	return true
}

func writeBindingReferences(tx *gorm.DB, input BindingCommitInput, ordered []bindingAssetRow, parentReferences []AssetReference) error {
	desired := make(map[uint64]struct{}, len(ordered))
	for _, row := range ordered {
		desired[row.ID] = struct{}{}
	}
	maxSlot := uint32(maxReferenceBindingAssets)
	for _, reference := range parentReferences {
		if uint32(reference.Slot) > maxSlot {
			maxSlot = uint32(reference.Slot)
		}
	}
	move := make(map[uint64]uint16)
	for _, reference := range parentReferences {
		_, wanted := desired[reference.AssetID]
		if uint32(reference.Slot) >= uint32(len(ordered)) && !(wanted && reference.Status != ReferenceRevoked) {
			continue
		}
		maxSlot++
		if maxSlot > uint32(^uint16(0)) {
			return ErrBindingConflict
		}
		move[reference.ID] = uint16(maxSlot)
	}
	now := time.Now().UTC()
	for _, reference := range parentReferences {
		slot, shouldMove := move[reference.ID]
		if !shouldMove {
			continue
		}
		updated := tx.Model(&AssetReference{}).Where("id = ?", reference.ID).
			Updates(map[string]any{"slot": slot, "aggregate_version": gorm.Expr("aggregate_version + 1"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if updated.Error != nil || updated.RowsAffected != 1 {
			return bindingMutationError(updated)
		}
	}

	operationID := strings.TrimPrefix(input.Token, module10BindingTokenPrefix)
	for index, row := range ordered {
		var existing *AssetReference
		for candidateIndex := range parentReferences {
			candidate := &parentReferences[candidateIndex]
			if candidate.AssetID == row.ID && candidate.Status != ReferenceRevoked {
				if existing != nil {
					return ErrBindingMismatch
				}
				existing = candidate
			}
		}
		if existing != nil {
			updated := tx.Model(&AssetReference{}).Where("id = ?", existing.ID).
				Updates(map[string]any{"slot": uint16(index), "aggregate_version": gorm.Expr("aggregate_version + 1"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return bindingMutationError(updated)
			}
			continue
		}
		reference := AssetReference{
			AssetID: row.ID, ReferenceType: input.ReferenceType, ReferenceID: input.ReferenceID, Slot: uint16(index),
			Status: ReferencePending, ParentVisibility: ParentHidden, AggregateVersion: 1,
			OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&reference).Error; err != nil {
			if isBindingDuplicate(err) {
				return ErrBindingConflict
			}
			return err
		}
	}
	return nil
}

func clearBindingReservation(tx *gorm.DB, ownerID uint64, token string) error {
	return tx.Model(&Asset{}).Where("owner_id = ? AND reservation_token = ?", ownerID, token).
		Updates(map[string]any{"reservation_token": "", "reservation_expires_at": nil}).Error
}

func lockBindingParentReferences(tx *gorm.DB, referenceType Purpose, referenceID string) ([]AssetReference, error) {
	var assetIDs []uint64
	if err := tx.Model(&AssetReference{}).Where("reference_type = ? AND reference_id = ?", referenceType, referenceID).
		Distinct().Pluck("asset_id", &assetIDs).Error; err != nil {
		return nil, err
	}
	if len(assetIDs) > 0 {
		sort.Slice(assetIDs, func(i, j int) bool { return assetIDs[i] < assetIDs[j] })
		var locked []Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id IN ?", assetIDs).Order("id ASC").Find(&locked).Error; err != nil {
			return nil, err
		}
	}
	var references []AssetReference
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reference_type = ? AND reference_id = ?", referenceType, referenceID).
		Order("id ASC").Find(&references).Error; err != nil {
		return nil, err
	}
	return references, nil
}

func updateBindingReference(tx *gorm.DB, id uint64, changes map[string]any, now time.Time) error {
	changes["aggregate_version"] = gorm.Expr("aggregate_version + 1")
	changes["row_version"] = gorm.Expr("row_version + 1")
	changes["updated_at"] = now
	updated := tx.Model(&AssetReference{}).Where("id = ?", id).Updates(changes)
	if updated.Error != nil || updated.RowsAffected != 1 {
		return bindingMutationError(updated)
	}
	return nil
}

func rebuildBindingActiveRefCounts(tx *gorm.DB, assetIDs []uint64, now time.Time) error {
	sort.Slice(assetIDs, func(i, j int) bool { return assetIDs[i] < assetIDs[j] })
	assetIDs = compactBindingIDs(assetIDs)
	for _, assetID := range assetIDs {
		var count int64
		if err := tx.Model(&AssetReference{}).Where("asset_id = ? AND status = ?", assetID, ReferenceActive).Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(&Asset{}).Where("id = ? AND active_ref_count <> ?", assetID, count).
			Updates(map[string]any{"active_ref_count": uint64(count), "row_version": gorm.Expr("row_version + 1"), "updated_at": now}).Error; err != nil {
			return err
		}
	}
	return nil
}

func compactBindingIDs(ids []uint64) []uint64 {
	if len(ids) < 2 {
		return ids
	}
	result := ids[:1]
	for _, id := range ids[1:] {
		if id != result[len(result)-1] {
			result = append(result, id)
		}
	}
	return result
}

func containsBindingID(ids []uint64, wanted uint64) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func bindingMutationError(result *gorm.DB) error {
	if result.Error != nil {
		if isBindingDuplicate(result.Error) {
			return ErrBindingConflict
		}
		return result.Error
	}
	return ErrBindingMismatch
}

func isBindingDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

var _ ReferenceBindingRepository = (*GormReferenceBindingRepository)(nil)
