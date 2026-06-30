package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FundsRepository interface {
	Wallet(context.Context, uint64, time.Time) (MerchantWallet, error)
	ListLedger(context.Context, uint64, Page) ([]WalletLedger, int64, error)
	CreatePaymentAttempt(context.Context, *PaymentAttempt) (PaymentAttempt, error)
	FindPaymentAttemptOwned(context.Context, string, uint64) (PaymentAttempt, error)
	MarkPaymentPrepayIssued(context.Context, string, uint64, string, time.Time) (PaymentAttempt, error)
	PayOrderWithWallet(context.Context, uint64, string, uint64, string, time.Time) (FundsResult, error)
	PayOrderWithMock(context.Context, *PaymentAttempt, uint64, time.Time) (FundsResult, error)
	CreditMockRecharge(context.Context, *PaymentAttempt, time.Time) (FundsResult, error)
	ConfirmWechatPayment(context.Context, PaymentConfirmation) (FundsResult, bool, error)
	RequestRefund(context.Context, RefundRequest) (RefundResult, error)
	CompleteRefund(context.Context, string, string, bool, string, time.Time) (RefundResult, error)
	FindRefundByOutRefundNo(context.Context, string) (Refund, error)
}

var _ FundsRepository = (*GormRepository)(nil)

type FundsResult struct {
	Wallet     *MerchantWallet
	Ledger     *WalletLedger
	Attempt    *PaymentAttempt
	Settlement *FeeSettlement
	Order      *Order
	Outbox     *OutboxEvent
}

type PaymentConfirmation struct {
	OutTradeNo          string
	WechatTransactionID string
	AmountCents         uint64
	RawBody             []byte
	PaidAt              time.Time
}

type RefundRequest struct {
	MerchantUserID     uint64
	OrderPublicID      string
	IdempotencyKey     string
	Admin              bool
	ExpectedRowVersion uint64
	Now                time.Time
}

type RefundResult struct {
	Refund     Refund
	Settlement FeeSettlement
	Order      Order
	Wallet     *MerchantWallet
	Ledger     *WalletLedger
}

func (r *GormRepository) Wallet(ctx context.Context, merchantUserID uint64, now time.Time) (MerchantWallet, error) {
	var result MerchantWallet
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = lockOrCreateWalletTx(tx, merchantUserID, now)
		return err
	})
	return result, err
}

func (r *GormRepository) ListLedger(ctx context.Context, merchantUserID uint64, page Page) ([]WalletLedger, int64, error) {
	if merchantUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	query := r.db.WithContext(ctx).Model(&WalletLedger{}).Where("merchant_user_id = ?", merchantUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []WalletLedger
	err := query.Order("id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) CreatePaymentAttempt(ctx context.Context, value *PaymentAttempt) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing PaymentAttempt
		err := tx.Where("merchant_user_id = ? AND purpose = ? AND client_request_id = ?",
			value.MerchantUserID, value.Purpose, value.ClientRequestID).First(&existing).Error
		if err == nil {
			if samePaymentRequest(existing, *value) {
				result = existing
				return nil
			}
			return ErrIdempotencyConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		result = *value
		return nil
	})
	return result, err
}

func (r *GormRepository) FindPaymentAttemptOwned(ctx context.Context, publicID string, merchantUserID uint64) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Where("payment_public_id = ? AND merchant_user_id = ?", publicID, merchantUserID).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PaymentAttempt{}, ErrPaymentForbidden
	}
	return result, err
}

func (r *GormRepository) MarkPaymentPrepayIssued(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	prepayID string,
	at time.Time,
) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_public_id = ?", publicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == PaymentPrepayIssued && result.PrepayID != nil && *result.PrepayID == prepayID {
			return nil
		}
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, PaymentCreated).
			Updates(map[string]any{
				"status": PaymentPrepayIssued, "prepay_id": prepayID,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": at,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = PaymentPrepayIssued
		result.PrepayID = &prepayID
		result.RowVersion++
		result.UpdatedAt = at
		return nil
	})
	return result, err
}

func (r *GormRepository) PayOrderWithWallet(
	ctx context.Context,
	merchantUserID uint64,
	orderPublicID string,
	expectedRowVersion uint64,
	idempotencyKey string,
	now time.Time,
) (FundsResult, error) {
	var result FundsResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockOwnedOrderTx(tx, orderPublicID, merchantUserID)
		if err != nil {
			return err
		}
		if existing, found, err := findSettlementByOrderTx(tx, orderPublicID); err != nil {
			return err
		} else if found {
			if existing.Source != SettlementWallet || existing.MerchantUserID != merchantUserID {
				return ErrAlreadySettled
			}
			result.Settlement = &existing
			result.Order = &order
			return nil
		}
		if order.Status != StatusPendingFeePayment || order.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		wallet, err := lockOrCreateWalletTx(tx, merchantUserID, now)
		if err != nil {
			return err
		}
		if wallet.AvailableBalanceCents < order.DeliveryFeeCents {
			return ErrWalletInsufficient
		}
		ledger, err := newLedger(wallet, LedgerDebit, LedgerOrderFee, order.DeliveryFeeCents, "delivery_order", orderPublicID, idempotencyKey, now)
		if err != nil {
			return err
		}
		update := tx.Model(&MerchantWallet{}).
			Where("id = ? AND row_version = ? AND available_balance_cents >= ?", wallet.ID, wallet.RowVersion, order.DeliveryFeeCents).
			Updates(map[string]any{
				"available_balance_cents": gorm.Expr("available_balance_cents - ?", order.DeliveryFeeCents),
				"row_version":             gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		settlement, err := newSettlement(order, SettlementWallet, nil, &ledger.LedgerPublicID, now)
		if err != nil {
			return err
		}
		if err := tx.Create(&settlement).Error; err != nil {
			return mapSettlementConflict(err)
		}
		ready, event, err := transitionToReadyAndEnqueueTx(
			tx, orderPublicID, &merchantUserID, expectedRowVersion, StatusPendingFeePayment, nil, now,
		)
		if err != nil {
			return err
		}
		wallet.AvailableBalanceCents -= order.DeliveryFeeCents
		wallet.RowVersion++
		wallet.UpdatedAt = now
		result = FundsResult{Wallet: &wallet, Ledger: &ledger, Settlement: &settlement, Order: &ready, Outbox: &event}
		return nil
	})
	return result, err
}

func (r *GormRepository) PayOrderWithMock(
	ctx context.Context,
	attempt *PaymentAttempt,
	expectedRowVersion uint64,
	now time.Time,
) (FundsResult, error) {
	var result FundsResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockOwnedOrderTx(tx, *attempt.DeliveryOrderPublicID, attempt.MerchantUserID)
		if err != nil {
			return err
		}
		if order.DeliveryFeeCents != attempt.AmountCents {
			return ErrPaymentAmount
		}
		if existing, found, err := findSettlementByOrderTx(tx, order.DeliveryOrderPublicID); err != nil {
			return err
		} else if found {
			if existing.Source != SettlementMock {
				return ErrAlreadySettled
			}
			result = FundsResult{Attempt: attempt, Settlement: &existing, Order: &order}
			return nil
		}
		if order.Status != StatusPendingFeePayment || order.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		settlement, err := newSettlement(order, SettlementMock, &attempt.PaymentPublicID, nil, now)
		if err != nil {
			return err
		}
		if err := tx.Create(&settlement).Error; err != nil {
			return mapSettlementConflict(err)
		}
		ready, event, err := transitionToReadyAndEnqueueTx(
			tx, order.DeliveryOrderPublicID, &order.MerchantUserID, expectedRowVersion,
			StatusPendingFeePayment, nil, now,
		)
		if err != nil {
			return err
		}
		result = FundsResult{Attempt: attempt, Settlement: &settlement, Order: &ready, Outbox: &event}
		return nil
	})
	return result, err
}

func (r *GormRepository) CreditMockRecharge(ctx context.Context, attempt *PaymentAttempt, now time.Time) (FundsResult, error) {
	var result FundsResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing PaymentAttempt
		err := tx.Where("merchant_user_id = ? AND purpose = ? AND client_request_id = ?",
			attempt.MerchantUserID, PaymentWalletRecharge, attempt.ClientRequestID).First(&existing).Error
		if err == nil {
			if existing.Status != PaymentPaid || existing.AmountCents != attempt.AmountCents {
				return ErrIdempotencyConflict
			}
			wallet, walletErr := lockOrCreateWalletTx(tx, attempt.MerchantUserID, now)
			result = FundsResult{Wallet: &wallet, Attempt: &existing}
			return walletErr
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		wallet, err := lockOrCreateWalletTx(tx, attempt.MerchantUserID, now)
		if err != nil {
			return err
		}
		ledger, err := newLedger(wallet, LedgerCredit, LedgerRecharge, attempt.AmountCents, "payment", attempt.PaymentPublicID, attempt.PaymentPublicID, now)
		if err != nil {
			return err
		}
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		if err := creditWalletTx(tx, &wallet, attempt.AmountCents, now); err != nil {
			return err
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		result = FundsResult{Wallet: &wallet, Ledger: &ledger, Attempt: attempt}
		return nil
	})
	return result, err
}

func (r *GormRepository) ConfirmWechatPayment(ctx context.Context, input PaymentConfirmation) (FundsResult, bool, error) {
	var result FundsResult
	duplicate := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt PaymentAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("out_trade_no = ?", input.OutTradeNo).First(&attempt).Error; err != nil {
			return normalizeNotFound(err)
		}
		if attempt.Provider != PaymentWechat || attempt.AmountCents != input.AmountCents {
			return ErrPaymentAmount
		}
		bodyHash := sha256.Sum256(input.RawBody)
		notificationID, err := NewULID()
		if err != nil {
			return err
		}
		notification := PaymentNotification{
			NotificationPublicID: notificationID, OutTradeNo: input.OutTradeNo,
			WechatTransactionID: input.WechatTransactionID, BodySHA256: hex.EncodeToString(bodyHash[:]),
			Status: NotificationProcessed, ProcessedAt: timePointer(input.PaidAt), CreatedAt: input.PaidAt,
		}
		createNotification := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification)
		if createNotification.Error != nil {
			return createNotification.Error
		}
		duplicate = createNotification.RowsAffected == 0
		if attempt.Status == PaymentPaid {
			if attempt.WechatTransactionID == nil || *attempt.WechatTransactionID != input.WechatTransactionID {
				return ErrIdempotencyConflict
			}
			result.Attempt = &attempt
			return nil
		}
		if attempt.Status != PaymentCreated && attempt.Status != PaymentPrepayIssued {
			return ErrPaymentNotPending
		}
		transactionID := input.WechatTransactionID
		updateAttempt := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status IN ?", attempt.ID, attempt.RowVersion, []PaymentStatus{PaymentCreated, PaymentPrepayIssued}).
			Updates(map[string]any{
				"status": PaymentPaid, "wechat_transaction_id": transactionID,
				"paid_at": input.PaidAt, "updated_at": input.PaidAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if updateAttempt.Error != nil {
			return updateAttempt.Error
		}
		if updateAttempt.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		attempt.Status = PaymentPaid
		attempt.WechatTransactionID = &transactionID
		attempt.PaidAt = timePointer(input.PaidAt)
		attempt.RowVersion++
		attempt.UpdatedAt = input.PaidAt
		result.Attempt = &attempt
		if attempt.Purpose == PaymentWalletRecharge {
			wallet, err := lockOrCreateWalletTx(tx, attempt.MerchantUserID, input.PaidAt)
			if err != nil {
				return err
			}
			ledger, err := newLedger(wallet, LedgerCredit, LedgerRecharge, attempt.AmountCents, "payment", attempt.PaymentPublicID, attempt.PaymentPublicID, input.PaidAt)
			if err != nil {
				return err
			}
			if err := creditWalletTx(tx, &wallet, attempt.AmountCents, input.PaidAt); err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger).Error; err != nil {
				return err
			}
			result.Wallet, result.Ledger = &wallet, &ledger
			return nil
		}
		order, err := lockOwnedOrderTx(tx, *attempt.DeliveryOrderPublicID, attempt.MerchantUserID)
		if err != nil {
			return err
		}
		if order.DeliveryFeeCents != attempt.AmountCents {
			return ErrPaymentAmount
		}
		settlement, err := newSettlement(order, SettlementWechat, &attempt.PaymentPublicID, nil, input.PaidAt)
		if err != nil {
			return err
		}
		if err := tx.Create(&settlement).Error; err != nil {
			return mapSettlementConflict(err)
		}
		ready, event, err := transitionToReadyAndEnqueueTx(
			tx, order.DeliveryOrderPublicID, &attempt.MerchantUserID, order.RowVersion,
			StatusPendingFeePayment, nil, input.PaidAt,
		)
		if err != nil {
			return err
		}
		result.Settlement, result.Order, result.Outbox = &settlement, &ready, &event
		return nil
	})
	return result, duplicate, err
}

func (r *GormRepository) RequestRefund(ctx context.Context, request RefundRequest) (RefundResult, error) {
	var result RefundResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockOwnedOrderTx(tx, request.OrderPublicID, request.MerchantUserID)
		if err != nil {
			return err
		}
		var settlement FeeSettlement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).First(&settlement).Error; err != nil {
			return normalizeNotFound(err)
		}
		var existing Refund
		if err := tx.Where("settlement_public_id = ?", settlement.SettlementPublicID).First(&existing).Error; err == nil {
			result = RefundResult{Refund: existing, Settlement: settlement, Order: order}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if request.ExpectedRowVersion != 0 && order.RowVersion != request.ExpectedRowVersion {
			return ErrConcurrentUpdate
		}
		if !request.Admin {
			if order.Status != StatusReadyForClaim || order.RiderUserID != nil {
				return ErrRefundNotAllowed
			}
		} else if order.Status == StatusPendingFeePayment || order.Status == StatusPaymentFailed ||
			order.Status == StatusDelivered || order.Status == StatusCancelled {
			return ErrRefundNotAllowed
		}
		refund, err := newRefund(settlement, request.IdempotencyKey, request.Now)
		if err != nil {
			return err
		}
		if settlement.Source == SettlementWallet {
			wallet, err := lockOrCreateWalletTx(tx, settlement.MerchantUserID, request.Now)
			if err != nil {
				return err
			}
			ledger, err := newLedger(wallet, LedgerCredit, LedgerRefund, settlement.AmountCents, "refund", refund.RefundPublicID, request.IdempotencyKey, request.Now)
			if err != nil {
				return err
			}
			if err := creditWalletTx(tx, &wallet, settlement.AmountCents, request.Now); err != nil {
				return err
			}
			if err := tx.Create(&ledger).Error; err != nil {
				return err
			}
			refund.Status = RefundSucceeded
			refund.SucceededAt = timePointer(request.Now)
			settlement.Status = SettlementRefunded
			settlement.RefundedAt = timePointer(request.Now)
			settlement.RowVersion++
			if err := tx.Model(&FeeSettlement{}).Where("id = ? AND row_version = ?", settlement.ID, settlement.RowVersion-1).
				Updates(map[string]any{
					"status": SettlementRefunded, "refunded_at": request.Now,
					"row_version": gorm.Expr("row_version + 1"),
				}).Error; err != nil {
				return err
			}
			result.Wallet, result.Ledger = &wallet, &ledger
		} else {
			settlement.Status = SettlementRefundPending
			settlement.RowVersion++
			if settlement.Source == SettlementMock {
				refund.Status = RefundSucceeded
				refund.SucceededAt = timePointer(request.Now)
				settlement.Status = SettlementRefunded
				settlement.RefundedAt = timePointer(request.Now)
			}
			updates := map[string]any{"status": settlement.Status, "row_version": gorm.Expr("row_version + 1")}
			if settlement.RefundedAt != nil {
				updates["refunded_at"] = request.Now
			}
			if err := tx.Model(&FeeSettlement{}).Where("id = ? AND row_version = ?", settlement.ID, settlement.RowVersion-1).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&refund).Error; err != nil {
			return err
		}
		if CanTransition(order.Status, StatusCancelled) {
			updates, err := buildTransitionUpdates(order, order.Status, StatusCancelled, nil, request.Now)
			if err != nil {
				return err
			}
			updates["status"] = StatusCancelled
			updates["updated_at"] = request.Now
			updates["row_version"] = gorm.Expr("row_version + 1")
			update := tx.Model(&Order{}).Where("id = ? AND status = ? AND row_version = ?", order.ID, order.Status, order.RowVersion).Updates(updates)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ErrConcurrentUpdate
			}
			applyTransition(&order, StatusCancelled, nil, request.Now)
			order.RowVersion++
			order.UpdatedAt = request.Now
		}
		result.Refund, result.Settlement, result.Order = refund, settlement, order
		return nil
	})
	return result, err
}

func (r *GormRepository) CompleteRefund(
	ctx context.Context,
	outRefundNo string,
	wechatRefundID string,
	success bool,
	reason string,
	now time.Time,
) (RefundResult, error) {
	var result RefundResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var refund Refund
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("out_refund_no = ?", outRefundNo).First(&refund).Error; err != nil {
			return normalizeNotFound(err)
		}
		var settlement FeeSettlement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("settlement_public_id = ?", refund.SettlementPublicID).First(&settlement).Error; err != nil {
			return normalizeNotFound(err)
		}
		if refund.Status == RefundSucceeded {
			result = RefundResult{Refund: refund, Settlement: settlement}
			return nil
		}
		refundID := strings.TrimSpace(wechatRefundID)
		status := RefundRetryPending
		settlementStatus := SettlementRefundPending
		updates := map[string]any{
			"status": status, "failure_reason": truncate(reason, 512),
			"updated_at": now, "row_version": gorm.Expr("row_version + 1"),
		}
		if refundID != "" {
			updates["wechat_refund_id"] = refundID
		}
		if success {
			status = RefundSucceeded
			settlementStatus = SettlementRefunded
			updates["status"] = status
			updates["failure_reason"] = ""
			updates["succeeded_at"] = now
		}
		updateRefund := tx.Model(&Refund{}).Where("id = ? AND row_version = ?", refund.ID, refund.RowVersion).Updates(updates)
		if updateRefund.Error != nil {
			return updateRefund.Error
		}
		if updateRefund.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		settlementUpdates := map[string]any{"status": settlementStatus, "row_version": gorm.Expr("row_version + 1")}
		if success {
			settlementUpdates["refunded_at"] = now
		}
		updateSettlement := tx.Model(&FeeSettlement{}).
			Where("id = ? AND row_version = ?", settlement.ID, settlement.RowVersion).
			Updates(settlementUpdates)
		if updateSettlement.Error != nil {
			return updateSettlement.Error
		}
		if updateSettlement.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		refund.Status = status
		refund.WechatRefundID = stringPointerOrNil(refundID)
		refund.FailureReason = truncate(reason, 512)
		refund.RowVersion++
		refund.UpdatedAt = now
		settlement.Status = settlementStatus
		settlement.RowVersion++
		if success {
			refund.FailureReason = ""
			refund.SucceededAt = timePointer(now)
			settlement.RefundedAt = timePointer(now)
		}
		result = RefundResult{Refund: refund, Settlement: settlement}
		return nil
	})
	return result, err
}

func (r *GormRepository) FindRefundByOutRefundNo(ctx context.Context, outRefundNo string) (Refund, error) {
	var result Refund
	err := r.db.WithContext(ctx).Where("out_refund_no = ?", outRefundNo).First(&result).Error
	return result, normalizeNotFound(err)
}

func lockOwnedOrderTx(tx *gorm.DB, orderPublicID string, merchantUserID uint64) (Order, error) {
	var order Order
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("delivery_order_public_id = ? AND merchant_user_id = ?", orderPublicID, merchantUserID).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Order{}, ErrForbiddenOrder
	}
	return order, err
}

func lockOrCreateWalletTx(tx *gorm.DB, merchantUserID uint64, now time.Time) (MerchantWallet, error) {
	if merchantUserID == 0 {
		return MerchantWallet{}, ErrInvalidRecord
	}
	var wallet MerchantWallet
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("merchant_user_id = ?", merchantUserID).First(&wallet).Error
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return MerchantWallet{}, err
	}
	publicID, err := NewULID()
	if err != nil {
		return MerchantWallet{}, err
	}
	wallet = MerchantWallet{
		WalletPublicID: publicID, MerchantUserID: merchantUserID,
		AvailableBalanceCents: 0, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error; err != nil {
		return MerchantWallet{}, err
	}
	if wallet.ID == 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("merchant_user_id = ?", merchantUserID).First(&wallet).Error; err != nil {
			return MerchantWallet{}, err
		}
	}
	return wallet, nil
}

func creditWalletTx(tx *gorm.DB, wallet *MerchantWallet, amount uint64, now time.Time) error {
	if amount == 0 || ^uint64(0)-wallet.AvailableBalanceCents < amount {
		return ErrAmountOverflow
	}
	update := tx.Model(&MerchantWallet{}).Where("id = ? AND row_version = ?", wallet.ID, wallet.RowVersion).
		Updates(map[string]any{
			"available_balance_cents": gorm.Expr("available_balance_cents + ?", amount),
			"row_version":             gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	wallet.AvailableBalanceCents += amount
	wallet.RowVersion++
	wallet.UpdatedAt = now
	return nil
}

func newLedger(
	wallet MerchantWallet,
	direction LedgerDirection,
	entryType LedgerEntryType,
	amount uint64,
	referenceType, referenceID, idempotencyKey string,
	now time.Time,
) (WalletLedger, error) {
	publicID, err := NewULID()
	if err != nil {
		return WalletLedger{}, err
	}
	if !ValidULID(idempotencyKey) || !ValidULID(referenceID) || amount == 0 {
		return WalletLedger{}, ErrInvalidRecord
	}
	after := wallet.AvailableBalanceCents
	if direction == LedgerCredit {
		if ^uint64(0)-after < amount {
			return WalletLedger{}, ErrAmountOverflow
		}
		after += amount
	} else {
		if after < amount {
			return WalletLedger{}, ErrWalletInsufficient
		}
		after -= amount
	}
	return WalletLedger{
		LedgerPublicID: publicID, WalletPublicID: wallet.WalletPublicID, MerchantUserID: wallet.MerchantUserID,
		Direction: direction, EntryType: entryType, AmountCents: amount,
		BalanceBeforeCents: wallet.AvailableBalanceCents, BalanceAfterCents: after,
		ReferenceType: referenceType, ReferencePublicID: referenceID,
		IdempotencyKey: idempotencyKey, CreatedAt: now,
	}, nil
}

func newSettlement(order Order, source SettlementSource, paymentID, ledgerID *string, now time.Time) (FeeSettlement, error) {
	publicID, err := NewULID()
	if err != nil {
		return FeeSettlement{}, err
	}
	return FeeSettlement{
		SettlementPublicID: publicID, DeliveryOrderPublicID: order.DeliveryOrderPublicID,
		MerchantUserID: order.MerchantUserID, Source: source, AmountCents: order.DeliveryFeeCents,
		PaymentPublicID: paymentID, LedgerPublicID: ledgerID,
		Status: SettlementPaid, PaidAt: now, RowVersion: 1,
	}, nil
}

func newRefund(settlement FeeSettlement, idempotencyKey string, now time.Time) (Refund, error) {
	publicID, err := NewULID()
	if err != nil {
		return Refund{}, err
	}
	suffix, err := NewULID()
	if err != nil {
		return Refund{}, err
	}
	if !ValidULID(idempotencyKey) {
		return Refund{}, ErrInvalidULID
	}
	return Refund{
		RefundPublicID: publicID, SettlementPublicID: settlement.SettlementPublicID,
		DeliveryOrderPublicID: settlement.DeliveryOrderPublicID, MerchantUserID: settlement.MerchantUserID,
		Source: settlement.Source, AmountCents: settlement.AmountCents, Status: RefundPending,
		IdempotencyKey: idempotencyKey, OutRefundNo: "D3R" + suffix,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func findSettlementByOrderTx(tx *gorm.DB, orderPublicID string) (FeeSettlement, bool, error) {
	var result FeeSettlement
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("delivery_order_public_id = ?", orderPublicID).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FeeSettlement{}, false, nil
	}
	return result, err == nil, err
}

func samePaymentRequest(left, right PaymentAttempt) bool {
	if left.MerchantUserID != right.MerchantUserID || left.Purpose != right.Purpose ||
		left.Provider != right.Provider || left.AmountCents != right.AmountCents {
		return false
	}
	if left.DeliveryOrderPublicID == nil || right.DeliveryOrderPublicID == nil {
		return left.DeliveryOrderPublicID == nil && right.DeliveryOrderPublicID == nil
	}
	return *left.DeliveryOrderPublicID == *right.DeliveryOrderPublicID
}

func mapSettlementConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return ErrAlreadySettled
	}
	return err
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
