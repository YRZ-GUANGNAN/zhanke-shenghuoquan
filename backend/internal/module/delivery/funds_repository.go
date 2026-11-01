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
	RequestRechargeCancellation(context.Context, string, uint64, time.Time) (PaymentAttempt, error)
	RequestRechargeCancellationByRequest(context.Context, *PaymentAttempt, time.Time) (PaymentAttempt, error)
	FindRechargeByRequestOwned(context.Context, uint64, string, uint64) (*PaymentAttempt, error)
	MarkPaymentPrepayRequested(context.Context, string, uint64, string, time.Time) (PaymentAttempt, error)
	MarkPaymentPrepayIssued(context.Context, string, uint64, string, string, time.Time) (PaymentAttempt, error)
	PayOrderWithWallet(context.Context, uint64, string, uint64, string, time.Time) (FundsResult, error)
	PayOrderWithMock(context.Context, *PaymentAttempt, uint64, time.Time) (FundsResult, error)
	CreditMockRecharge(context.Context, *PaymentAttempt, time.Time) (FundsResult, error)
	ConfirmWechatPayment(context.Context, PaymentConfirmation) (FundsResult, bool, error)
	RequestRefund(context.Context, RefundRequest) (RefundResult, error)
	ObserveRefund(context.Context, RefundObservation) (RefundResult, error)
	ClaimPaymentVerification(context.Context, string, time.Time, time.Duration) (PaymentVerificationLease, bool, error)
	ClaimNextPaymentVerification(context.Context, time.Time, time.Duration) (PaymentVerificationLease, bool, error)
	UpdatePaymentVerification(context.Context, PaymentVerificationLease, PaymentVerificationUpdate) error
	ClaimRefundVerification(context.Context, string, time.Time, time.Duration) (RefundVerificationLease, bool, error)
	ClaimNextRefundVerification(context.Context, time.Time, time.Duration) (RefundVerificationLease, bool, error)
	UpdateRefundVerification(context.Context, RefundVerificationLease, RefundVerificationUpdate) error
	FindRefundByOutRefundNo(context.Context, string) (Refund, error)
	FindOrderFundsOwned(context.Context, string, uint64) (OrderFundsSnapshot, error)
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
	PayerOpenID         string
	AmountCents         uint64
	ProviderStatus      string
	RawBody             []byte
	PaidAt              time.Time
}

type RefundObservation struct {
	OutRefundNo        string
	OutTradeNo         string
	WechatRefundID     string
	AmountCents        uint64
	ProviderStatus     RefundProviderStatus
	FailureReason      string
	ObservedAt         time.Time
	SucceededAt        *time.Time
	ExpectedRowVersion uint64
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

type OrderFundsSnapshot struct {
	Order      Order
	Settlement *FeeSettlement
	Refund     *Refund
	Payment    *PaymentAttempt
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
		if value.Purpose == PaymentOrderFee {
			if value.DeliveryOrderPublicID == nil {
				return ErrInvalidRecord
			}
			var order Order
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("delivery_order_public_id = ? AND merchant_user_id = ?", *value.DeliveryOrderPublicID, value.MerchantUserID).
				First(&order).Error; err != nil {
				return normalizeNotFound(err)
			}
			if order.Status != StatusPendingFeePayment || order.DeliveryFeeCents != value.AmountCents {
				return ErrPaymentNotPending
			}
			var active PaymentAttempt
			err := tx.Where(
				"delivery_order_public_id = ? AND merchant_user_id = ? AND provider = ? AND status IN ?",
				*value.DeliveryOrderPublicID, value.MerchantUserID, PaymentWechat,
				[]PaymentStatus{PaymentCreated, PaymentPrepayIssued},
			).Order("id DESC").First(&active).Error
			if err == nil {
				if active.AmountCents != value.AmountCents || active.Currency != value.Currency {
					return ErrIdempotencyConflict
				}
				result = active
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
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

func (r *GormRepository) MarkPaymentPrepayRequested(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	payerOpenID string,
	at time.Time,
) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_public_id = ?", publicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status != PaymentCreated || result.CancelRequestedAt != nil {
			return ErrPaymentNotPending
		}
		if result.PayerOpenID != nil && *result.PayerOpenID != payerOpenID {
			return ErrPaymentPayer
		}
		if result.PayerOpenID != nil && *result.PayerOpenID == payerOpenID &&
			result.ProviderStatus == "PREPAY_REQUESTED" {
			return nil
		}
		nextVerifyAt := at.Add(30 * time.Second)
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, PaymentCreated).
			Updates(map[string]any{
				"payer_openid": payerOpenID, "provider_status": "PREPAY_REQUESTED",
				"next_verify_at": nextVerifyAt, "verify_lease_until": nil,
				"last_error_code": "", "row_version": gorm.Expr("row_version + 1"),
				"updated_at": at,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.PayerOpenID = &payerOpenID
		result.ProviderStatus = "PREPAY_REQUESTED"
		result.NextVerifyAt = &nextVerifyAt
		result.VerifyLeaseUntil = nil
		result.LastErrorCode = ""
		result.RowVersion++
		result.UpdatedAt = at
		return nil
	})
	return result, err
}

func (r *GormRepository) MarkPaymentPrepayIssued(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	prepayID string,
	payerOpenID string,
	at time.Time,
) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_public_id = ?", publicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.CancelRequestedAt != nil {
			return ErrPaymentNotPending
		}
		if result.Status == PaymentPrepayIssued && result.PrepayID != nil && *result.PrepayID == prepayID {
			if result.PayerOpenID == nil || *result.PayerOpenID != payerOpenID {
				return ErrPaymentPayer
			}
			return nil
		}
		if result.PayerOpenID != nil && *result.PayerOpenID != payerOpenID {
			return ErrPaymentPayer
		}
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, PaymentCreated).
			Updates(map[string]any{
				"status": PaymentPrepayIssued, "prepay_id": prepayID,
				"payer_openid": payerOpenID, "provider_status": "PREPAY_ISSUED",
				"next_verify_at": at.Add(30 * time.Second), "last_error_code": "",
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
		result.PayerOpenID = &payerOpenID
		result.ProviderStatus = "PREPAY_ISSUED"
		result.NextVerifyAt = timePointer(at.Add(30 * time.Second))
		result.LastErrorCode = ""
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
		// An issued WeChat payment may still succeed after the client cancels or
		// loses connectivity. Do not charge the wallet while that original
		// provider order is unresolved. The order row lock also serializes this
		// check against CreatePaymentAttempt and payment confirmation.
		var unresolved int64
		if err := unresolvedOrderWechatPayments(tx, orderPublicID, merchantUserID).Count(&unresolved).Error; err != nil {
			return err
		}
		if unresolved > 0 {
			return ErrPaymentNotPending
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
		if _, err := allocateWalletFundingTx(tx, wallet, settlement, now); err != nil {
			return err
		}
		ready, event, err := transitionToReadyAndEnqueueTx(
			tx, orderPublicID, &merchantUserID, expectedRowVersion, StatusPendingFeePayment, nil, now, r.notificationAppender,
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

func unresolvedOrderWechatPayments(tx *gorm.DB, orderPublicID string, merchantUserID uint64) *gorm.DB {
	return tx.Model(&PaymentAttempt{}).
		Where("delivery_order_public_id = ? AND merchant_user_id = ? AND provider = ? AND status IN ?",
			orderPublicID, merchantUserID, PaymentWechat, []PaymentStatus{PaymentCreated, PaymentPrepayIssued})
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
			StatusPendingFeePayment, nil, now, r.notificationAppender,
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
		lots, available, err := lockWalletFundingLotsTx(tx, wallet)
		if err != nil {
			return err
		}
		if _, _, err := ensureWalletFundingCoverageTx(tx, wallet, lots, available, now); err != nil {
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
		if _, err := createRechargeFundingLotTx(tx, wallet, *attempt, now); err != nil {
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
		if err := validatePaymentObservation(attempt, input); err != nil {
			return err
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
				"provider_status": input.ProviderStatus, "next_verify_at": nil,
				"verify_lease_until": nil, "last_error_code": "",
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
		attempt.ProviderStatus = input.ProviderStatus
		attempt.NextVerifyAt = nil
		attempt.VerifyLeaseUntil = nil
		attempt.LastErrorCode = ""
		attempt.RowVersion++
		attempt.UpdatedAt = input.PaidAt
		result.Attempt = &attempt
		if attempt.Purpose == PaymentWalletRecharge {
			wallet, err := lockOrCreateWalletTx(tx, attempt.MerchantUserID, input.PaidAt)
			if err != nil {
				return err
			}
			lots, available, err := lockWalletFundingLotsTx(tx, wallet)
			if err != nil {
				return err
			}
			if _, _, err := ensureWalletFundingCoverageTx(tx, wallet, lots, available, input.PaidAt); err != nil {
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
			if _, err := createRechargeFundingLotTx(tx, wallet, attempt, input.PaidAt); err != nil {
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
			StatusPendingFeePayment, nil, input.PaidAt, r.notificationAppender,
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
		if settlement.Source == SettlementWechat {
			if settlement.PaymentPublicID == nil {
				return ErrRefundObservation
			}
			var attempt PaymentAttempt
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("payment_public_id = ?", *settlement.PaymentPublicID).First(&attempt).Error; err != nil {
				return normalizeNotFound(err)
			}
			if attempt.Status != PaymentPaid || attempt.OutTradeNo == "" ||
				attempt.AmountCents != settlement.AmountCents {
				return ErrRefundObservation
			}
			refund.OutTradeNo = stringPointerOrNil(attempt.OutTradeNo)
			refund.NextVerifyAt = timePointer(request.Now)
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
			if err := restoreWalletFundingTx(tx, wallet, settlement, request.Now); err != nil {
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
			before := order
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
			if err := appendOrderNotificationTx(tx, r.notificationAppender, before, order, request.Now); err != nil {
				return err
			}
		}
		result.Refund, result.Settlement, result.Order = refund, settlement, order
		return nil
	})
	return result, err
}

func (r *GormRepository) ObserveRefund(ctx context.Context, input RefundObservation) (RefundResult, error) {
	var result RefundResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var refund Refund
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("out_refund_no = ?", input.OutRefundNo).First(&refund).Error; err != nil {
			return normalizeNotFound(err)
		}
		var settlement FeeSettlement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("settlement_public_id = ?", refund.SettlementPublicID).First(&settlement).Error; err != nil {
			return normalizeNotFound(err)
		}
		if err := validateRefundObservation(refund, input); err != nil {
			return err
		}
		if refund.Status == RefundSucceeded {
			if input.ProviderStatus != RefundProviderSucceeded {
				result = RefundResult{Refund: refund, Settlement: settlement}
				return nil
			}
			if refund.WechatRefundID == nil || *refund.WechatRefundID != input.WechatRefundID {
				return ErrRefundObservation
			}
			result = RefundResult{Refund: refund, Settlement: settlement}
			return nil
		}
		if input.ExpectedRowVersion != 0 &&
			(refund.RowVersion != input.ExpectedRowVersion || refund.VerifyLeaseUntil == nil ||
				refund.VerifyLeaseUntil.Before(input.ObservedAt)) {
			return ErrVerificationLease
		}
		if refund.Status == RefundFailed && input.ProviderStatus != RefundProviderSucceeded {
			result = RefundResult{Refund: refund, Settlement: settlement}
			return nil
		}
		refundID := strings.TrimSpace(input.WechatRefundID)
		status := RefundRetryPending
		settlementStatus := SettlementRefundPending
		succeededAt := refundObservationSucceededAt(input)
		updates := map[string]any{
			"status": status, "wechat_refund_id": refundID,
			"provider_status":    string(input.ProviderStatus),
			"failure_reason":     truncate(input.FailureReason, 512),
			"verify_lease_until": nil, "updated_at": input.ObservedAt,
			"row_version": gorm.Expr("row_version + 1"),
		}
		switch input.ProviderStatus {
		case RefundProviderSucceeded:
			status = RefundSucceeded
			settlementStatus = SettlementRefunded
			updates["status"] = status
			updates["failure_reason"] = ""
			updates["last_error_code"] = ""
			updates["next_verify_at"] = nil
			updates["succeeded_at"] = succeededAt
		case RefundProviderProcessing:
			updates["next_verify_at"] = input.ObservedAt.Add(30 * time.Second)
			updates["last_error_code"] = ""
		case RefundProviderAbnormal:
			status = RefundFailed
			updates["status"] = status
			updates["next_verify_at"] = nil
			updates["last_error_code"] = "refund_abnormal"
		case RefundProviderClosed:
			status = RefundFailed
			updates["status"] = status
			updates["next_verify_at"] = nil
			updates["last_error_code"] = "refund_closed"
		}
		updateRefund := tx.Model(&Refund{}).Where("id = ? AND row_version = ?", refund.ID, refund.RowVersion).Updates(updates)
		if updateRefund.Error != nil {
			return updateRefund.Error
		}
		if updateRefund.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		settlementUpdates := map[string]any{"status": settlementStatus, "row_version": gorm.Expr("row_version + 1")}
		if input.ProviderStatus == RefundProviderSucceeded {
			settlementUpdates["refunded_at"] = succeededAt
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
		refund.ProviderStatus = string(input.ProviderStatus)
		refund.FailureReason = truncate(input.FailureReason, 512)
		refund.VerifyLeaseUntil = nil
		refund.RowVersion++
		refund.UpdatedAt = input.ObservedAt
		settlement.Status = settlementStatus
		settlement.RowVersion++
		switch input.ProviderStatus {
		case RefundProviderSucceeded:
			refund.FailureReason = ""
			refund.LastErrorCode = ""
			refund.NextVerifyAt = nil
			refund.SucceededAt = timePointer(succeededAt)
			settlement.RefundedAt = timePointer(succeededAt)
		case RefundProviderProcessing:
			refund.LastErrorCode = ""
			refund.NextVerifyAt = timePointer(input.ObservedAt.Add(30 * time.Second))
		case RefundProviderAbnormal:
			refund.LastErrorCode = "refund_abnormal"
			refund.NextVerifyAt = nil
		case RefundProviderClosed:
			refund.LastErrorCode = "refund_closed"
			refund.NextVerifyAt = nil
		}
		result = RefundResult{Refund: refund, Settlement: settlement}
		return nil
	})
	return result, err
}

func (r *GormRepository) ClaimPaymentVerification(
	ctx context.Context,
	paymentPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (PaymentVerificationLease, bool, error) {
	return r.claimPaymentVerification(ctx, paymentPublicID, now, leaseTTL)
}

func (r *GormRepository) ClaimNextPaymentVerification(
	ctx context.Context,
	now time.Time,
	leaseTTL time.Duration,
) (PaymentVerificationLease, bool, error) {
	return r.claimPaymentVerification(ctx, "", now, leaseTTL)
}

func (r *GormRepository) claimPaymentVerification(
	ctx context.Context,
	paymentPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (PaymentVerificationLease, bool, error) {
	var lease PaymentVerificationLease
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ?", []PaymentStatus{PaymentCreated, PaymentPrepayIssued}).
			Where("payer_openid IS NOT NULL AND payer_openid <> ''").
			Where("next_verify_at IS NOT NULL AND next_verify_at <= ?", now).
			Where("verify_lease_until IS NULL OR verify_lease_until <= ?", now)
		if paymentPublicID != "" {
			query = query.Where("payment_public_id = ?", paymentPublicID)
		}
		var attempt PaymentAttempt
		err := query.Order("id ASC").First(&attempt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		expiresAt := now.Add(leaseTTL)
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status IN ?", attempt.ID, attempt.RowVersion,
				[]PaymentStatus{PaymentCreated, PaymentPrepayIssued}).
			Updates(map[string]any{
				"verify_lease_until":   expiresAt,
				"verify_attempt_count": gorm.Expr("verify_attempt_count + 1"),
				"row_version":          gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrVerificationLease
		}
		attempt.VerifyLeaseUntil = &expiresAt
		attempt.VerifyAttemptCount++
		attempt.RowVersion++
		attempt.UpdatedAt = now
		lease = PaymentVerificationLease{Attempt: attempt}
		found = true
		return nil
	})
	return lease, found, err
}

func (r *GormRepository) UpdatePaymentVerification(
	ctx context.Context,
	lease PaymentVerificationLease,
	update PaymentVerificationUpdate,
) error {
	if lease.Attempt.ID == 0 || lease.Attempt.RowVersion == 0 || lease.Attempt.VerifyLeaseUntil == nil ||
		update.UpdatedAt.IsZero() || (update.Status != "" && update.Status != PaymentFailed && update.Status != PaymentClosed) {
		return ErrInvalidRecord
	}
	updates := map[string]any{
		"next_verify_at": update.NextVerifyAt, "verify_lease_until": nil,
		"last_error_code": truncate(strings.TrimSpace(update.LastErrorCode), 64),
		"row_version":     gorm.Expr("row_version + 1"), "updated_at": update.UpdatedAt,
	}
	if providerStatus := strings.TrimSpace(update.ProviderStatus); providerStatus != "" {
		updates["provider_status"] = truncate(providerStatus, 32)
	}
	if update.Status != "" {
		updates["status"] = update.Status
		updates["next_verify_at"] = nil
		if update.Status == PaymentClosed {
			updates["closed_at"] = update.UpdatedAt
		}
	}
	result := r.db.WithContext(ctx).Model(&PaymentAttempt{}).
		Where("id = ? AND row_version = ? AND status IN ? AND verify_lease_until >= ?",
			lease.Attempt.ID, lease.Attempt.RowVersion,
			[]PaymentStatus{PaymentCreated, PaymentPrepayIssued}, update.UpdatedAt).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrVerificationLease
	}
	return nil
}

func (r *GormRepository) ClaimRefundVerification(
	ctx context.Context,
	refundPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (RefundVerificationLease, bool, error) {
	return r.claimRefundVerification(ctx, refundPublicID, now, leaseTTL)
}

func (r *GormRepository) ClaimNextRefundVerification(
	ctx context.Context,
	now time.Time,
	leaseTTL time.Duration,
) (RefundVerificationLease, bool, error) {
	return r.claimRefundVerification(ctx, "", now, leaseTTL)
}

func (r *GormRepository) claimRefundVerification(
	ctx context.Context,
	refundPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (RefundVerificationLease, bool, error) {
	var lease RefundVerificationLease
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("source = ?", SettlementWechat).
			Where("status IN ?", []RefundStatus{RefundPending, RefundRetryPending}).
			Where("out_trade_no IS NOT NULL AND out_trade_no <> ''").
			Where("next_verify_at IS NOT NULL AND next_verify_at <= ?", now).
			Where("verify_lease_until IS NULL OR verify_lease_until <= ?", now)
		if refundPublicID != "" {
			query = query.Where("refund_public_id = ?", refundPublicID)
		}
		var refund Refund
		err := query.Order("id ASC").First(&refund).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		expiresAt := now.Add(leaseTTL)
		update := tx.Model(&Refund{}).
			Where("id = ? AND row_version = ? AND status IN ?", refund.ID, refund.RowVersion,
				[]RefundStatus{RefundPending, RefundRetryPending}).
			Updates(map[string]any{
				"verify_lease_until":   expiresAt,
				"verify_attempt_count": gorm.Expr("verify_attempt_count + 1"),
				"row_version":          gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrVerificationLease
		}
		refund.VerifyLeaseUntil = &expiresAt
		refund.VerifyAttemptCount++
		refund.RowVersion++
		refund.UpdatedAt = now
		lease = RefundVerificationLease{Refund: refund}
		found = true
		return nil
	})
	return lease, found, err
}

func (r *GormRepository) UpdateRefundVerification(
	ctx context.Context,
	lease RefundVerificationLease,
	update RefundVerificationUpdate,
) error {
	if lease.Refund.ID == 0 || lease.Refund.RowVersion == 0 || lease.Refund.VerifyLeaseUntil == nil ||
		update.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	updates := map[string]any{
		"next_verify_at": update.NextVerifyAt, "verify_lease_until": nil,
		"last_error_code": truncate(strings.TrimSpace(update.LastErrorCode), 64),
		"row_version":     gorm.Expr("row_version + 1"), "updated_at": update.UpdatedAt,
	}
	if providerStatus := strings.TrimSpace(update.ProviderStatus); providerStatus != "" {
		updates["provider_status"] = truncate(providerStatus, 32)
	}
	result := r.db.WithContext(ctx).Model(&Refund{}).
		Where("id = ? AND row_version = ? AND status IN ? AND verify_lease_until >= ?",
			lease.Refund.ID, lease.Refund.RowVersion,
			[]RefundStatus{RefundPending, RefundRetryPending}, update.UpdatedAt).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrVerificationLease
	}
	return nil
}

func validRefundProviderStatus(value RefundProviderStatus) bool {
	switch value {
	case RefundProviderProcessing, RefundProviderSucceeded, RefundProviderClosed, RefundProviderAbnormal:
		return true
	default:
		return false
	}
}

func validatePaymentObservation(attempt PaymentAttempt, input PaymentConfirmation) error {
	if attempt.Provider != PaymentWechat || attempt.AmountCents != input.AmountCents {
		return ErrPaymentAmount
	}
	if attempt.PayerOpenID == nil || *attempt.PayerOpenID != input.PayerOpenID {
		return ErrPaymentPayer
	}
	return nil
}

func validateRefundObservation(refund Refund, input RefundObservation) error {
	if refund.Source != SettlementWechat || refund.OutTradeNo == nil ||
		*refund.OutTradeNo != input.OutTradeNo || refund.AmountCents != input.AmountCents ||
		strings.TrimSpace(input.WechatRefundID) == "" || !validRefundProviderStatus(input.ProviderStatus) {
		return ErrRefundObservation
	}
	if refund.WechatRefundID != nil && *refund.WechatRefundID != input.WechatRefundID {
		return ErrRefundObservation
	}
	return nil
}

func refundObservationSucceededAt(input RefundObservation) time.Time {
	if input.ProviderStatus == RefundProviderSucceeded && input.SucceededAt != nil && !input.SucceededAt.IsZero() {
		return input.SucceededAt.UTC()
	}
	return input.ObservedAt
}

func (r *GormRepository) FindRefundByOutRefundNo(ctx context.Context, outRefundNo string) (Refund, error) {
	var result Refund
	err := r.db.WithContext(ctx).Where("out_refund_no = ?", outRefundNo).First(&result).Error
	return result, normalizeNotFound(err)
}

func (r *GormRepository) FindOrderFundsOwned(
	ctx context.Context,
	orderPublicID string,
	merchantUserID uint64,
) (OrderFundsSnapshot, error) {
	var result OrderFundsSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"delivery_order_public_id = ? AND merchant_user_id = ?",
			orderPublicID,
			merchantUserID,
		).First(&result.Order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrForbiddenOrder
			}
			return err
		}
		var latestPayment PaymentAttempt
		if err := tx.Where(
			"delivery_order_public_id = ? AND merchant_user_id = ?",
			orderPublicID,
			merchantUserID,
		).Order("id DESC").First(&latestPayment).Error; err == nil {
			result.Payment = &latestPayment
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var settlement FeeSettlement
		if err := tx.Where(
			"delivery_order_public_id = ? AND merchant_user_id = ?",
			orderPublicID,
			merchantUserID,
		).First(&settlement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		result.Settlement = &settlement

		var refund Refund
		if err := tx.Where(
			"settlement_public_id = ? AND merchant_user_id = ?",
			settlement.SettlementPublicID,
			merchantUserID,
		).First(&refund).Error; err == nil {
			result.Refund = &refund
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if settlement.PaymentPublicID == nil {
			return nil
		}
		if result.Payment != nil && result.Payment.PaymentPublicID == *settlement.PaymentPublicID {
			return nil
		}
		var settledPayment PaymentAttempt
		if err := tx.Where(
			"payment_public_id = ? AND merchant_user_id = ?",
			*settlement.PaymentPublicID,
			merchantUserID,
		).First(&settledPayment).Error; err != nil {
			return normalizeNotFound(err)
		}
		result.Payment = &settledPayment
		return nil
	})
	return result, err
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
	// Both callers lock the owning order before their first consistent read.
	// That serializes settlement creation for this order; locking a missing
	// settlement would hold an InnoDB gap while waiting for the merchant wallet,
	// which can deadlock payments for different orders of the same merchant.
	err := tx.Where("delivery_order_public_id = ?", orderPublicID).First(&result).Error
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
