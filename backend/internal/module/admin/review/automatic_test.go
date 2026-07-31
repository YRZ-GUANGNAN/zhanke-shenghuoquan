package review

import "testing"

func TestClassifyAutomaticDecision(t *testing.T) {
	tests := []struct {
		name       string
		objectType string
		risk       string
		apply      bool
		status     Status
	}{
		{name: "post pass", objectType: "post", risk: "wechat pass", apply: true, status: StatusApproved},
		{name: "comment risky", objectType: "comment", risk: "wechat risky label=100", apply: true, status: StatusRejected},
		{name: "product review", objectType: "product", risk: "wechat review label=20001"},
		{name: "product comment fallback", objectType: "product_comment", risk: "manual:wechat_request_failed"},
		{name: "identity excluded", objectType: "identity_application", risk: "wechat pass"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifyAutomaticDecision(test.objectType, test.risk)
			if got.Apply != test.apply || got.Status != test.status {
				t.Fatalf("decision=%#v want apply=%v status=%q", got, test.apply, test.status)
			}
			if got.Apply && got.Reason == "" {
				t.Fatal("automatic decision must record a reason")
			}
		})
	}
}
