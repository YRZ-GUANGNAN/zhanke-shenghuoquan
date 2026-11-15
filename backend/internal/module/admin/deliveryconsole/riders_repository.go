package deliveryconsole

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"campus-circle/backend/internal/module/delivery"
)

// GormRiderAdmin implements RiderAdminRepository.
type GormRiderAdmin struct{ db *gorm.DB }

func NewRiderAdmin(db *gorm.DB) *GormRiderAdmin { return &GormRiderAdmin{db: db} }

var _ RiderAdminRepository = (*GormRiderAdmin)(nil)

type riderListRow struct {
	RiderUserID      uint64    `gorm:"column:rider_user_id"`
	Status           string    `gorm:"column:status"`
	RealName         string    `gorm:"column:real_name"`
	Phone            string    `gorm:"column:phone"`
	RowVersion       uint64    `gorm:"column:row_version"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	DutyState        string    `gorm:"column:duty_state"`
	InFlight         int64     `gorm:"column:in_flight"`
	TodayOrders      int64     `gorm:"column:today_orders"`
	TodayIncomeCents int64     `gorm:"column:today_income"`
	AccountStatus    string    `gorm:"column:account_status"`
	GrantActive      bool      `gorm:"column:grant_active"`
}

func (row riderListRow) view() RiderRow {
	return RiderRow{
		RiderUserID: row.RiderUserID, Status: row.Status, DutyState: row.DutyState,
		RealName: row.RealName, Phone: row.Phone, RowVersion: row.RowVersion,
		InFlight: row.InFlight, TodayOrders: row.TodayOrders, TodayIncomeCents: row.TodayIncomeCents,
		UpdatedAt: row.UpdatedAt, AccountStatus: row.AccountStatus, GrantActive: row.GrantActive,
	}
}

const riderListSelect = `
SELECT r.rider_user_id, r.status, r.real_name, r.phone, r.row_version, r.updated_at,
       COALESCE(a.state, 'off_duty') AS duty_state,
       COALESCE(u.status, 'missing') AS account_status,
       (g.id IS NOT NULL) AS grant_active,
       COALESCE(f.in_flight, 0) AS in_flight,
       COALESCE(t.today_orders, 0) AS today_orders,
       COALESCE(t.today_income, 0) AS today_income
FROM takeout_riders r
LEFT JOIN users u ON u.id = r.rider_user_id AND u.deleted_at IS NULL
LEFT JOIN user_identity_grants g ON g.user_id = r.rider_user_id AND g.role = 'rider' AND g.status = 'active'
LEFT JOIN delivery_rider_availability a ON a.rider_user_id = r.rider_user_id
LEFT JOIN (
  SELECT rider_user_id, COUNT(*) AS in_flight FROM delivery_orders
  WHERE rider_user_id IS NOT NULL AND status IN ('claimed','to_pickup','picked_up','delivering')
  GROUP BY rider_user_id
) f ON f.rider_user_id = r.rider_user_id
LEFT JOIN (
  SELECT rider_user_id, COUNT(*) AS today_orders, COALESCE(SUM(rider_fee_cents), 0) AS today_income
  FROM delivery_orders
  WHERE rider_user_id IS NOT NULL AND status = 'delivered' AND delivered_at IS NOT NULL AND DATE(delivered_at) = CURDATE()
  GROUP BY rider_user_id
) t ON t.rider_user_id = r.rider_user_id
WHERE (? = '' OR r.status = ?)
  AND (? = '' OR r.real_name LIKE ? OR r.phone LIKE ?)
`

func (r *GormRiderAdmin) ListRiders(ctx context.Context, filter RiderFilter) ([]RiderRow, int64, error) {
	like := "%" + filter.Query + "%"
	var total int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM takeout_riders r WHERE (? = '' OR r.status = ?) AND (? = '' OR r.real_name LIKE ? OR r.phone LIKE ?)`,
		filter.Status, filter.Status, filter.Query, like, like,
	).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []riderListRow
	if err := r.db.WithContext(ctx).Raw(
		riderListSelect+` ORDER BY r.status = 'active' DESC, r.rider_user_id DESC LIMIT ? OFFSET ?`,
		filter.Status, filter.Status, filter.Query, like, like,
		filter.PageSize, (filter.Page-1)*filter.PageSize,
	).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]RiderRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.view())
	}
	return result, total, nil
}

func (r *GormRiderAdmin) RiderOrders(ctx context.Context, riderUserID uint64) ([]delivery.Order, error) {
	var rows []delivery.Order
	if err := r.db.WithContext(ctx).
		Where("rider_user_id = ? AND status IN ?", riderUserID, []delivery.Status{
			delivery.StatusClaimed, delivery.StatusToPickup, delivery.StatusPickedUp, delivery.StatusDelivering,
		}).
		Order("created_at DESC").Limit(100).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormRiderAdmin) SetRiderStatus(ctx context.Context, riderUserID uint64, status string, rowVersion uint64, actor uint64, reason string, now time.Time) (RiderRow, error) {
	status = strings.TrimSpace(status)
	var result RiderRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rider riderListRow
		if err := tx.Table("takeout_riders").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("rider_user_id = ?", riderUserID).First(&rider).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return delivery.ErrNotFound
			}
			return err
		}
		if rider.RowVersion != rowVersion {
			return delivery.ErrConcurrentUpdate
		}
		if status != "active" {
			// 骑手暂停或封禁后无法登录工作台推进已接订单，必须先转出在途单。
			var inFlight int64
			if err := tx.Table("delivery_orders").
				Where("rider_user_id = ? AND status IN ?", riderUserID, []delivery.Status{
					delivery.StatusClaimed, delivery.StatusToPickup, delivery.StatusPickedUp, delivery.StatusDelivering,
				}).Count(&inFlight).Error; err != nil {
				return err
			}
			if inFlight > 0 {
				return ErrRiderHasActiveOrders
			}
		}
		if err := tx.Table("takeout_riders").Where("rider_user_id = ?", riderUserID).Updates(map[string]any{
			"status":      status,
			"row_version": gorm.Expr("row_version + 1"),
			"updated_at":  now.UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Table("delivery_rider_admin_actions").Create(map[string]any{
			"rider_user_id": riderUserID,
			"actor_user_id": actor,
			"from_status":   rider.Status,
			"to_status":     status,
			"reason":        reason,
			"created_at":    now.UTC(),
		}).Error; err != nil {
			return err
		}
		// Leaving the active pool also ends the current duty shift so no new
		// orders are broadcast to the rider.
		if status != "active" {
			if err := tx.Table("delivery_rider_availability").Where("rider_user_id = ?", riderUserID).
				Updates(map[string]any{
					"state":       "off_duty",
					"row_version": gorm.Expr("row_version + 1"),
					"updated_at":  now.UTC(),
				}).Error; err != nil {
				return err
			}
		}
		var updated []riderListRow
		if err := tx.Raw(riderListSelect+` AND r.rider_user_id = ?`, "", "", "", "%", "%", riderUserID).Scan(&updated).Error; err != nil {
			return err
		}
		for _, row := range updated {
			if row.RiderUserID == riderUserID {
				result = row.view()
				return nil
			}
		}
		return delivery.ErrNotFound
	})
	if err != nil {
		return RiderRow{}, err
	}
	return result, nil
}

// ErrRiderHasActiveOrders rejects suspension while the rider still has orders
// in flight; the console maps it to a retry-after-handover conflict.
var ErrRiderHasActiveOrders = errors.New("rider still has active orders")
