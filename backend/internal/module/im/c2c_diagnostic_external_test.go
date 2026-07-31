package im

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestTencentC2CDiagnosticAgainstRealIM is a read-only production diagnostic.
// It verifies that the two configured business accounts exist in the same
// Tencent IM application and that their C2C roaming history is queryable.
func TestTencentC2CDiagnosticAgainstRealIM(t *testing.T) {
	if os.Getenv("RUN_REAL_TENCENT_C2C_DIAGNOSTIC") != "1" {
		t.Skip("set RUN_REAL_TENCENT_C2C_DIAGNOSTIC=1 for the read-only Tencent IM diagnostic")
	}
	client, err := NewTencentRESTClient(
		strings.TrimSpace(os.Getenv("TENCENT_IM_SDK_APP_ID")),
		strings.TrimSpace(os.Getenv("TENCENT_IM_SECRET_KEY")),
		strings.TrimSpace(os.Getenv("TENCENT_IM_ADMIN_USER")),
		strings.TrimSpace(os.Getenv("TENCENT_IM_REST_ENDPOINT")),
	)
	if err != nil {
		t.Fatal(err)
	}

	operator := strings.TrimSpace(os.Getenv("TEST_TENCENT_IM_OPERATOR"))
	peer := strings.TrimSpace(os.Getenv("TEST_TENCENT_IM_PEER"))
	if operator == "" || peer == "" || operator == peer {
		t.Fatal("TEST_TENCENT_IM_OPERATOR and TEST_TENCENT_IM_PEER must be different non-empty accounts")
	}

	var accounts struct {
		Items []struct {
			UserID        string `json:"UserID"`
			ResultCode    int    `json:"ResultCode"`
			AccountStatus string `json:"AccountStatus"`
		} `json:"ResultItem"`
	}
	if err := client.restCall(t.Context(), "im_open_login_svc", "account_check", map[string]any{
		"CheckItem": []map[string]string{{"UserID": operator}, {"UserID": peer}},
	}, &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts.Items) != 2 {
		t.Fatalf("account_check returned %d accounts, want 2", len(accounts.Items))
	}
	for _, item := range accounts.Items {
		if item.ResultCode != 0 || item.AccountStatus != "Imported" {
			t.Fatalf("account %s is unavailable: code=%d status=%s", item.UserID, item.ResultCode, item.AccountStatus)
		}
	}

	now := time.Now().Unix()
	var history struct {
		Complete int `json:"Complete"`
		Count    int `json:"MsgCnt"`
		Messages []struct {
			From string `json:"From_Account"`
			To   string `json:"To_Account"`
		} `json:"MsgList"`
	}
	if err := client.restCall(t.Context(), "openim", "admin_getroammsg", map[string]any{
		"Operator_Account": operator,
		"Peer_Account":     peer,
		"MaxCnt":           100,
		"MinTime":          now - int64(7*24*time.Hour/time.Second),
		"MaxTime":          now,
	}, &history); err != nil {
		t.Fatal(err)
	}

	var operatorToPeer, peerToOperator int
	for _, message := range history.Messages {
		switch {
		case message.From == operator && message.To == peer:
			operatorToPeer++
		case message.From == peer && message.To == operator:
			peerToOperator++
		default:
			t.Fatalf("history contains an unexpected participant pair: from=%s to=%s", message.From, message.To)
		}
	}
	t.Logf(
		"real_tencent_c2c=PASS accounts=2 history=%d operator_to_peer=%d peer_to_operator=%d complete=%d",
		history.Count, operatorToPeer, peerToOperator, history.Complete,
	)
	if os.Getenv("TEST_TENCENT_IM_EXPECT_C2C_HISTORY") == "1" && history.Count == 0 {
		t.Fatal("Tencent IM returned no C2C roaming history for the configured accounts")
	}
}
