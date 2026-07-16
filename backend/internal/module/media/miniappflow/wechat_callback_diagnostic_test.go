package miniappflow

import (
	"fmt"
	"strings"
	"testing"
)

func TestDiagnoseWechatCallbackUsesOnlyClosedCategories(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		body          string
		wantStructure wechatCallbackStructure
		wantCategory  wechatCallbackCategory
	}{
		{
			name:          "platform download error",
			body:          `{"Event":"wxa_media_check","trace_id":"secret-trace","errcode":-1008,"errmsg":"signature=secret-query https://cos.example/secret"}`,
			wantStructure: wechatStructureJSONMediaError, wantCategory: wechatCategoryPlatformDownloadError,
		},
		{
			name:          "platform other error",
			body:          `{"Event":"wxa_media_check","trace_id":"secret-trace","errcode":-9}`,
			wantStructure: wechatStructureJSONMediaError, wantCategory: wechatCategoryPlatformOtherError,
		},
		{
			name:          "unrelated push",
			body:          `{"MsgType":"text","Event":"attacker-controlled-event","Content":"signature=secret-query"}`,
			wantStructure: wechatStructureJSONOtherPush, wantCategory: wechatCategoryUnrelatedPush,
		},
		{
			name:          "encrypted envelope",
			body:          `{"Encrypt":"signature=secret-query"}`,
			wantStructure: wechatStructureJSONEncrypted, wantCategory: wechatCategoryEncryptedEnvelope,
		},
		{
			name:          "xml download error",
			body:          `<xml><Event>wxa_media_check</Event><trace_id>secret-trace</trace_id><errcode>-1008</errcode><errmsg>signature=secret-query</errmsg></xml>`,
			wantStructure: wechatStructureXMLMediaError, wantCategory: wechatCategoryPlatformDownloadError,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			structure, category := diagnoseWechatCallback([]byte(test.body))
			if structure != test.wantStructure || category != test.wantCategory {
				t.Fatalf("diagnostic=(%s,%s) want=(%s,%s)", structure, category, test.wantStructure, test.wantCategory)
			}
			message := fmt.Sprintf("wechat_callback structure=%s category=%s", structure, category)
			for _, forbidden := range []string{"secret-trace", "secret-query", "attacker-controlled-event", "cos.example", "signature="} {
				if strings.Contains(message, forbidden) {
					t.Fatalf("diagnostic leaked %q: %s", forbidden, message)
				}
			}
		})
	}
}

func TestDiagnoseWechatCallbackClassifiesRejectedMediaShape(t *testing.T) {
	t.Parallel()
	structure, category := diagnoseWechatCallback([]byte(`{
		"MsgType":"event","Event":"wxa_media_check","trace_id":"trace-1","errcode":0,
		"result":{"suggest":"pass"}
	}`))
	if structure != wechatStructureJSONMediaSuccess || category != wechatCategoryMissingLabel {
		t.Fatalf("diagnostic=(%s,%s)", structure, category)
	}
}
