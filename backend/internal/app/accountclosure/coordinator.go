// Package accountclosure coordinates the cross-domain, atomic account closure.
// It owns no domain model and keeps historical/audit facts intact.
package accountclosure

import (
	"context"
	"fmt"
	"time"

	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Coordinator struct {
	db  *gorm.DB
	now func() time.Time
}

func New(db *gorm.DB) *Coordinator { return &Coordinator{db: db, now: time.Now} }

func (c *Coordinator) Close(ctx context.Context, userID uint) ([]user.AccountClosureBlocker, error) {
	if c == nil || c.db == nil || userID == 0 {
		return nil, fmt.Errorf("invalid account closure coordinator")
	}
	var blockers []user.AccountClosureBlocker
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account user.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, userID).Error; err != nil {
			return err
		}
		if account.Status == user.StatusClosed {
			return nil
		}
		checks := []struct {
			code, message, query string
			args                 []any
		}{
			{"delivery_order", "仍有未完成的配送订单", "SELECT COUNT(*) FROM delivery_orders WHERE (merchant_user_id = ? OR rider_user_id = ?) AND status NOT IN ('delivered','cancelled')", []any{userID, userID}},
			{"delivery_payment", "仍有处理中配送支付", "SELECT COUNT(*) FROM delivery_payment_attempts WHERE merchant_user_id = ? AND status IN ('created','prepay_issued')", []any{userID}},
			{"delivery_refund", "仍有处理中配送退款", "SELECT COUNT(*) FROM delivery_fee_settlements WHERE merchant_user_id = ? AND status = 'refund_pending'", []any{userID}},
			{"delivery_refund", "仍有处理中配送退款", "SELECT COUNT(*) FROM delivery_refunds WHERE merchant_user_id = ? AND status IN ('pending','refund_pending')", []any{userID}},
			{"delivery_balance", "配送商家钱包仍有余额", "SELECT COUNT(*) FROM delivery_merchant_wallets WHERE merchant_user_id = ? AND available_balance_cents > 0", []any{userID}},
			{"delivery_rider_balance", "配送骑手钱包仍有余额", "SELECT COUNT(*) FROM delivery_rider_wallets WHERE rider_user_id = ? AND available_balance_cents > 0", []any{userID}},
			{"delivery_rider_earning", "仍有未结清的配送骑手收入", "SELECT COUNT(*) FROM delivery_rider_earning_records WHERE rider_user_id = ? AND settlement_status <> 'paid_offline'", []any{userID}},
			{"takeout_order", "仍有未完成的外卖订单", "SELECT COUNT(*) FROM takeout_orders o LEFT JOIN takeout_merchants m ON m.merchant_public_id = o.merchant_public_id WHERE (o.buyer_user_id = ? OR o.rider_user_id = ? OR m.owner_user_id = ?) AND o.status NOT IN ('completed','cancelled','refunded')", []any{userID, userID, userID}},
			{"takeout_payment", "仍有处理中的外卖支付", "SELECT COUNT(*) FROM takeout_payment_attempts WHERE buyer_user_id = ? AND status IN ('created','prepay_issued')", []any{userID}},
			{"takeout_withdraw", "仍有待处理的外卖提现", "SELECT COUNT(*) FROM takeout_withdraw_requests WHERE rider_user_id = ? AND status = 'pending'", []any{userID}},
			{"takeout_earning", "仍有未结算的外卖骑手收入", "SELECT COUNT(*) FROM takeout_rider_earning_records WHERE rider_user_id = ? AND settlement_status = 'pending_settlement'", []any{userID}},
		}
		seen := make(map[string]struct{}, len(checks))
		for _, check := range checks {
			var count int64
			if err := tx.Raw(check.query, check.args...).Scan(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				if _, exists := seen[check.code]; !exists {
					blockers = append(blockers, user.AccountClosureBlocker{Code: check.code, Message: check.message})
					seen[check.code] = struct{}{}
				}
			}
		}
		if len(blockers) > 0 {
			return nil
		}

		now := c.now().UTC()
		openID := fmt.Sprintf("closed:%d:%d", account.ID, account.TokenVersion+1)
		updates := map[string]any{
			"open_id": openID, "phone": nil, "nickname": "账号已注销", "avatar": "", "avatar_asset_id": nil,
			"school": "", "campus": "", "current_identity": "student", "role": "user", "status": user.StatusClosed,
			"token_version": gorm.Expr("token_version + 1"), "closed_at": now,
		}
		if err := tx.Table("users").Where("id = ?", userID).Updates(updates).Error; err != nil {
			return err
		}
		for _, deletion := range []struct {
			table string
			where string
			args  []any
		}{
			{"user_settings", "user_id = ?", []any{userID}},
			{"user_blocks", "user_id = ? OR blocked_user_id = ?", []any{userID, userID}},
			{"community_search_histories", "user_id = ?", []any{userID}},
			{"trade_search_histories", "user_id = ?", []any{userID}},
			{"navigation_search_histories", "user_id = ?", []any{userID}},
			{"user_follows", "follower_id = ? OR followee_id = ?", []any{userID, userID}},
		} {
			query := "DELETE FROM " + deletion.table + " WHERE " + deletion.where
			if err := tx.Exec(query, deletion.args...).Error; err != nil {
				return err
			}
		}
		if err := tx.Table("products").Where("seller_id = ? AND status <> ? AND deleted_at IS NULL", userID, "sold").Update("deleted_at", now).Error; err != nil {
			return err
		}
		if err := tx.Table("takeout_merchants").Where("owner_user_id = ?", userID).Updates(map[string]any{"status": "suspended", "row_version": gorm.Expr("row_version + 1"), "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Table("takeout_riders").Where("rider_user_id = ?", userID).Updates(map[string]any{"status": "suspended", "row_version": gorm.Expr("row_version + 1"), "updated_at": now}).Error
	})
	return blockers, err
}

var _ user.AccountClosureCoordinator = (*Coordinator)(nil)
