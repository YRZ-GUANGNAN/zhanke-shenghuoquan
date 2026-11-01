package delivery

import (
	"errors"
	"strings"
	"testing"
)

func TestWithdrawalValidation(t *testing.T) {
	key := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, value := range []struct {
		rider, amount uint64
		key           string
	}{
		{0, 1, key}, {1, 0, key}, {1, 1, "bad-key"},
	} {
		if err := validateRiderWithdrawalInput(value.rider, value.amount, value.key); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("invalid input accepted: %+v", value)
		}
	}
	if err := validateRiderWithdrawalInput(1, 1, key); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(validateWithdrawalReason(" \t"), ErrWithdrawalReasonRequired) {
		t.Fatal("blank rejection reason accepted")
	}
	if !errors.Is(validateWithdrawalReason(strings.Repeat("字", 257)), ErrInvalidRecord) {
		t.Fatal("overlong rejection reason accepted")
	}
	if err := validateWithdrawalReason("未实际付款，退回余额"); err != nil {
		t.Fatal(err)
	}
}

func TestWithdrawalReplayRequiresSameOwnerAndAmount(t *testing.T) {
	value := RiderWithdrawal{RiderUserID: 7, AmountCents: 123}
	if err := withdrawalReplay(value, 7, 123); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][2]uint64{{8, 123}, {7, 124}} {
		if !errors.Is(withdrawalReplay(value, args[0], args[1]), ErrIdempotencyConflict) {
			t.Fatal("changed request accepted")
		}
	}
}

func TestWithdrawalDecisionReplayAndCAS(t *testing.T) {
	key := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	operator, version := uint64(7), uint64(2)
	value := RiderWithdrawal{Status: RiderWithdrawalProcessing, RowVersion: version}
	if replay, err := withdrawalDecision(value, RiderWithdrawalPaid, version, operator, key, "", "BANK-123"); replay || err != nil {
		t.Fatalf("first decision: %v %v", replay, err)
	}
	if _, err := withdrawalDecision(value, RiderWithdrawalPaid, 1, operator, key, "", "BANK-123"); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatal(err)
	}
	value.Status, value.RowVersion = RiderWithdrawalPaid, 3
	value.ProcessKey, value.ProcessVersion, value.ProcessedBy, value.PaymentReference = &key, &version, &operator, "BANK-123"
	if replay, err := withdrawalDecision(value, RiderWithdrawalPaid, version, operator, key, "", "BANK-123"); !replay || err != nil {
		t.Fatalf("exact replay: %v %v", replay, err)
	}
	for _, test := range []struct {
		status            RiderWithdrawalStatus
		version, operator uint64
		reason, reference string
	}{
		{RiderWithdrawalRejected, version, operator, "refund", ""},
		{RiderWithdrawalPaid, 3, operator, "", "BANK-123"},
		{RiderWithdrawalPaid, version, 8, "", "BANK-123"},
		{RiderWithdrawalPaid, version, operator, "", "BANK-456"},
	} {
		if _, err := withdrawalDecision(value, test.status, test.version, test.operator, key, test.reason, test.reference); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("changed operation accepted: %+v %v", test, err)
		}
	}
	if _, err := withdrawalDecision(value, RiderWithdrawalRejected, version, operator, "01ARZ3NDEKTSV4RRFFQ69G5FAW", "refund", ""); !errors.Is(err, ErrWithdrawalAlreadyProcessed) {
		t.Fatal(err)
	}
}
