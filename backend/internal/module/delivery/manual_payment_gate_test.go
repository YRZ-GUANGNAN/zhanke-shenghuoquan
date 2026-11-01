package delivery

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func manualPaymentDryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestClaimableQueryExcludesUnpaidAndAssignedOrders(t *testing.T) {
	db := manualPaymentDryDB(t)
	var statements []string
	var arguments [][]interface{}
	err := db.Callback().Query().After("gorm:query").Register("test:capture_claimable", func(tx *gorm.DB) {
		statements = append(statements, strings.ToLower(tx.Statement.SQL.String()))
		arguments = append(arguments, append([]interface{}{}, tx.Statement.Vars...))
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewRepository(db).ListClaimable(context.Background(), Page{Page: 1, Size: 20}); err != nil {
		t.Fatal(err)
	}
	if len(statements) != 2 {
		t.Fatalf("count and list must both be scoped: %v", statements)
	}
	for i, sql := range statements {
		if !strings.Contains(sql, "status = ? and rider_user_id is null") || len(arguments[i]) == 0 || arguments[i][0] != StatusReadyForClaim {
			t.Fatalf("unsafe pool query: %s args=%#v", sql, arguments[i])
		}
	}
}

func TestUnpaidOrdersCannotBeClaimed(t *testing.T) {
	rider := uint64(77)
	for _, status := range []Status{StatusPendingFeePayment, StatusPaymentFailed, StatusCancelled} {
		t.Run(string(status), func(t *testing.T) {
			if CanTransition(status, StatusClaimed) {
				t.Fatal("unpaid order allowed into claimed")
			}
			_, err := buildTransitionUpdates(Order{Status: status}, StatusReadyForClaim, StatusClaimed, &rider, time.Now())
			if !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("stale or forged claim returned %v", err)
			}
		})
	}
}

func TestUnresolvedWechatPaymentQueryIsOrderAndMerchantScoped(t *testing.T) {
	db := manualPaymentDryDB(t)
	var count int64
	query := unresolvedOrderWechatPayments(db, "ORDER-A", 77).Count(&count)
	if query.Error != nil {
		t.Fatal(query.Error)
	}
	sql := strings.ToLower(query.Statement.SQL.String())
	if !strings.Contains(sql, "delivery_order_public_id = ? and merchant_user_id = ? and provider = ? and status in") {
		t.Fatalf("unsafe unresolved payment query: %s", sql)
	}
	want := []interface{}{"ORDER-A", uint64(77), PaymentWechat, PaymentCreated, PaymentPrepayIssued}
	if !reflect.DeepEqual(query.Statement.Vars, want) {
		t.Fatalf("args=%#v want=%#v", query.Statement.Vars, want)
	}
}
