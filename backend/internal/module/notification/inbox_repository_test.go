package notification

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAuthorizationRejectionRevokesOnlyAvailableGrantsAndCannotRestoreOnReplay(t *testing.T) {
	for _, status := range []string{"rejected", "banned"} {
		t.Run(status, func(t *testing.T) {
			repo, state, _ := newSubscribeSQLFixture(t)
			available := subscriptionSQLRow(20, state.now)
			reserved := subscriptionSQLRow(21, state.now)
			reserved["remaining_quota"], reserved["reserved_quota"], reserved["reservation_nonce"], reserved["reserved_at"] = int64(0), int64(1), "in-flight", state.now
			consumed := subscriptionSQLRow(22, state.now)
			consumed["remaining_quota"], consumed["status"] = int64(0), "exhausted"
			otherUser := subscriptionSQLRow(23, state.now)
			otherUser["user_id"] = int64(10)
			otherIdentity := subscriptionSQLRow(24, state.now)
			otherIdentity["openid"] = "other-private-id"
			challenge := subscriptionSQLRow(30, state.now)
			challenge["authorization_nonce"], challenge["status"], challenge["remaining_quota"], challenge["invalidation_reason"], challenge["expires_at"] = strings.Repeat("b", 64), "invalid", int64(0), authorizationPending, state.now.Add(time.Minute)
			state.tables["notification_subscriptions"] = []map[string]driver.Value{available, reserved, consumed, otherUser, otherIdentity, challenge}
			report := AuthorizationReport{UserID: 9, AppID: "wx-test", OpenID: "private-openid", TemplateKey: "order", Nonce: strings.Repeat("b", 64), Status: status, Now: state.now}
			duplicate, err := repo.ReportSubscriptionAuthorization(context.Background(), report)
			if err != nil || duplicate {
				t.Fatalf("duplicate=%v err=%v", duplicate, err)
			}
			rows := state.tables["notification_subscriptions"]
			if rows[0]["status"] != "revoked" || rows[0]["remaining_quota"] != int64(0) || rows[0]["invalidation_reason"] != authorizationReported+"accepted" {
				t.Fatalf("available consent not safely revoked %+v", rows[0])
			}
			if rows[1]["status"] != "accepted" || rows[1]["reserved_quota"] != int64(1) || rows[1]["reservation_nonce"] != "in-flight" || rows[2]["status"] != "exhausted" {
				t.Fatal("rejection changed occupied or consumed quota")
			}
			if rows[3]["remaining_quota"] != int64(1) || rows[4]["remaining_quota"] != int64(1) {
				t.Fatal("rejection crossed identity boundary")
			}
			replay := AuthorizationReport{UserID: 9, AppID: "wx-test", OpenID: "private-openid", TemplateKey: "order", Nonce: available["authorization_nonce"].(string), Status: "accepted", Now: state.now}
			duplicate, err = repo.ReportSubscriptionAuthorization(context.Background(), replay)
			if err != nil || !duplicate || state.tables["notification_subscriptions"][0]["remaining_quota"] != int64(0) || state.tables["notification_subscriptions"][0]["status"] != "revoked" {
				t.Fatalf("old accepted replay restored quota: duplicate=%v err=%v", duplicate, err)
			}
			report.Status = "accepted"
			if _, err := repo.ReportSubscriptionAuthorization(context.Background(), report); !errors.Is(err, ErrAuthorizationConflict) {
				t.Fatalf("changed report accepted: %v", err)
			}
		})
	}
}

func TestAuthorizationRejectionRollsBackReportIfRevocationFails(t *testing.T) {
	repo, state, _ := newSubscribeSQLFixture(t)
	challenge := subscriptionSQLRow(30, state.now)
	challenge["authorization_nonce"], challenge["status"], challenge["remaining_quota"], challenge["invalidation_reason"], challenge["expires_at"] = strings.Repeat("b", 64), "invalid", int64(0), authorizationPending, state.now.Add(time.Minute)
	state.tables["notification_subscriptions"] = append(state.tables["notification_subscriptions"], challenge)
	state.failSubscriptionUpdate = true
	if _, err := repo.ReportSubscriptionAuthorization(context.Background(), AuthorizationReport{UserID: 9, AppID: "wx-test", OpenID: "private-openid", TemplateKey: "order", Nonce: strings.Repeat("b", 64), Status: "rejected", Now: state.now}); err == nil {
		t.Fatal("expected transaction failure")
	}
	if state.tables["notification_subscriptions"][1]["invalidation_reason"] != authorizationPending || state.tables["notification_subscriptions"][0]["remaining_quota"] != int64(1) {
		t.Fatal("partial report/revocation committed")
	}
}
