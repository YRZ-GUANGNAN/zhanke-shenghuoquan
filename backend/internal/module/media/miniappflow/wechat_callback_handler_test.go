package miniappflow

import (
	"context"
	"crypto/sha1" // #nosec G505 -- the WeChat protocol mandates SHA-1.
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)

func TestDecodeWechatReviewResultAcceptsWireVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{
			name: "json numeric label",
			body: `{"ToUserName":"gh_test","MsgType":"event","Event":"wxa_media_check","trace_id":"trace-json-number","result":{"suggest":"pass","label":100}}`,
		},
		{
			name: "json string label",
			body: `{"ToUserName":"gh_test","MsgType":"event","Event":"wxa_media_check","trace_id":"trace-json-string","result":{"suggest":"pass","label":"100"}}`,
		},
		{
			name: "xml label",
			body: `<xml><ToUserName>gh_test</ToUserName><MsgType>event</MsgType><Event>wxa_media_check</Event><trace_id>trace-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := decodeWechatReviewResult([]byte(test.body))
			if err != nil {
				t.Fatalf("decode callback: %v", err)
			}
			if value.TraceID == "" || value.Result.Suggest != "pass" || value.Result.Label == nil || int(*value.Result.Label) != 100 {
				t.Fatalf("unexpected callback: %#v", value)
			}
		})
	}
}

func TestDecodeWechatReviewResultRejectsAmbiguousInput(t *testing.T) {
	t.Parallel()
	tests := []string{
		``,
		`{}`,
		`{"trace_id":"trace-1","result":{"suggest":"","label":100}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass"}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":"100x"}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":1.5}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":-1}}`,
		`<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>bad</label></result></xml>`,
	}
	for _, body := range tests {
		if value, err := decodeWechatReviewResult([]byte(body)); err == nil {
			t.Fatalf("expected rejection, got %#v for %q", value, body)
		}
	}
}

func TestWechatCallbackRejectsDuplicateCriticalFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{
			name: "json duplicate escaped trace id",
			body: `{"trace_id":"trace-1","trace\u005fid":"trace-2","result":{"suggest":"pass","label":100}}`,
		},
		{
			name: "json duplicate escaped result",
			body: `{"trace_id":"trace-1","result":{"suggest":"pass","label":100},"res\u0075lt":{"suggest":"risky","label":200}}`,
		},
		{
			name: "json duplicate escaped suggest",
			body: `{"trace_id":"trace-1","result":{"suggest":"pass","sug\u0067est":"risky","label":100}}`,
		},
		{
			name: "json duplicate escaped label",
			body: `{"trace_id":"trace-1","result":{"suggest":"pass","label":100,"lab\u0065l":200}}`,
		},
		{
			name: "xml duplicate trace id",
			body: `<xml><trace_id>trace-1</trace_id><trace_id>trace-2</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
		},
		{
			name: "xml duplicate result",
			body: `<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>100</label></result><result><suggest>risky</suggest><label>200</label></result></xml>`,
		},
		{
			name: "xml duplicate suggest",
			body: `<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><suggest>risky</suggest><label>100</label></result></xml>`,
		},
		{
			name: "xml duplicate label",
			body: `<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>100</label><label>200</label></result></xml>`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, "application/octet-stream", "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusBadRequest || applier.calls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
			}
		})
	}
}

func TestWechatCallbackRejectsInvalidLabelForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"json null", `{"trace_id":"trace-1","result":{"suggest":"pass","label":null}}`},
		{"json exponent", `{"trace_id":"trace-1","result":{"suggest":"pass","label":1e2}}`},
		{"json boolean", `{"trace_id":"trace-1","result":{"suggest":"pass","label":true}}`},
		{"json object", `{"trace_id":"trace-1","result":{"suggest":"pass","label":{"value":100}}}`},
		{"json array", `{"trace_id":"trace-1","result":{"suggest":"pass","label":[100]}}`},
		{"json string plus sign", `{"trace_id":"trace-1","result":{"suggest":"pass","label":"+100"}}`},
		{"json numeric overflow", `{"trace_id":"trace-1","result":{"suggest":"pass","label":9223372036854775808}}`},
		{"json string overflow", `{"trace_id":"trace-1","result":{"suggest":"pass","label":"9223372036854775808"}}`},
		{"xml plus sign", `<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>+100</label></result></xml>`},
		{"xml overflow", `<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>9223372036854775808</label></result></xml>`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, "application/json", "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusBadRequest || applier.calls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
			}
		})
	}
}

func TestWechatCallbackDoesNotFallBackFromSemanticJSONErrorToXML(t *testing.T) {
	t.Parallel()
	body := `"<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>"`
	applier := &recordingWechatReviewApplier{}
	request := signedWechatReviewRequest(body, "application/xml", "callback-token-strong", true)
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusBadRequest || applier.calls != 0 {
		t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
	}
}

func TestWechatCallbackRejectsUnsafeXML(t *testing.T) {
	t.Parallel()
	validFields := `<trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>100</label></result>`
	tests := []struct {
		name string
		body string
	}{
		{"dtd directive", `<!DOCTYPE xml [<!ENTITY trace "trace-1">]><xml>` + validFields + `</xml>`},
		{"processing instruction", `<?unsafe callback?><xml>` + validFields + `</xml>`},
		{"nested trace id", `<xml><unknown><trace_id>nested</trace_id></unknown>` + validFields + `</xml>`},
		{"nested result", `<xml><unknown><result><suggest>pass</suggest><label>100</label></result></unknown>` + validFields + `</xml>`},
		{"nested suggest", `<xml><trace_id>trace-1</trace_id><result><unknown><suggest>pass</suggest></unknown><suggest>pass</suggest><label>100</label></result></xml>`},
		{"nested label", `<xml><trace_id>trace-1</trace_id><result><unknown><label>100</label></unknown><suggest>pass</suggest><label>100</label></result></xml>`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, "application/xml", "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusBadRequest || applier.calls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
			}
		})
	}
}

func TestWechatCallbackIgnoresUnknownOrdinaryFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{
			name: "json",
			body: `{"trace_id":"trace-json","extra":{"nested":[1,true,null]},"extra":"duplicate is harmless","result":{"suggest":"pass","label":100,"detail":{"score":1}}}`,
		},
		{
			name: "xml",
			body: `<xml><extra><ordinary>value</ordinary></extra><extra>duplicate is harmless</extra><trace_id>trace-xml</trace_id><result><detail><score>1</score></detail><suggest>pass</suggest><label>100</label></result></xml>`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, "application/octet-stream", "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusOK || applier.calls != 1 {
				t.Fatalf("status=%d calls=%d body=%q", response.Code, applier.calls, response.Body.String())
			}
		})
	}
}

type recordingWechatReviewApplier struct {
	calls          int
	resultCalls    int
	failureCalls   int
	traceID        string
	suggest        string
	label          int
	failureTraceID string
	failureCode    string
	err            error
}

func (value *recordingWechatReviewApplier) ApplyWechatReviewResult(_ context.Context, traceID, suggest string, label int) error {
	value.calls++
	value.resultCalls++
	value.traceID = traceID
	value.suggest = suggest
	value.label = label
	return value.err
}

func (value *recordingWechatReviewApplier) ApplyWechatReviewFailure(
	_ context.Context,
	traceID, failureCode string,
) error {
	value.calls++
	value.failureCalls++
	value.failureTraceID = traceID
	value.failureCode = failureCode
	return value.err
}

func signedWechatReviewRequest(body, contentType, token string, validSignature bool) *http.Request {
	timestamp, nonce := "1710000000", "nonce-1"
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	signature := hex.EncodeToString(sum[:])
	if !validSignature {
		signature = strings.Repeat("0", 40)
	}
	query := url.Values{
		"timestamp": {timestamp},
		"nonce":     {nonce},
		"signature": {signature},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/miniapp/media/wechat/callback?"+query.Encode(), strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}

func runWechatReviewCallback(t *testing.T, applier wechatReviewResultApplier, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	handler := &WechatCallbackHandler{service: applier, callbackToken: "callback-token-strong"}
	handler.Register(router.Group("/api/v1/miniapp/media"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestWechatReviewCallbackAppliesOfficialSuccessJSONAndXML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		traceID string
	}{
		{
			name:    "json",
			body:    `{"MsgType":"event","Event":"wxa_media_check","errcode":0,"trace_id":"trace-official-json","result":{"suggest":"pass","label":100}}`,
			traceID: "trace-official-json",
		},
		{
			name:    "xml",
			body:    `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>0</errcode><trace_id>trace-official-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
			traceID: "trace-official-xml",
		},
		{
			name:    "json string label whitespace",
			body:    `{"MsgType":"event","Event":"wxa_media_check","errcode":0,"trace_id":"trace-official-json-string","result":{"suggest":"pass","label":" 100 "}}`,
			traceID: "trace-official-json-string",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
				test.body, "application/octet-stream", "callback-token-strong", true,
			))
			if response.Code != http.StatusOK || response.Body.String() != "success" {
				t.Fatalf("status=%d response_length=%d", response.Code, response.Body.Len())
			}
			fieldsMatch := applier.traceID == test.traceID && applier.suggest == "pass" && applier.label == 100
			if applier.resultCalls != 1 || applier.failureCalls != 0 || !fieldsMatch {
				t.Fatalf(
					"result_calls=%d failure_calls=%d fields_match=%t",
					applier.resultCalls, applier.failureCalls, fieldsMatch,
				)
			}
		})
	}
}

func TestWechatReviewCallbackMapsDownloadAndPlatformErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		traceID     string
		failureCode string
	}{
		{
			name:        "json download",
			body:        `{"MsgType":"event","Event":"wxa_media_check","errcode":-1008,"trace_id":"trace-download-json"}`,
			traceID:     "trace-download-json",
			failureCode: wechatMediaDownloadFailureCode,
		},
		{
			name:        "xml download",
			body:        `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>-1008</errcode><trace_id>trace-download-xml</trace_id></xml>`,
			traceID:     "trace-download-xml",
			failureCode: wechatMediaDownloadFailureCode,
		},
		{
			name:        "json platform",
			body:        `{"MsgType":"event","Event":"wxa_media_check","errcode":-1001,"trace_id":"trace-platform-json"}`,
			traceID:     "trace-platform-json",
			failureCode: wechatMediaPlatformFailureCode,
		},
		{
			name:        "xml platform",
			body:        `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>-1001</errcode><trace_id>trace-platform-xml</trace_id></xml>`,
			traceID:     "trace-platform-xml",
			failureCode: wechatMediaPlatformFailureCode,
		},
		{
			name:        "legacy json download",
			body:        `{"errcode":-1008,"trace_id":"trace-download-legacy-json"}`,
			traceID:     "trace-download-legacy-json",
			failureCode: wechatMediaDownloadFailureCode,
		},
		{
			name:        "legacy xml platform",
			body:        `<xml><errcode>-1001</errcode><trace_id>trace-platform-legacy-xml</trace_id></xml>`,
			traceID:     "trace-platform-legacy-xml",
			failureCode: wechatMediaPlatformFailureCode,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
				test.body, "application/octet-stream", "callback-token-strong", true,
			))
			if response.Code != http.StatusOK || response.Body.String() != "success" {
				t.Fatalf("status=%d response_length=%d", response.Code, response.Body.Len())
			}
			fieldsMatch := applier.failureTraceID == test.traceID && applier.failureCode == test.failureCode
			if applier.resultCalls != 0 || applier.failureCalls != 1 || !fieldsMatch {
				t.Fatalf(
					"result_calls=%d failure_calls=%d fields_match=%t",
					applier.resultCalls, applier.failureCalls, fieldsMatch,
				)
			}
		})
	}
}

func TestWechatReviewCallbackIgnoresUnrelatedPushes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		body string
	}{
		{"json text", `{"MsgType":"text","Content":"hello"}`},
		{"json other event", `{"MsgType":"event","Event":"subscribe","FromUserName":"opaque"}`},
		{"xml text", `<xml><MsgType>text</MsgType><Content>hello</Content></xml>`},
		{"xml other event", `<xml><MsgType>event</MsgType><Event>subscribe</Event><FromUserName>opaque</FromUserName></xml>`},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
				test.body, "application/octet-stream", "callback-token-strong", true,
			))
			if response.Code != http.StatusOK || response.Body.String() != "success" ||
				applier.resultCalls != 0 || applier.failureCalls != 0 {
				t.Fatalf(
					"status=%d response_length=%d result_calls=%d failure_calls=%d",
					response.Code, response.Body.Len(), applier.resultCalls, applier.failureCalls,
				)
			}
		})
	}
}

func TestWechatReviewCallbackRejectsAmbiguousOfficialEnvelope(t *testing.T) {
	t.Parallel()
	validJSONTail := `,"Event":"wxa_media_check","errcode":0,"trace_id":"trace-ambiguous","result":{"suggest":"pass","label":100}}`
	validXMLTail := `<Event>wxa_media_check</Event><errcode>0</errcode><trace_id>trace-ambiguous</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`
	tests := []struct {
		name string
		body string
	}{
		{"json duplicate msg type", `{"MsgType":"event","Msg\u0054ype":"event"` + validJSONTail},
		{"json duplicate event", `{"MsgType":"event","Event":"wxa_media_check","Ev\u0065nt":"wxa_media_check","errcode":0,"trace_id":"trace-ambiguous","result":{"suggest":"pass","label":100}}`},
		{"json duplicate error code", `{"MsgType":"event","Event":"wxa_media_check","errcode":0,"errc\u006fde":0,"trace_id":"trace-ambiguous","result":{"suggest":"pass","label":100}}`},
		{"json duplicate encrypt", `{"Encrypt":"cipher","Encr\u0079pt":"cipher"}`},
		{"json msg type scalar", `{"MsgType":1` + validJSONTail},
		{"json event scalar", `{"MsgType":"event","Event":true,"errcode":0,"trace_id":"trace-ambiguous","result":{"suggest":"pass","label":100}}`},
		{"json error code string", `{"MsgType":"event","Event":"wxa_media_check","errcode":"-1008","trace_id":"trace-ambiguous"}`},
		{"json error code float", `{"MsgType":"event","Event":"wxa_media_check","errcode":-1008.0,"trace_id":"trace-ambiguous"}`},
		{"json error code exponent", `{"MsgType":"event","Event":"wxa_media_check","errcode":-1e3,"trace_id":"trace-ambiguous"}`},
		{"json encrypt scalar", `{"Encrypt":"opaque-ciphertext"}`},
		{"json encrypt wrong type", `{"Encrypt":{"cipher":"opaque"}}`},
		{"json media event with text type", `{"MsgType":"text","Event":"wxa_media_check","errcode":0,"trace_id":"trace-ambiguous","result":{"suggest":"pass","label":100}}`},
		{"xml duplicate msg type", `<xml><MsgType>event</MsgType><MsgType>event</MsgType>` + validXMLTail},
		{"xml duplicate event", `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><Event>wxa_media_check</Event><errcode>0</errcode><trace_id>trace-ambiguous</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`},
		{"xml duplicate error code", `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>0</errcode><errcode>0</errcode><trace_id>trace-ambiguous</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`},
		{"xml duplicate encrypt", `<xml><Encrypt>cipher</Encrypt><Encrypt>cipher</Encrypt></xml>`},
		{"xml msg type non scalar", `<xml><MsgType><value>event</value></MsgType>` + validXMLTail},
		{"xml event non scalar", `<xml><MsgType>event</MsgType><Event><value>wxa_media_check</value></Event><errcode>0</errcode><trace_id>trace-ambiguous</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`},
		{"xml error code plus", `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>+1008</errcode><trace_id>trace-ambiguous</trace_id></xml>`},
		{"xml error code float", `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>-1008.0</errcode><trace_id>trace-ambiguous</trace_id></xml>`},
		{"xml error code exponent", `<xml><MsgType>event</MsgType><Event>wxa_media_check</Event><errcode>-1e3</errcode><trace_id>trace-ambiguous</trace_id></xml>`},
		{"xml encrypt scalar", `<xml><Encrypt>opaque-ciphertext</Encrypt></xml>`},
		{"xml encrypt non scalar", `<xml><Encrypt><cipher>opaque</cipher></Encrypt></xml>`},
		{"xml media event with text type", `<xml><MsgType>text</MsgType><Event>wxa_media_check</Event><errcode>0</errcode><trace_id>trace-ambiguous</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
				test.body, "application/octet-stream", "callback-token-strong", true,
			))
			if response.Code != http.StatusBadRequest || response.Body.Len() != 0 ||
				applier.resultCalls != 0 || applier.failureCalls != 0 {
				t.Fatalf(
					"status=%d response_length=%d result_calls=%d failure_calls=%d",
					response.Code, response.Body.Len(), applier.resultCalls, applier.failureCalls,
				)
			}
		})
	}
}

func TestWechatReviewCallbackReturns500WhenFailureApplicationFails(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{err: errors.New("projection unavailable")}
	response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
		`{"MsgType":"event","Event":"wxa_media_check","errcode":-1001,"trace_id":"trace-service-failure"}`,
		"application/json", "callback-token-strong", true,
	))
	if response.Code != http.StatusInternalServerError || response.Body.Len() != 0 ||
		applier.resultCalls != 0 || applier.failureCalls != 1 ||
		applier.failureCode != wechatMediaPlatformFailureCode {
		t.Fatalf(
			"status=%d response_length=%d result_calls=%d failure_calls=%d failure_code=%q",
			response.Code, response.Body.Len(), applier.resultCalls, applier.failureCalls, applier.failureCode,
		)
	}
}

func TestWechatReviewCallbackKeepsLegacyMinimalSuccessEnvelope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		body    string
		traceID string
	}{
		{
			name:    "json",
			body:    `{"trace_id":"trace-legacy-json","result":{"suggest":"pass","label":100}}`,
			traceID: "trace-legacy-json",
		},
		{
			name:    "xml",
			body:    `<xml><trace_id>trace-legacy-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
			traceID: "trace-legacy-xml",
		},
		{
			name:    "json with zero error code",
			body:    `{"errcode":0,"trace_id":"trace-legacy-json-zero","result":{"suggest":"pass","label":100}}`,
			traceID: "trace-legacy-json-zero",
		},
		{
			name:    "xml with zero error code",
			body:    `<xml><errcode>0</errcode><trace_id>trace-legacy-xml-zero</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
			traceID: "trace-legacy-xml-zero",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			response := runWechatReviewCallback(t, applier, signedWechatReviewRequest(
				test.body, "application/octet-stream", "callback-token-strong", true,
			))
			if response.Code != http.StatusOK || response.Body.String() != "success" ||
				applier.resultCalls != 1 || applier.failureCalls != 0 || applier.traceID != test.traceID {
				t.Fatalf(
					"status=%d response_length=%d result_calls=%d failure_calls=%d",
					response.Code, response.Body.Len(), applier.resultCalls, applier.failureCalls,
				)
			}
		})
	}
}

func TestWechatCallbackIgnoresMisleadingContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		contentType string
		traceID     string
	}{
		{
			name:        "json marked as xml",
			body:        `{"trace_id":"trace-json","result":{"suggest":"pass","label":"100"}}`,
			contentType: "application/xml",
			traceID:     "trace-json",
		},
		{
			name:        "xml marked as text",
			body:        `<xml><trace_id>trace-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
			contentType: "text/plain",
			traceID:     "trace-xml",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, test.contentType, "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusOK || response.Body.String() != "success" {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if applier.calls != 1 || applier.traceID != test.traceID || applier.suggest != "pass" || applier.label != 100 {
				t.Fatalf("unexpected call: %#v", applier)
			}
		})
	}
}

func TestWechatCallbackRejectsBadSignatureBeforeReadingBody(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{}
	body := &countingReadCloser{reader: strings.NewReader(`{"trace_id":"trace-1","result":{"suggest":"pass","label":100}}`)}
	request := signedWechatReviewRequest("", "application/json", "callback-token-strong", false)
	request.Body = body
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusForbidden || applier.calls != 0 || body.reads != 0 {
		t.Fatalf("status=%d calls=%d body_reads=%d", response.Code, applier.calls, body.reads)
	}
}

type countingReadCloser struct {
	reader io.Reader
	reads  int
}

func (value *countingReadCloser) Read(buffer []byte) (int, error) {
	value.reads++
	return value.reader.Read(buffer)
}

func (*countingReadCloser) Close() error { return nil }

func TestWechatCallbackEnforcesBodySizeBoundary(t *testing.T) {
	t.Parallel()
	prefix := `{"trace_id":"trace-boundary","result":{"suggest":"pass","label":100},"padding":"`
	suffix := `"}`
	for _, delta := range []int{0, 1} {
		delta := delta
		t.Run("delta="+strconv.Itoa(delta), func(t *testing.T) {
			t.Parallel()
			target := int(maxWechatReviewCallbackBodyBytes) + delta
			body := prefix + strings.Repeat("x", target-len(prefix)-len(suffix)) + suffix
			if len(body) != target {
				t.Fatalf("body length=%d target=%d", len(body), target)
			}
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(body, "application/json", "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if delta == 0 && (response.Code != http.StatusOK || applier.calls != 1) {
				t.Fatalf("exact boundary status=%d calls=%d", response.Code, applier.calls)
			}
			if delta == 1 && (response.Code != http.StatusBadRequest || applier.calls != 0) {
				t.Fatalf("oversized status=%d calls=%d", response.Code, applier.calls)
			}
		})
	}
}

func TestWechatCallbackMapsServiceFailureToRetryableError(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{err: errors.New("projection unavailable")}
	request := signedWechatReviewRequest(`{"trace_id":"trace-1","result":{"suggest":"pass","label":100}}`, "application/json", "callback-token-strong", true)
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusInternalServerError || applier.calls != 1 {
		t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
	}
}

func TestWechatCallbackUnknownSuggestTransitionsRealServiceToReviewFailed(t *testing.T) {
	traceID := "wechat-unknown-suggest"
	reviewTaskID, err := WechatReviewTaskID(traceID)
	if err != nil {
		t.Fatal(err)
	}
	projection := &projectionStub{value: coremedia.ReviewProjection{
		ProjectionID: testULID(t), AssetID: 41, Status: coremedia.ProjectionReviewing,
		ReviewTaskID: pointer(reviewTaskID), RowVersion: 3,
	}}
	outbox := &outboxStub{}
	consumer := &consumerStub{result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1}, onConsume: func() {
		projection.value.Status = coremedia.ProjectionReviewFailed
		projection.value.AppliedDecisionVersion = 1
		projection.value.RowVersion = 4
	}}
	service, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
		outbox, consumer, &statusStub{}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := signedWechatReviewRequest(
		`{"trace_id":"`+traceID+`","result":{"suggest":"unexpected-value","label":100}}`,
		"application/json", "callback-token-strong", true,
	)
	response := runWechatReviewCallback(t, service, request)
	if response.Code != http.StatusOK || projection.value.Status != coremedia.ProjectionReviewFailed {
		t.Fatalf("status=%d projection=%s", response.Code, projection.value.Status)
	}
	if len(outbox.values) != 1 || outbox.values[0].Operation != coremedia.ConsumerOperationDecision {
		t.Fatalf("expected one decision event, got %#v", outbox.values)
	}
	var decision coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.values[0].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != coremedia.ProjectionReviewFailed || decision.ResultCode != "wechat_unknown_suggest" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestWechatCallbackKeepsTerminalProjectionIdempotent(t *testing.T) {
	t.Parallel()
	for _, status := range []coremedia.ProjectionStatus{
		coremedia.ProjectionApproved,
		coremedia.ProjectionRejected,
		coremedia.ProjectionReviewFailed,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			traceID := "wechat-terminal-" + string(status)
			reviewTaskID, err := WechatReviewTaskID(traceID)
			if err != nil {
				t.Fatal(err)
			}
			projection := &projectionStub{value: coremedia.ReviewProjection{
				ProjectionID:           testULID(t),
				AssetID:                41,
				Status:                 status,
				ReviewTaskID:           &reviewTaskID,
				AppliedDecisionVersion: 1,
				RowVersion:             4,
			}}
			outbox := &outboxStub{}
			service, err := NewService(
				&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
				outbox, &consumerStub{}, &statusStub{}, nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			request := signedWechatReviewRequest(
				`{"trace_id":"`+traceID+`","result":{"suggest":"pass","label":100}}`,
				"application/json", "callback-token-strong", true,
			)
			response := runWechatReviewCallback(t, service, request)
			if response.Code != http.StatusOK || response.Body.String() != "success" {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if len(outbox.values) != 0 {
				t.Fatalf("terminal callback appended events: %#v", outbox.values)
			}
		})
	}
}
