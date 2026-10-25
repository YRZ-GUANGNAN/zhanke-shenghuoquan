package delivery

import (
	"errors"
	"testing"
	"time"
)

func TestFundingLotAvailableCentsPreservesConservation(t *testing.T) {
	lot := FundingLot{
		TotalAmountCents:     500,
		AllocatedAmountCents: 350,
		RefundedAmountCents:  150,
	}
	available, err := fundingLotAvailableCents(lot)
	if err != nil {
		t.Fatal(err)
	}
	if available != 300 {
		t.Fatalf("available=%d want=300", available)
	}

	lot.RefundedAmountCents = 351
	if _, err := fundingLotAvailableCents(lot); !errors.Is(err, ErrFundingSourceMismatch) {
		t.Fatalf("refunded beyond allocated returned %v", err)
	}
	lot = FundingLot{TotalAmountCents: 100, AllocatedAmountCents: 101}
	if _, err := fundingLotAvailableCents(lot); !errors.Is(err, ErrFundingSourceMismatch) {
		t.Fatalf("net consumption beyond total returned %v", err)
	}
}

func TestFundingLotValidationNeverPromotesMockOrHistoricalMoney(t *testing.T) {
	now := time.Now().UTC()
	lotID, _ := NewULID()
	walletID, _ := NewULID()
	paymentID, _ := NewULID()
	base := FundingLot{
		FundingLotPublicID: lotID,
		WalletPublicID:     walletID,
		MerchantUserID:     1,
		Source:             FundingSourceWechatRecharge,
		VerificationStatus: FundingVerificationVerified,
		PaymentPublicID:    &paymentID,
		TotalAmountCents:   500,
		Currency:           "CNY",
		RowVersion:         1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := validateFundingLot(base); err != nil {
		t.Fatalf("valid verified WeChat lot rejected: %v", err)
	}

	mock := base
	mock.Source = FundingSourceMockRecharge
	if !errors.Is(validateFundingLot(mock), ErrInvalidRecord) {
		t.Fatal("mock funding lot was accepted as verified")
	}
	mock.VerificationStatus = FundingVerificationUnverified
	if err := validateFundingLot(mock); err != nil {
		t.Fatalf("unverified mock funding lot rejected: %v", err)
	}

	historical := base
	historical.Source = FundingSourceHistoricalUnknown
	historical.PaymentPublicID = nil
	if !errors.Is(validateFundingLot(historical), ErrInvalidRecord) {
		t.Fatal("historical funding lot was accepted as verified")
	}
	historical.VerificationStatus = FundingVerificationUnverified
	if err := validateFundingLot(historical); err != nil {
		t.Fatalf("unverified historical funding lot rejected: %v", err)
	}
}

func TestFundingAllocationValidationRequiresBoundedRefund(t *testing.T) {
	now := time.Now().UTC()
	allocationID, _ := NewULID()
	settlementID, _ := NewULID()
	lotID, _ := NewULID()
	value := FundingAllocation{
		FundingAllocationPublicID: allocationID,
		SettlementPublicID:        settlementID,
		FundingLotPublicID:        lotID,
		MerchantUserID:            1,
		AmountCents:               150,
		RowVersion:                1,
		CreatedAt:                 now,
	}
	if err := validateFundingAllocation(value); err != nil {
		t.Fatalf("valid allocation rejected: %v", err)
	}
	value.RefundedAmountCents = 151
	if !errors.Is(validateFundingAllocation(value), ErrInvalidRecord) {
		t.Fatal("allocation accepted refund beyond original amount")
	}
	value.RefundedAmountCents = 150
	value.RefundedAt = &now
	if err := validateFundingAllocation(value); err != nil {
		t.Fatalf("fully refunded allocation rejected: %v", err)
	}
}
