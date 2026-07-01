package im

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestTencentRESTClientWorkGroupLifecycleAndIdempotency(t *testing.T) {
	var mu sync.Mutex
	createCalls := 0
	addCalls := 0
	removeCalls := 0
	readOnlyCalls := 0
	destroyCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sdkappid") != "1400000000" ||
			r.URL.Query().Get("identifier") != "administrator" ||
			r.URL.Query().Get("usersig") == "" {
			t.Errorf("missing Tencent REST authentication: %s", r.URL.RawQuery)
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/v4/group_open_http_svc/create_group":
			createCalls++
			var body struct {
				Owner  string `json:"Owner_Account"`
				Type   string `json:"Type"`
				Group  string `json:"GroupId"`
				Member []struct {
					Account string `json:"Member_Account"`
				} `json:"MemberList"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Owner != "" || body.Type != "Private" || body.Group != "d5_group" ||
				len(body.Member) != 2 || body.Member[0].Account != "campus_1" ||
				body.Member[1].Account != "campus_2" {
				t.Errorf("unexpected create body: %#v", body)
			}
			if createCalls == 1 {
				writeTencentResult(w, 0, "OK", nil)
			} else {
				writeTencentResult(w, 10025, "FAIL", nil)
			}
		case "/v4/group_open_http_svc/add_group_member":
			addCalls++
			writeTencentResult(w, 0, "OK", map[string]any{
				"MemberList": []map[string]any{
					{"Member_Account": "campus_1", "Result": 2},
					{"Member_Account": "campus_2", "Result": 2},
				},
			})
		case "/v4/group_open_http_svc/delete_group_member":
			removeCalls++
			writeTencentResult(w, 0, "OK", nil)
		case "/v4/group_open_http_svc/modify_group_base_info":
			readOnlyCalls++
			var body struct {
				Group string `json:"GroupId"`
				Mute  string `json:"MuteAllMember"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Group != "d5_group" || body.Mute != "On" {
				t.Errorf("unexpected read-only body: %#v", body)
			}
			writeTencentResult(w, 0, "OK", nil)
		case "/v4/group_open_http_svc/destroy_group":
			destroyCalls++
			writeTencentResult(w, 10010, "FAIL", nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewTencentRESTClient("1400000000", "secret", "administrator", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		if err := client.CreateWorkGroup(
			t.Context(), "d5_group", "D-order", "", []string{"campus_1", "campus_2"},
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.RemoveWorkGroupMembers(t.Context(), "d5_group", []string{"campus_2"}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetWorkGroupReadOnly(t.Context(), "d5_group", true); err != nil {
		t.Fatal(err)
	}
	if err := client.DismissWorkGroup(t.Context(), "d5_group"); err != nil {
		t.Fatal(err)
	}
	if createCalls != 2 || addCalls != 1 || removeCalls != 1 || readOnlyCalls != 1 || destroyCalls != 1 {
		t.Fatalf(
			"calls create=%d add=%d remove=%d readOnly=%d destroy=%d",
			createCalls, addCalls, removeCalls, readOnlyCalls, destroyCalls,
		)
	}
}

func writeTencentResult(w http.ResponseWriter, code int, status string, extra map[string]any) {
	value := map[string]any{"ActionStatus": status, "ErrorCode": code, "ErrorInfo": status}
	for key, item := range extra {
		value[key] = item
	}
	_ = json.NewEncoder(w).Encode(value)
}
