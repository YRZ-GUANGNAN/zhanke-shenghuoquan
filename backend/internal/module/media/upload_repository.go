package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UploadTicketRepository interface {
	CreateUploadBundle(context.Context, CreateUploadBundleParams) (UploadBundle, bool, error)
	FindUploadBundle(context.Context, string, uint64) (UploadBundle, error)
	AuthorizeUploadTicket(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error)
	MarkUploadTicketUploaded(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error)
	ConsumeUploadTicket(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error)
	CancelUploadTicket(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error)
	ExpireUploadTickets(context.Context, time.Time, int) (int, error)
}

func (r *GormRepository) CreateUploadBundle(ctx context.Context, params CreateUploadBundleParams) (UploadBundle, bool, error) {
	var result UploadBundle
	created := false
	err := r.withOwnerQuotaLock(ctx, params.Bundle.Asset.OwnerID, func(tx *gorm.DB) error {
		existing, found, err := findIdempotentBundle(tx, params.Bundle.Asset.OwnerID, params.Bundle.Asset.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if !sameUploadDeclaration(existing, params.Bundle) {
				return ErrIdempotencyConflict
			}
			result = existing
			return nil
		}

		var reservations []QuotaReservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_id = ?", params.Bundle.Asset.OwnerID).
			Order("reservation_id ASC").Find(&reservations).Error; err != nil {
			return err
		}
		if err := enforceQuota(reservations, params.Bundle.Reservation, params.Policy, params.Now); err != nil {
			return err
		}

		result = params.Bundle
		if err := tx.Create(&result.Asset).Error; err != nil {
			return err
		}
		result.Ticket.AssetID = result.Asset.ID
		if err := tx.Create(&result.Ticket).Error; err != nil {
			return err
		}
		if err := tx.Create(&result.Reservation).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return result, created, err
}

func (r *GormRepository) withOwnerQuotaLock(ctx context.Context, ownerID uint64, operation func(*gorm.DB) error) error {
	lockName := fmt.Sprintf("media-quota:%d", ownerID)
	return r.db.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		var acquired int
		if err := connection.Raw("SELECT GET_LOCK(?, 10)", lockName).Scan(&acquired).Error; err != nil {
			return err
		}
		if acquired != 1 {
			return ErrQuotaLockTimeout
		}
		operationErr := connection.Transaction(operation)
		releaseCtx, cancelRelease := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRelease()
		var released int
		releaseErr := connection.Session(&gorm.Session{Context: releaseCtx}).Raw("SELECT RELEASE_LOCK(?)", lockName).Scan(&released).Error
		if releaseErr != nil || released != 1 {
			return errors.Join(operationErr, releaseErr, ErrQuotaLockTimeout)
		}
		return operationErr
	})
}

func (r *GormRepository) FindUploadBundle(ctx context.Context, ticketID string, ownerID uint64) (UploadBundle, error) {
	return loadUploadBundle(r.db.WithContext(ctx), ticketID, ownerID, false)
}

func (r *GormRepository) AuthorizeUploadTicket(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, now time.Time) (UploadBundle, error) {
	var bundle UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loaded, err := loadUploadBundle(tx, ticketID, ownerID, true)
		if err != nil {
			return err
		}
		if loaded.Ticket.ExpiresAt.Before(now) || loaded.Ticket.ExpiresAt.Equal(now) {
			return ErrTicketExpired
		}
		if loaded.Ticket.Status != TicketCreated || loaded.Ticket.RowVersion != expectedVersion {
			if loaded.Ticket.Status == TicketAuthorizationIssued || loaded.Ticket.CredentialIssueCount > 0 {
				return ErrAuthorizationAlreadyIssued
			}
			return ErrConcurrentUpdate
		}
		if loaded.Ticket.CredentialIssueCount != 0 || loaded.Asset.StagingKey == nil || *loaded.Asset.StagingKey == "" || loaded.Asset.StorageStage != StorageStageStaging {
			return ErrTicketInvariant
		}
		update := tx.Model(&UploadTicket{}).
			Where("ticket_id = ? AND owner_id = ? AND row_version = ? AND status = ? AND credential_issue_count = 0", ticketID, ownerID, expectedVersion, TicketCreated).
			Updates(map[string]any{
				"status": TicketAuthorizationIssued, "credential_issue_count": gorm.Expr("credential_issue_count + 1"),
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if err := casUpdate(update); err != nil {
			return err
		}
		loaded.Ticket.Status = TicketAuthorizationIssued
		loaded.Ticket.CredentialIssueCount++
		loaded.Ticket.RowVersion++
		loaded.Ticket.UpdatedAt = now
		bundle = loaded
		return nil
	})
	return bundle, err
}

func (r *GormRepository) MarkUploadTicketUploaded(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, now time.Time) (UploadBundle, error) {
	return r.transitionTicketOnly(ctx, ticketID, ownerID, expectedVersion, TicketAuthorizationIssued, TicketUploaded, now)
}

func (r *GormRepository) ConsumeUploadTicket(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, now time.Time) (UploadBundle, error) {
	return r.transitionTicketAndReservation(ctx, ticketID, ownerID, expectedVersion, TicketUploaded, TicketConsumed, ReservationCommitted, now)
}

func (r *GormRepository) CancelUploadTicket(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, now time.Time) (UploadBundle, error) {
	var result UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, ticketID, ownerID, true)
		if err != nil {
			return err
		}
		if bundle.Ticket.Status == TicketCancelled {
			result = bundle
			return nil
		}
		if bundle.Ticket.RowVersion != expectedVersion || !CanTransitionTicket(bundle.Ticket.Status, TicketCancelled) {
			return ErrConcurrentUpdate
		}
		if err := updateTicketCAS(tx, &bundle.Ticket, expectedVersion, bundle.Ticket.Status, TicketCancelled, now); err != nil {
			return err
		}
		if bundle.Reservation.Status == ReservationActive {
			if err := updateReservationCAS(tx, &bundle.Reservation, ReservationActive, ReservationReleased, now); err != nil {
				return err
			}
		}
		result = bundle
		return nil
	})
	return result, err
}

func (r *GormRepository) ExpireUploadTickets(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tickets []UploadTicket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND expires_at <= ?", []TicketStatus{TicketCreated, TicketAuthorizationIssued, TicketUploaded}, now).
			Order("expires_at ASC, ticket_id ASC").Limit(limit).Find(&tickets).Error; err != nil {
			return err
		}
		for index := range tickets {
			ticket := &tickets[index]
			if err := updateTicketCAS(tx, ticket, ticket.RowVersion, ticket.Status, TicketExpired, now); err != nil {
				return err
			}
			var reservation QuotaReservation
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ticket_id = ?", ticket.TicketID).First(&reservation).Error; err != nil {
				return normalizeNotFound(err)
			}
			if reservation.Status == ReservationActive {
				if err := updateReservationCAS(tx, &reservation, ReservationActive, ReservationReleased, now); err != nil {
					return err
				}
			}
			count++
		}
		return nil
	})
	return count, err
}

func (r *GormRepository) transitionTicketOnly(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, from, to TicketStatus, now time.Time) (UploadBundle, error) {
	var result UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, ticketID, ownerID, true)
		if err != nil {
			return err
		}
		if bundle.Ticket.Status == to {
			result = bundle
			return nil
		}
		if bundle.Ticket.Status != from || bundle.Ticket.RowVersion != expectedVersion || !CanTransitionTicket(from, to) {
			return ErrConcurrentUpdate
		}
		if err := updateTicketCAS(tx, &bundle.Ticket, expectedVersion, from, to, now); err != nil {
			return err
		}
		result = bundle
		return nil
	})
	return result, err
}

func (r *GormRepository) transitionTicketAndReservation(ctx context.Context, ticketID string, ownerID, expectedVersion uint64, from, to TicketStatus, reservationTo ReservationStatus, now time.Time) (UploadBundle, error) {
	var result UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, ticketID, ownerID, true)
		if err != nil {
			return err
		}
		if bundle.Ticket.Status == to && bundle.Reservation.Status == reservationTo {
			result = bundle
			return nil
		}
		if bundle.Ticket.Status != from || bundle.Ticket.RowVersion != expectedVersion || !CanTransitionTicket(from, to) {
			return ErrConcurrentUpdate
		}
		if bundle.Reservation.Status != ReservationActive {
			return ErrTicketInvariant
		}
		if err := updateTicketCAS(tx, &bundle.Ticket, expectedVersion, from, to, now); err != nil {
			return err
		}
		if err := updateReservationCAS(tx, &bundle.Reservation, ReservationActive, reservationTo, now); err != nil {
			return err
		}
		result = bundle
		return nil
	})
	return result, err
}

func findIdempotentBundle(tx *gorm.DB, ownerID uint64, idempotencyKey string) (UploadBundle, bool, error) {
	var asset Asset
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_id = ? AND idempotency_key = ?", ownerID, idempotencyKey).First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UploadBundle{}, false, nil
	}
	if err != nil {
		return UploadBundle{}, false, err
	}
	bundle, err := loadUploadBundleByAsset(tx, asset, true)
	if errors.Is(err, ErrNotFound) {
		return UploadBundle{}, true, ErrTicketInvariant
	}
	return bundle, true, err
}

func loadUploadBundle(tx *gorm.DB, ticketID string, ownerID uint64, lock bool) (UploadBundle, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var ticket UploadTicket
	err := query.Where("ticket_id = ?", ticketID).First(&ticket).Error
	if err != nil {
		return UploadBundle{}, normalizeNotFound(err)
	}
	if ticket.OwnerID != ownerID {
		return UploadBundle{}, ErrTicketForbidden
	}
	assetQuery := tx
	reservationQuery := tx
	if lock {
		assetQuery = assetQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		reservationQuery = reservationQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var asset Asset
	if err := assetQuery.First(&asset, ticket.AssetID).Error; err != nil {
		return UploadBundle{}, normalizeNotFound(err)
	}
	var reservation QuotaReservation
	if err := reservationQuery.Where("ticket_id = ?", ticketID).First(&reservation).Error; err != nil {
		return UploadBundle{}, normalizeNotFound(err)
	}
	if asset.OwnerID != ownerID || reservation.OwnerID != ownerID || reservation.TicketID != ticketID {
		return UploadBundle{}, ErrTicketInvariant
	}
	return UploadBundle{Asset: asset, Ticket: ticket, Reservation: reservation}, nil
}

func loadUploadBundleByAsset(tx *gorm.DB, asset Asset, lock bool) (UploadBundle, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var ticket UploadTicket
	if err := query.Where("asset_id = ?", asset.ID).First(&ticket).Error; err != nil {
		return UploadBundle{}, normalizeNotFound(err)
	}
	var reservation QuotaReservation
	query = tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Where("ticket_id = ?", ticket.TicketID).First(&reservation).Error; err != nil {
		return UploadBundle{}, normalizeNotFound(err)
	}
	return UploadBundle{Asset: asset, Ticket: ticket, Reservation: reservation}, nil
}

func sameUploadDeclaration(existing, requested UploadBundle) bool {
	return existing.Asset.Purpose == requested.Asset.Purpose && existing.Ticket.ExpectedSize == requested.Ticket.ExpectedSize &&
		existing.Ticket.ExpectedMD5 == requested.Ticket.ExpectedMD5 && existing.Ticket.ExpectedContentType == requested.Ticket.ExpectedContentType
}

func enforceQuota(existing []QuotaReservation, next QuotaReservation, policy QuotaPolicy, now time.Time) error {
	if !policy.valid() {
		return ErrInvalidRecord
	}
	var activeCount, minuteCount, hourCount, dayCount uint64
	var activeBytes, minuteBytes, hourBytes, dayBytes uint64
	minuteCutoff, hourCutoff, dayCutoff := now.Add(-time.Minute), now.Add(-time.Hour), now.Add(-24*time.Hour)
	for _, value := range existing {
		if value.Status == ReservationActive {
			activeCount += uint64(value.CountReserved)
			activeBytes += value.BytesReserved
		}
		if !value.CreatedAt.Before(minuteCutoff) {
			minuteCount += uint64(value.CountReserved)
			minuteBytes += value.BytesReserved
		}
		if !value.CreatedAt.Before(hourCutoff) {
			hourCount += uint64(value.CountReserved)
			hourBytes += value.BytesReserved
		}
		if !value.CreatedAt.Before(dayCutoff) {
			dayCount += uint64(value.CountReserved)
			dayBytes += value.BytesReserved
		}
	}
	checks := []struct {
		name              string
		count, countLimit uint64
		bytes, byteLimit  uint64
	}{
		{"active", activeCount, uint64(policy.ActiveCount), activeBytes, policy.ActiveBytes},
		{"minute", minuteCount, uint64(policy.MinuteCount), minuteBytes, policy.MinuteBytes},
		{"hour", hourCount, uint64(policy.HourCount), hourBytes, policy.HourBytes},
		{"day", dayCount, uint64(policy.DayCount), dayBytes, policy.DayBytes},
	}
	for _, check := range checks {
		if check.count+uint64(next.CountReserved) > check.countLimit || check.bytes+next.BytesReserved > check.byteLimit {
			return fmt.Errorf("%w: %s", ErrQuotaExceeded, check.name)
		}
	}
	return nil
}

func updateTicketCAS(tx *gorm.DB, ticket *UploadTicket, expectedVersion uint64, from, to TicketStatus, now time.Time) error {
	if !CanTransitionTicket(from, to) {
		return ErrInvalidTransition
	}
	if err := casUpdate(tx.Model(&UploadTicket{}).
		Where("ticket_id = ? AND row_version = ? AND status = ?", ticket.TicketID, expectedVersion, from).
		Updates(map[string]any{"status": to, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})); err != nil {
		return err
	}
	ticket.Status, ticket.RowVersion, ticket.UpdatedAt = to, expectedVersion+1, now
	return nil
}

func updateReservationCAS(tx *gorm.DB, reservation *QuotaReservation, from, to ReservationStatus, now time.Time) error {
	if !CanTransitionReservation(from, to) {
		return ErrInvalidTransition
	}
	version := reservation.RowVersion
	if err := casUpdate(tx.Model(&QuotaReservation{}).
		Where("reservation_id = ? AND row_version = ? AND status = ?", reservation.ReservationID, version, from).
		Updates(map[string]any{"status": to, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})); err != nil {
		return err
	}
	reservation.Status, reservation.RowVersion, reservation.UpdatedAt = to, version+1, now
	return nil
}
