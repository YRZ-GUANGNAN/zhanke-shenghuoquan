package deliverypayment

import (
	"context"
	"errors"
	"strings"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

type reconciliationFunds interface {
	ClaimPaymentVerification(context.Context, string, time.Time, time.Duration) (delivery.PaymentVerificationLease, bool, error)
	ClaimNextPaymentVerification(context.Context, time.Time, time.Duration) (delivery.PaymentVerificationLease, bool, error)
	UpdatePaymentVerification(context.Context, delivery.PaymentVerificationLease, delivery.PaymentVerificationUpdate) error
	ConfirmWechatPayment(context.Context, delivery.PaymentConfirmation) (delivery.FundsResult, bool, error)
	ClaimRefundVerification(context.Context, string, time.Time, time.Duration) (delivery.RefundVerificationLease, bool, error)
	ClaimNextRefundVerification(context.Context, time.Time, time.Duration) (delivery.RefundVerificationLease, bool, error)
	UpdateRefundVerification(context.Context, delivery.RefundVerificationLease, delivery.RefundVerificationUpdate) error
	ObserveRefund(context.Context, delivery.RefundObservation) (delivery.RefundResult, error)
}

type ReconcilerConfig struct {
	LeaseTTL       time.Duration
	RetryBase      time.Duration
	MaxRetryDelay  time.Duration
	RequestTimeout time.Duration
	CloseAfter     time.Duration
	BatchSize      int
	Now            func() time.Time
}

type Reconciler struct {
	funds    reconciliationFunds
	provider Provider
	config   ReconcilerConfig
}

func NewReconciler(funds reconciliationFunds, provider Provider, config ReconcilerConfig) (*Reconciler, error) {
	if funds == nil || provider == nil || provider.Provider() != delivery.PaymentWechat {
		return nil, delivery.ErrInvalidRecord
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 30 * time.Second
	}
	if config.RetryBase <= 0 {
		config.RetryBase = 5 * time.Second
	}
	if config.MaxRetryDelay <= 0 {
		config.MaxRetryDelay = 15 * time.Minute
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 8 * time.Second
	}
	if config.CloseAfter <= 0 {
		config.CloseAfter = 30 * time.Minute
	}
	if config.BatchSize <= 0 || config.BatchSize > 100 {
		config.BatchSize = 20
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Reconciler{funds: funds, provider: provider, config: config}, nil
}

func (r *Reconciler) ReconcilePayment(ctx context.Context, paymentPublicID string) error {
	now := r.config.Now().UTC()
	lease, found, err := r.funds.ClaimPaymentVerification(
		ctx, strings.TrimSpace(paymentPublicID), now, r.config.LeaseTTL,
	)
	if err != nil || !found {
		return err
	}
	return r.reconcilePaymentLease(ctx, lease)
}

func (r *Reconciler) ReconcileRefund(ctx context.Context, refundPublicID string) error {
	now := r.config.Now().UTC()
	lease, found, err := r.funds.ClaimRefundVerification(
		ctx, strings.TrimSpace(refundPublicID), now, r.config.LeaseTTL,
	)
	if err != nil || !found {
		return err
	}
	return r.reconcileRefundLease(ctx, lease)
}

func (r *Reconciler) ReconcileBatch(ctx context.Context) (int, error) {
	processed := 0
	for processed < r.config.BatchSize {
		claimed := false
		now := r.config.Now().UTC()
		paymentLease, found, err := r.funds.ClaimNextPaymentVerification(ctx, now, r.config.LeaseTTL)
		if err != nil {
			return processed, err
		}
		if found {
			claimed = true
			processed++
			if err := r.reconcilePaymentLease(ctx, paymentLease); err != nil {
				return processed, err
			}
		}
		if processed >= r.config.BatchSize {
			break
		}
		now = r.config.Now().UTC()
		refundLease, found, err := r.funds.ClaimNextRefundVerification(ctx, now, r.config.LeaseTTL)
		if err != nil {
			return processed, err
		}
		if found {
			claimed = true
			processed++
			if err := r.reconcileRefundLease(ctx, refundLease); err != nil {
				return processed, err
			}
		}
		if !claimed {
			break
		}
	}
	return processed, nil
}

func (r *Reconciler) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.ReconcileBatch(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *Reconciler) reconcilePaymentLease(ctx context.Context, lease delivery.PaymentVerificationLease) error {
	attempt := lease.Attempt
	if attempt.PayerOpenID == nil || strings.TrimSpace(*attempt.PayerOpenID) == "" {
		return r.updatePaymentAfterProviderError(ctx, lease, ErrUntrustedProviderResponse)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.config.RequestTimeout)
	result, err := r.provider.QueryPayment(queryCtx, PaymentQueryRequest{
		OutTradeNo: attempt.OutTradeNo, AmountCents: attempt.AmountCents,
		PayerOpenID: *attempt.PayerOpenID,
	})
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return r.updatePaymentAfterProviderError(ctx, lease, err)
	}
	now := r.config.Now().UTC()
	switch result.State {
	case "SUCCESS":
		if result.PaidAt == nil || len(result.RawBody) == 0 {
			return r.updatePaymentAfterProviderError(ctx, lease, ErrUntrustedProviderResponse)
		}
		_, _, err := r.funds.ConfirmWechatPayment(ctx, delivery.PaymentConfirmation{
			OutTradeNo: result.OutTradeNo, WechatTransactionID: result.WechatTransactionID,
			PayerOpenID: result.PayerOpenID, AmountCents: result.AmountCents,
			ProviderStatus: result.State, RawBody: result.RawBody, PaidAt: result.PaidAt.UTC(),
		})
		return ignoreVerificationRace(err)
	case "NOTPAY":
		cancelRequested := attempt.Purpose == delivery.PaymentWalletRecharge && attempt.CancelRequestedAt != nil
		if cancelRequested || (!attempt.CreatedAt.IsZero() && now.Sub(attempt.CreatedAt) >= r.config.CloseAfter) {
			// Querying may consume the lease. Never issue a close using an expired
			// lease, nor let it outlive that lease and overlap a recovery worker.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt.VerifyLeaseUntil == nil || !attempt.VerifyLeaseUntil.After(now) {
				return nil
			}
			timeout := r.config.RequestTimeout
			if remaining := attempt.VerifyLeaseUntil.Sub(now); remaining < timeout {
				timeout = remaining
			}
			closeCtx, closeCancel := context.WithTimeout(ctx, timeout)
			closeErr := r.provider.ClosePayment(closeCtx, attempt.OutTradeNo)
			closeCancel()
			if closeErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return r.updatePaymentAfterProviderError(ctx, lease, closeErr)
			}
			return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
				Status: delivery.PaymentClosed, ProviderStatus: "CLOSED", UpdatedAt: r.config.Now().UTC(),
			}))
		}
		fallthrough
	case "USERPAYING":
		next := now.Add(r.retryDelay(attempt.VerifyAttemptCount))
		return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
			ProviderStatus: result.State, NextVerifyAt: &next, UpdatedAt: now,
		}))
	case "CLOSED", "REVOKED":
		return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
			Status: delivery.PaymentClosed, ProviderStatus: result.State,
			LastErrorCode: "payment_closed", UpdatedAt: now,
		}))
	case "PAYERROR":
		return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
			Status: delivery.PaymentFailed, ProviderStatus: result.State,
			LastErrorCode: "payment_failed", UpdatedAt: now,
		}))
	case "REFUND":
		return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
			Status: delivery.PaymentFailed, ProviderStatus: result.State,
			LastErrorCode: "payment_refunded_before_confirmation", UpdatedAt: now,
		}))
	default:
		return r.updatePaymentAfterProviderError(ctx, lease, ErrUntrustedProviderResponse)
	}
}

func (r *Reconciler) reconcileRefundLease(ctx context.Context, lease delivery.RefundVerificationLease) error {
	refund := lease.Refund
	if refund.OutTradeNo == nil || strings.TrimSpace(*refund.OutTradeNo) == "" {
		return r.updateRefundAfterProviderError(ctx, lease, ErrUntrustedProviderResponse)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.config.RequestTimeout)
	result, err := r.provider.QueryRefund(queryCtx, RefundQueryRequest{
		OutRefundNo: refund.OutRefundNo, OutTradeNo: *refund.OutTradeNo,
		AmountCents: refund.AmountCents,
	})
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ErrRefundNotFound) && canResubmitMissingRefund(refund) {
			now := r.config.Now().UTC()
			if refund.VerifyLeaseUntil == nil || !refund.VerifyLeaseUntil.After(now) {
				// The expired claim must be reclaimed by the scheduler. Do not
				// submit externally or overwrite another worker's verification.
				return nil
			}
			timeout := r.config.RequestTimeout
			if remaining := refund.VerifyLeaseUntil.Sub(now); remaining < timeout {
				timeout = remaining
			}
			createCtx, createCancel := context.WithTimeout(ctx, timeout)
			created, createErr := r.provider.CreateRefund(createCtx, RefundRequest{
				OutTradeNo: *refund.OutTradeNo, OutRefundNo: refund.OutRefundNo,
				AmountCents: refund.AmountCents, Reason: "配送服务取消退款",
			})
			createCancel()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if createErr == nil {
				if strings.TrimSpace(created.WechatRefundID) == "" || !supportedRefundStatus(created.Status) {
					return r.updateRefundAfterProviderError(ctx, lease, ErrUntrustedProviderResponse)
				}
				observedAt := r.config.Now().UTC()
				return ignoreVerificationRace(r.observeRefund(ctx, refund, created, observedAt))
			}
			return r.updateRefundAfterProviderError(ctx, lease, createErr)
		}
		return r.updateRefundAfterProviderError(ctx, lease, err)
	}
	observedAt := r.config.Now().UTC()
	reason := ""
	switch result.State {
	case "ABNORMAL":
		reason = "wechat refund abnormal"
	case "CLOSED":
		reason = "wechat refund closed"
	}
	_, err = r.funds.ObserveRefund(ctx, delivery.RefundObservation{
		OutRefundNo: result.OutRefundNo, OutTradeNo: result.OutTradeNo,
		WechatRefundID: result.WechatRefundID, AmountCents: result.AmountCents,
		ProviderStatus: delivery.RefundProviderStatus(result.State), FailureReason: reason,
		ObservedAt: observedAt, SucceededAt: result.SuccessAt, ExpectedRowVersion: refund.RowVersion,
	})
	return ignoreVerificationRace(err)
}

func canResubmitMissingRefund(refund delivery.Refund) bool {
	// Every claim increments this persisted counter before the external call,
	// so restarts and unknown outcomes cannot reset the three-attempt budget.
	return refund.Source == delivery.SettlementWechat && refund.Status == delivery.RefundPending &&
		refund.WechatRefundID == nil && refund.ProviderStatus == "" &&
		refund.VerifyAttemptCount > 0 && refund.VerifyAttemptCount <= 3 &&
		refund.OutRefundNo != "" && refund.AmountCents > 0
}

func (r *Reconciler) observeRefund(
	ctx context.Context,
	refund delivery.Refund,
	provider RefundProviderResult,
	observedAt time.Time,
) error {
	providerStatus := delivery.RefundProviderStatus(provider.Status)
	reason := ""
	if provider.Status == "ABNORMAL" {
		reason = "wechat refund abnormal"
	} else if provider.Status == "CLOSED" {
		reason = "wechat refund closed"
	}
	_, err := r.funds.ObserveRefund(ctx, delivery.RefundObservation{
		OutRefundNo: refund.OutRefundNo, OutTradeNo: *refund.OutTradeNo,
		WechatRefundID: provider.WechatRefundID, AmountCents: refund.AmountCents,
		ProviderStatus: providerStatus, FailureReason: reason,
		ObservedAt: observedAt, ExpectedRowVersion: refund.RowVersion,
	})
	return err
}

func (r *Reconciler) updatePaymentAfterProviderError(
	ctx context.Context,
	lease delivery.PaymentVerificationLease,
	providerErr error,
) error {
	now := r.config.Now().UTC()
	code, retry := reconciliationError(providerErr)
	var next *time.Time
	if retry {
		value := now.Add(r.retryDelay(lease.Attempt.VerifyAttemptCount))
		next = &value
	}
	return ignoreVerificationRace(r.funds.UpdatePaymentVerification(ctx, lease, delivery.PaymentVerificationUpdate{
		NextVerifyAt: next, LastErrorCode: code, UpdatedAt: now,
	}))
}

func (r *Reconciler) updateRefundAfterProviderError(
	ctx context.Context,
	lease delivery.RefundVerificationLease,
	providerErr error,
) error {
	now := r.config.Now().UTC()
	code, retry := reconciliationError(providerErr)
	var next *time.Time
	if retry {
		value := now.Add(r.retryDelay(lease.Refund.VerifyAttemptCount))
		next = &value
	}
	return ignoreVerificationRace(r.funds.UpdateRefundVerification(ctx, lease, delivery.RefundVerificationUpdate{
		NextVerifyAt: next, LastErrorCode: code, UpdatedAt: now,
	}))
}

func (r *Reconciler) retryDelay(attempt uint32) time.Duration {
	if attempt == 0 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 8 {
		shift = 8
	}
	delay := r.config.RetryBase * time.Duration(1<<shift)
	if delay > r.config.MaxRetryDelay {
		return r.config.MaxRetryDelay
	}
	return delay
}

func reconciliationError(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrProviderNotConfigured):
		return "provider_not_configured", false
	case errors.Is(err, ErrRefundNotFound):
		return "provider_not_found", false
	case errors.Is(err, ErrUntrustedProviderResponse), errors.Is(err, ErrInvalidNotification):
		return "untrusted_provider_response", true
	case errors.Is(err, ErrProviderRejected):
		return "provider_rejected", true
	default:
		return "provider_unavailable", true
	}
}

func ignoreVerificationRace(err error) error {
	if errors.Is(err, delivery.ErrVerificationLease) || errors.Is(err, delivery.ErrConcurrentUpdate) ||
		errors.Is(err, delivery.ErrPaymentNotPending) {
		return nil
	}
	return err
}
