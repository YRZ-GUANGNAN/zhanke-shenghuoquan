package delivery

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TxRiderEligibility is supplied by the composition root. It must use the
// supplied transaction for current, locked identity checks and perform no IO
// outside this database. Explicit revocation errors persist an off-duty state.
type TxRiderEligibility func(context.Context, *gorm.DB, uint64) (string, error)

type RiderAvailabilityStore struct {
	db       *gorm.DB
	appender NotificationEventAppender
	eligible TxRiderEligibility
}

func NewRiderAvailabilityStore(db *gorm.DB, appender NotificationEventAppender, eligible TxRiderEligibility) *RiderAvailabilityStore {
	return &RiderAvailabilityStore{db: db, appender: appender, eligible: eligible}
}

var _ RiderAvailabilityRepository = (*RiderAvailabilityStore)(nil)
var _ RiderAvailabilityClaimRepository = (*RiderAvailabilityStore)(nil)

func (s *RiderAvailabilityStore) ready(ctx context.Context) bool {
	return s != nil && s.db != nil && s.eligible != nil &&
		s.db.WithContext(ctx).Migrator().HasTable(&RiderAvailability{}) &&
		s.db.WithContext(ctx).Migrator().HasTable(&RiderBroadcastConfig{})
}

func readAvailabilityConfigTx(tx *gorm.DB) (RiderBroadcastConfig, error) {
	var value RiderBroadcastConfig
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("config_scope = ?", "global").First(&value).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return value, ErrRiderAvailabilityUnavailable
		}
		return value, err
	}
	if !value.Valid() {
		return value, ErrRiderAvailabilityUnavailable
	}
	return value, nil
}

func readAvailabilityTx(tx *gorm.DB, riderID uint64) (RiderAvailability, error) {
	var value RiderAvailability
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("rider_user_id = ?", riderID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, ErrRiderAvailabilityUnavailable
	}
	if err != nil {
		return value, err
	}
	if !value.Valid() {
		return value, ErrRiderAvailabilityUnavailable
	}
	return value, nil
}

// A locking SELECT is a current read under MySQL REPEATABLE READ. Count(*)
// without this read could retain the snapshot from before a concurrent claim.
func riderInFlightTx(tx *gorm.DB, riderID uint64) (uint32, error) {
	var rows []struct{ ID uint64 }
	err := tx.Model(&Order{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("rider_user_id = ? AND status IN ?", riderID,
		[]Status{StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering}).Order("id ASC").Find(&rows).Error
	if err != nil {
		return 0, err
	}
	if uint64(len(rows)) > math.MaxUint32 {
		return 0, ErrRiderAvailabilityUnavailable
	}
	return uint32(len(rows)), nil
}

func availabilitySnapshot(value RiderAvailability, config RiderBroadcastConfig, inFlight uint32) RiderAvailabilitySnapshot {
	return RiderAvailabilitySnapshot{RiderUserID: value.RiderUserID, State: value.State, Scope: value.Scope,
		InFlight: inFlight, MaxInFlight: config.MaxInFlightPerRider, RowVersion: value.RowVersion, UpdatedAt: value.UpdatedAt}
}

func persistRevokedDutyTx(tx *gorm.DB, value *RiderAvailability, revision string, now time.Time) error {
	if value.ID == 0 || value.State == RiderOffDuty {
		return nil
	}
	if value.RowVersion == math.MaxUint64 {
		return ErrRiderAvailabilityUnavailable
	}
	updates := map[string]any{"state": RiderOffDuty, "row_version": gorm.Expr("row_version + 1"), "updated_at": now.UTC()}
	if validRiderEligibilityRevision(revision) {
		updates["eligibility_revision"] = revision
	}
	update := tx.Model(&RiderAvailability{}).Where("id = ? AND row_version = ?", value.ID, value.RowVersion).
		Updates(updates)
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrRiderAvailabilityConflict
	}
	value.State, value.UpdatedAt = RiderOffDuty, now.UTC()
	if validRiderEligibilityRevision(revision) {
		value.EligibilityRevision = revision
	}
	value.RowVersion++
	return nil
}

func riderEligibilityRevoked(err error) bool {
	return errors.Is(err, ErrRiderUnavailable) || errors.Is(err, ErrRiderNotRegistered)
}

// withRiderLocks uses one order for duty changes, candidate reads and claims:
// availability -> global config -> eligibility locks -> assigned orders.
// Global config serializes claim admission with capacity edits. Completion and
// release only reduce occupancy and never acquire availability in reverse.
func (s *RiderAvailabilityStore) withRiderLocks(ctx context.Context, riderID uint64, now time.Time,
	initialize bool, action func(*gorm.DB, RiderAvailability, RiderBroadcastConfig, string) error) error {
	if riderID == 0 || now.IsZero() {
		return ErrInvalidRecord
	}
	if !s.ready(ctx) {
		return ErrRiderAvailabilityUnavailable
	}
	// Probe outside the transaction: a missing-row FOR UPDATE would keep a gap
	// lock while waiting for global config and deadlock a concurrent first start.
	// Availability facts are never deleted by normal operation. A missing GET is
	// a read-only, virtual off-duty state; only an explicit version-zero Set inserts.
	exists := false
	if !initialize {
		var row struct{ ID uint64 }
		err := s.db.WithContext(ctx).Model(&RiderAvailability{}).Select("id").Where("rider_user_id = ?", riderID).Take(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		exists = err == nil
	}
	var revoked error
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value := RiderAvailability{RiderUserID: riderID, State: RiderOffDuty, Scope: RiderAvailabilityScopeCampus}
		if initialize {
			value.EligibilityRevision, value.RowVersion = "pending", 1
			value.CreatedAt, value.UpdatedAt = now.UTC(), now.UTC()
			// Take the unique record lock before config. Duplicate starts never
			// acquire a missing-row gap lock and remain a version conflict.
			if err := tx.Create(&value).Error; err != nil {
				if isDuplicateKey(err) {
					return ErrRiderAvailabilityConflict
				}
				return err
			}
		} else if exists {
			var err error
			value, err = readAvailabilityTx(tx, riderID)
			if err != nil {
				return err
			}
		} else {
			// A missing row is a read-only virtual off-duty state. Only an
			// explicit start/stop may create the durable availability fact.
			value.RiderUserID = riderID
			value.State = RiderOffDuty
			value.Scope = RiderAvailabilityScopeCampus
		}
		config, err := readAvailabilityConfigTx(tx)
		if err != nil {
			return err
		}
		revision, err := s.eligible(ctx, tx, riderID)
		if err != nil {
			if initialize || !riderEligibilityRevoked(err) {
				return err
			}
			if err := persistRevokedDutyTx(tx, &value, "", now); err != nil {
				return err
			}
			revoked = err
			return nil
		}
		if !validRiderEligibilityRevision(revision) {
			return ErrRiderAvailabilityUnavailable
		}
		if value.State == RiderOnDuty && value.EligibilityRevision != revision {
			if err := persistRevokedDutyTx(tx, &value, revision, now); err != nil {
				return err
			}
			revoked = ErrRiderAvailabilityConflict
			return nil
		}
		return action(tx, value, config, revision)
	})
	if err != nil {
		return err
	}
	return revoked
}

func (s *RiderAvailabilityStore) ReadRiderAvailability(ctx context.Context, riderID uint64) (RiderAvailabilitySnapshot, error) {
	var result RiderAvailabilitySnapshot
	err := s.withRiderLocks(ctx, riderID, time.Now().UTC(), false, func(tx *gorm.DB, value RiderAvailability, config RiderBroadcastConfig, _ string) error {
		inFlight, err := riderInFlightTx(tx, riderID)
		if err != nil {
			return err
		}
		result = availabilitySnapshot(value, config, inFlight)
		return nil
	})
	return result, err
}

func (s *RiderAvailabilityStore) SetRiderDuty(ctx context.Context, riderID uint64, state RiderDutyState, version uint64, now time.Time) (RiderAvailabilitySnapshot, error) {
	if !state.Valid() || version == math.MaxUint64 {
		return RiderAvailabilitySnapshot{}, ErrInvalidRecord
	}
	var result RiderAvailabilitySnapshot
	err := s.withRiderLocks(ctx, riderID, now, version == 0, func(tx *gorm.DB, value RiderAvailability, config RiderBroadcastConfig, revision string) error {
		if version != 0 && value.RowVersion != version {
			return ErrRiderAvailabilityConflict
		}
		if value.ID == 0 {
			return ErrRiderAvailabilityConflict
		}
		nextVersion := value.RowVersion + 1
		if version == 0 {
			nextVersion = 1
		}
		update := tx.Model(&RiderAvailability{}).Where("id = ? AND row_version = ?", value.ID, value.RowVersion).
			Updates(map[string]any{"state": state, "eligibility_revision": revision, "row_version": nextVersion, "updated_at": now.UTC()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrRiderAvailabilityConflict
		}
		value.State, value.EligibilityRevision, value.UpdatedAt, value.RowVersion = state, revision, now.UTC(), nextVersion
		inFlight, err := riderInFlightTx(tx, riderID)
		if err != nil {
			return err
		}
		result = availabilitySnapshot(value, config, inFlight)
		return nil
	})
	return result, err
}

func (s *RiderAvailabilityStore) ReadRiderBroadcastConfig(ctx context.Context) (RiderBroadcastConfig, error) {
	if !s.ready(ctx) {
		return RiderBroadcastConfig{}, ErrRiderAvailabilityUnavailable
	}
	var result RiderBroadcastConfig
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = readAvailabilityConfigTx(tx)
		return err
	})
	return result, err
}

func (s *RiderAvailabilityStore) UpdateRiderBroadcastConfig(ctx context.Context, max uint32, version uint64, now time.Time) (RiderBroadcastConfig, error) {
	if max == 0 || max > 100 || version == 0 || version == math.MaxUint64 || now.IsZero() {
		return RiderBroadcastConfig{}, ErrInvalidRecord
	}
	if !s.ready(ctx) {
		return RiderBroadcastConfig{}, ErrRiderAvailabilityUnavailable
	}
	var result RiderBroadcastConfig
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = readAvailabilityConfigTx(tx)
		if err != nil {
			return err
		}
		if result.RowVersion != version {
			return ErrRiderAvailabilityConflict
		}
		update := tx.Model(&RiderBroadcastConfig{}).Where("id = ? AND row_version = ?", result.ID, version).
			Updates(map[string]any{"max_in_flight_per_rider": max, "row_version": gorm.Expr("row_version + 1"), "updated_at": now.UTC()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrRiderAvailabilityConflict
		}
		result.MaxInFlightPerRider, result.UpdatedAt = max, now.UTC()
		result.RowVersion++
		return nil
	})
	return result, err
}

func (s *RiderAvailabilityStore) ListOnDutyRiderAvailability(ctx context.Context, now time.Time) ([]RiderAvailabilitySnapshot, error) {
	if now.IsZero() {
		return nil, ErrInvalidRecord
	}
	if !s.ready(ctx) {
		return nil, ErrRiderAvailabilityUnavailable
	}
	if _, err := s.ReadRiderBroadcastConfig(ctx); err != nil {
		return nil, err
	}
	// This scan only nominates IDs; every nominated row is re-read under locks.
	var ids []uint64
	if err := s.db.WithContext(ctx).Model(&RiderAvailability{}).Where("state = ? AND scope = ?", RiderOnDuty, RiderAvailabilityScopeCampus).
		Order("rider_user_id ASC").Pluck("rider_user_id", &ids).Error; err != nil {
		return nil, err
	}
	result := make([]RiderAvailabilitySnapshot, 0, len(ids))
	for _, id := range ids {
		var snapshot RiderAvailabilitySnapshot
		err := s.withRiderLocks(ctx, id, now, false, func(tx *gorm.DB, value RiderAvailability, config RiderBroadcastConfig, _ string) error {
			if value.State != RiderOnDuty {
				return nil
			}
			count, err := riderInFlightTx(tx, id)
			if err != nil {
				return err
			}
			snapshot = availabilitySnapshot(value, config, count)
			return nil
		})
		if riderEligibilityRevoked(err) || errors.Is(err, ErrRiderAvailabilityConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if snapshot.Valid() && snapshot.State == RiderOnDuty && snapshot.InFlight < snapshot.MaxInFlight {
			result = append(result, snapshot)
		}
	}
	return result, nil
}

func (s *RiderAvailabilityStore) ClaimOrderWithAvailability(ctx context.Context, riderID uint64, publicID string, version uint64, now time.Time) (Order, error) {
	publicID = strings.TrimSpace(publicID)
	if !ValidULID(publicID) || version == 0 {
		return Order{}, ErrInvalidRecord
	}
	var result Order
	err := s.withRiderLocks(ctx, riderID, now, false, func(tx *gorm.DB, value RiderAvailability, config RiderBroadcastConfig, _ string) error {
		count, err := riderInFlightTx(tx, riderID)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("delivery_order_public_id = ?", publicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		// Replaying an already committed claim must not consume a second slot.
		if result.Status == StatusClaimed && result.RiderUserID != nil && *result.RiderUserID == riderID {
			return nil
		}
		if result.Status != StatusReadyForClaim || result.RiderUserID != nil || result.RowVersion != version {
			return ErrConcurrentUpdate
		}
		if value.State != RiderOnDuty {
			return ErrRiderOffDuty
		}
		if count >= config.MaxInFlightPerRider {
			return ErrRiderAtCapacity
		}
		before := result
		updates, err := buildTransitionUpdates(result, StatusReadyForClaim, StatusClaimed, &riderID, now.UTC())
		if err != nil {
			return err
		}
		updates["status"], updates["updated_at"], updates["row_version"] = StatusClaimed, now.UTC(), gorm.Expr("row_version + 1")
		update := tx.Model(&Order{}).Where("id = ? AND row_version = ? AND status = ? AND rider_user_id IS NULL", result.ID, version, StatusReadyForClaim).Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyTransition(&result, StatusClaimed, &riderID, now.UTC())
		result.RowVersion++
		result.UpdatedAt = now.UTC()
		return appendOrderNotificationTx(tx, s.appender, before, result, now.UTC())
	})
	return result, err
}
