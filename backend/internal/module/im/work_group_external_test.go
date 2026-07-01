package im

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTencentWorkGroupAgainstRealIM(t *testing.T) {
	if os.Getenv("DELIVERY_D5_RUN_REAL_TENCENT_IM") != "1" {
		t.Skip("set DELIVERY_D5_RUN_REAL_TENCENT_IM=1 for real Tencent IM acceptance")
	}
	sdkAppID := strings.TrimSpace(os.Getenv("TENCENT_IM_SDK_APP_ID"))
	secret := strings.TrimSpace(os.Getenv("TENCENT_IM_SECRET_KEY"))
	admin := strings.TrimSpace(os.Getenv("TENCENT_IM_ADMIN_USER"))
	endpoint := strings.TrimSpace(os.Getenv("TENCENT_IM_REST_ENDPOINT"))
	client, err := NewTencentRESTClient(sdkAppID, secret, admin, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	groupID := "d5_accept_work_group"
	merchant := "d5_accept_merchant"
	rider := "d5_accept_rider"
	if err := client.ImportAccount(t.Context(), merchant); err != nil {
		t.Fatal(err)
	}
	if err := client.ImportAccount(t.Context(), rider); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.DismissWorkGroup(ctx, groupID); err != nil {
			t.Errorf("cleanup real Tencent group: %v", err)
		}
	})
	if err := client.CreateWorkGroup(t.Context(), groupID, "D-5 acceptance", "", []string{merchant, rider}); err != nil {
		t.Fatal(err)
	}
	assertRealWorkGroupRole(t, client, groupID, merchant, "Member")
	assertRealWorkGroupRole(t, client, groupID, rider, "Member")

	// Deterministic group IDs make provisioning idempotent after crashes.
	if err := client.CreateWorkGroup(t.Context(), groupID, "D-5 acceptance", "", []string{merchant, rider}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetWorkGroupReadOnly(t.Context(), groupID, true); err != nil {
		t.Fatal(err)
	}
	assertRealWorkGroupMuted(t, client, groupID, true)
	if err := client.RemoveWorkGroupMembers(t.Context(), groupID, []string{rider}); err != nil {
		t.Fatal(err)
	}
	assertRealWorkGroupRole(t, client, groupID, rider, "NotMember")
	if err := client.AddWorkGroupMembers(t.Context(), groupID, []string{rider}); err != nil {
		t.Fatal(err)
	}
	assertRealWorkGroupRole(t, client, groupID, rider, "Member")
	if err := client.DismissWorkGroup(t.Context(), groupID); err != nil {
		t.Fatal(err)
	}
	t.Log("real_tencent_im=PASS account_import=PASS ownerless_work_group=PASS idempotent_create=PASS remote_read_only=PASS member_remove=PASS member_readd=PASS destroy=PASS")
}

func TestTencentWorkGroupReadReceiptAgainstRealIM(t *testing.T) {
	if os.Getenv("DELIVERY_D5_RUN_REAL_TENCENT_IM") != "1" ||
		os.Getenv("DELIVERY_D5_RUN_REAL_READ_RECEIPT") != "1" {
		t.Skip("set DELIVERY_D5_RUN_REAL_TENCENT_IM=1 and DELIVERY_D5_RUN_REAL_READ_RECEIPT=1")
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
	groupID := "d5_accept_read_receipt"
	merchant := "d5_receipt_merchant"
	rider := "d5_receipt_rider"
	if err := client.ImportAccount(t.Context(), merchant); err != nil {
		t.Fatal(err)
	}
	if err := client.ImportAccount(t.Context(), rider); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.DismissWorkGroup(ctx, groupID)
	})
	if err := client.CreateWorkGroup(t.Context(), groupID, "D-5 receipt", "", []string{merchant, rider}); err != nil {
		t.Fatal(err)
	}
	assertRealWorkGroupReadReceipt(t, client, groupID, merchant, rider)
	t.Log("real_tencent_work_group_read_receipt=PASS")
}

func TestTencentD5WorkGroupResidueAgainstRealIM(t *testing.T) {
	if os.Getenv("DELIVERY_D5_RUN_REAL_TENCENT_IM") != "1" {
		t.Skip("set DELIVERY_D5_RUN_REAL_TENCENT_IM=1 for real Tencent IM residue check")
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
	groups := listD5AcceptanceGroups(t, client)
	for _, groupID := range groups {
		if err := client.DismissWorkGroup(t.Context(), groupID); err != nil {
			t.Fatalf("cleanup D-5 acceptance group %s: %v", groupID, err)
		}
	}
	if remaining := listD5AcceptanceGroups(t, client); len(remaining) != 0 {
		t.Fatalf("D-5 Tencent work-group residue: %v", remaining)
	}
	t.Logf("real_tencent_group_residue=0 cleaned=%d", len(groups))
}

func listD5AcceptanceGroups(t *testing.T, client *TencentRESTClient) []string {
	t.Helper()
	var result struct {
		GroupIDList []struct {
			GroupID string `json:"GroupId"`
		} `json:"GroupIdList"`
		Next uint64 `json:"Next"`
	}
	var groups []string
	next := uint64(0)
	for {
		result.GroupIDList = nil
		result.Next = 0
		if err := client.groupCall(t.Context(), "get_appid_group_list", map[string]any{
			"Limit": 1000, "Next": next,
		}, &result); err != nil {
			t.Fatal(err)
		}
		for _, group := range result.GroupIDList {
			if strings.HasPrefix(group.GroupID, "d5_accept_") ||
				strings.HasPrefix(group.GroupID, "d5_receipt_") {
				groups = append(groups, group.GroupID)
			}
		}
		if result.Next == 0 {
			return groups
		}
		next = result.Next
	}
}

func assertRealWorkGroupReadReceipt(
	t *testing.T,
	client *TencentRESTClient,
	groupID, sender, reader string,
) {
	t.Helper()
	var sent struct {
		MsgSeq uint64 `json:"MsgSeq"`
	}
	if err := client.groupCall(t.Context(), "send_group_msg", map[string]any{
		"GroupId": groupID, "Random": uint32(time.Now().UnixNano()),
		"From_Account": sender, "NeedReadReceipt": 1,
		"MsgBody": []map[string]any{{
			"MsgType":    "TIMTextElem",
			"MsgContent": map[string]string{"Text": "D-5 acceptance"},
		}},
	}, &sent); err != nil {
		t.Fatalf("real Tencent group read-receipt send gate: %v", err)
	}
	if sent.MsgSeq == 0 {
		t.Fatal("real Tencent group message returned zero MsgSeq")
	}
	if err := client.groupCall(t.Context(), "group_msg_receipt", map[string]any{
		"GroupId": groupID, "From_Account": reader,
		"MsgSeqList": []map[string]uint64{{"MsgSeq": sent.MsgSeq}},
	}, nil); err != nil {
		t.Fatalf("real Tencent group read-receipt gate is not enabled: %v", err)
	}
	var receipt struct {
		Items []struct {
			Code      int    `json:"Code"`
			MsgSeq    uint64 `json:"MsgSeq"`
			ReadNum   int    `json:"ReadNum"`
			UnreadNum int    `json:"UnreadNum"`
		} `json:"GroupMsgReceiptList"`
	}
	for attempt := 0; attempt < 10; attempt++ {
		receipt.Items = nil
		if err := client.groupCall(t.Context(), "get_group_msg_receipt", map[string]any{
			"GroupId":    groupID,
			"MsgSeqList": []map[string]uint64{{"MsgSeq": sent.MsgSeq}},
		}, &receipt); err != nil {
			t.Fatalf("real Tencent group receipt query: %v", err)
		}
		if len(receipt.Items) == 1 && receipt.Items[0].Code == 0 &&
			receipt.Items[0].MsgSeq == sent.MsgSeq && receipt.Items[0].ReadNum >= 1 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("real Tencent group receipt=%#v", receipt.Items)
}

func assertRealWorkGroupMuted(
	t *testing.T,
	client *TencentRESTClient,
	groupID string,
	want bool,
) {
	t.Helper()
	var result struct {
		Groups []struct {
			GroupID       string `json:"GroupId"`
			MuteAllMember string `json:"MuteAllMember"`
		} `json:"GroupInfo"`
	}
	if err := client.groupCall(t.Context(), "get_group_info", map[string]any{
		"GroupIdList": []string{groupID},
	}, &result); err != nil {
		t.Fatal(err)
	}
	wantValue := "Off"
	if want {
		wantValue = "On"
	}
	if len(result.Groups) != 1 || result.Groups[0].GroupID != groupID ||
		result.Groups[0].MuteAllMember != wantValue {
		t.Fatalf("group=%s info=%#v want mute=%s", groupID, result.Groups, wantValue)
	}
}

func assertRealWorkGroupRole(
	t *testing.T,
	client *TencentRESTClient,
	groupID, account, want string,
) {
	t.Helper()
	var result struct {
		UserIDList []struct {
			Account string `json:"Member_Account"`
			Role    string `json:"Role"`
		} `json:"UserIdList"`
	}
	if err := client.groupCall(t.Context(), "get_role_in_group", map[string]any{
		"GroupId": groupID, "User_Account": []string{account},
	}, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.UserIDList) != 1 || result.UserIDList[0].Account != account ||
		result.UserIDList[0].Role != want {
		t.Fatalf("account=%s role=%#v want=%s", account, result.UserIDList, want)
	}
}
