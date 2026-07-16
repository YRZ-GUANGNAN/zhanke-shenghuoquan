package miniappflow

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"log"
	"strconv"
	"strings"
)

type wechatCallbackStructure string

const (
	wechatStructureJSONMediaSuccess wechatCallbackStructure = "json_media_success"
	wechatStructureJSONMediaError   wechatCallbackStructure = "json_media_error"
	wechatStructureJSONOtherPush    wechatCallbackStructure = "json_other_push"
	wechatStructureJSONEncrypted    wechatCallbackStructure = "json_encrypted"
	wechatStructureJSONUnknown      wechatCallbackStructure = "json_unknown"
	wechatStructureXMLMediaSuccess  wechatCallbackStructure = "xml_media_success"
	wechatStructureXMLMediaError    wechatCallbackStructure = "xml_media_error"
	wechatStructureXMLOtherPush     wechatCallbackStructure = "xml_other_push"
	wechatStructureXMLEncrypted     wechatCallbackStructure = "xml_encrypted"
	wechatStructureXMLUnknown       wechatCallbackStructure = "xml_unknown"
	wechatStructureUnrecognized     wechatCallbackStructure = "unrecognized"
)

type wechatCallbackCategory string

const (
	wechatCategoryPlatformDownloadError wechatCallbackCategory = "platform_download_error"
	wechatCategoryPlatformOtherError    wechatCallbackCategory = "platform_other_error"
	wechatCategoryUnrelatedPush         wechatCallbackCategory = "unrelated_push"
	wechatCategoryEncryptedEnvelope     wechatCallbackCategory = "encrypted_envelope"
	wechatCategoryMissingTrace          wechatCallbackCategory = "missing_trace"
	wechatCategoryMissingResult         wechatCallbackCategory = "missing_result"
	wechatCategoryMissingSuggest        wechatCallbackCategory = "missing_suggest"
	wechatCategoryMissingLabel          wechatCallbackCategory = "missing_label"
	wechatCategoryFieldTypeInvalid      wechatCallbackCategory = "field_type_invalid"
	wechatCategorySyntaxInvalid         wechatCallbackCategory = "syntax_invalid"
	wechatCategoryStrictParseRejected   wechatCallbackCategory = "strict_parse_rejected"
	wechatCategoryBodyReadError         wechatCallbackCategory = "body_read_error"
)

func logWechatCallbackRejection(body []byte) {
	structure, category := diagnoseWechatCallback(body)
	logWechatCallbackCategory(structure, category)
}

func logWechatCallbackCategory(structure wechatCallbackStructure, category wechatCallbackCategory) {
	log.Printf("wechat_callback structure=%s category=%s", structure, category)
}

func diagnoseWechatCallback(body []byte) (wechatCallbackStructure, wechatCallbackCategory) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return wechatStructureUnrecognized, wechatCategorySyntaxInvalid
	}
	if json.Valid(trimmed) {
		return diagnoseWechatCallbackJSON(trimmed)
	}
	if bytes.HasPrefix(trimmed, []byte("\xef\xbb\xbf")) {
		return wechatStructureJSONUnknown, wechatCategorySyntaxInvalid
	}
	return diagnoseWechatCallbackXML(trimmed)
}

func diagnoseWechatCallbackJSON(body []byte) (wechatCallbackStructure, wechatCallbackCategory) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil || envelope == nil {
		return wechatStructureJSONUnknown, wechatCategorySyntaxInvalid
	}
	if rawJSONPresent(envelope["Encrypt"]) {
		return wechatStructureJSONEncrypted, wechatCategoryEncryptedEnvelope
	}
	event, eventOK := decodeDiagnosticJSONString(envelope["Event"])
	trace, traceOK := decodeDiagnosticJSONString(envelope["trace_id"])
	errCode, errCodePresent, errCodeOK := decodeDiagnosticJSONInteger(envelope["errcode"])
	if eventOK && event != "wxa_media_check" {
		return wechatStructureJSONOtherPush, wechatCategoryUnrelatedPush
	}
	looksLikeMedia := event == "wxa_media_check" || (traceOK && strings.TrimSpace(trace) != "")
	if !looksLikeMedia {
		return wechatStructureJSONOtherPush, wechatCategoryUnrelatedPush
	}
	if errCodePresent && !errCodeOK {
		return wechatStructureJSONMediaError, wechatCategoryFieldTypeInvalid
	}
	if errCodeOK && errCode != 0 {
		if errCode == -1008 {
			return wechatStructureJSONMediaError, wechatCategoryPlatformDownloadError
		}
		return wechatStructureJSONMediaError, wechatCategoryPlatformOtherError
	}
	if !traceOK || strings.TrimSpace(trace) == "" {
		return wechatStructureJSONMediaSuccess, wechatCategoryMissingTrace
	}
	return diagnoseWechatCallbackJSONResult(envelope["result"])
}

func diagnoseWechatCallbackJSONResult(raw json.RawMessage) (wechatCallbackStructure, wechatCallbackCategory) {
	if !rawJSONPresent(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return wechatStructureJSONMediaSuccess, wechatCategoryMissingResult
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil || result == nil {
		return wechatStructureJSONMediaSuccess, wechatCategoryFieldTypeInvalid
	}
	suggest, suggestOK := decodeDiagnosticJSONString(result["suggest"])
	if !suggestOK || strings.TrimSpace(suggest) == "" {
		if !rawJSONPresent(result["suggest"]) {
			return wechatStructureJSONMediaSuccess, wechatCategoryMissingSuggest
		}
		return wechatStructureJSONMediaSuccess, wechatCategoryFieldTypeInvalid
	}
	if !rawJSONPresent(result["label"]) || bytes.Equal(bytes.TrimSpace(result["label"]), []byte("null")) {
		return wechatStructureJSONMediaSuccess, wechatCategoryMissingLabel
	}
	return wechatStructureJSONMediaSuccess, wechatCategoryStrictParseRejected
}

func rawJSONPresent(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0
}

func decodeDiagnosticJSONString(raw json.RawMessage) (string, bool) {
	if !rawJSONPresent(raw) {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func decodeDiagnosticJSONInteger(raw json.RawMessage) (int64, bool, bool) {
	if !rawJSONPresent(raw) {
		return 0, false, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, true, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, true, false
	}
	parsed, err := strconv.ParseInt(string(number), 10, 64)
	return parsed, true, err == nil
}

func diagnoseWechatCallbackXML(body []byte) (wechatCallbackStructure, wechatCallbackCategory) {
	var envelope struct {
		Event   string `xml:"Event"`
		TraceID string `xml:"trace_id"`
		ErrCode string `xml:"errcode"`
		Encrypt string `xml:"Encrypt"`
		Result  *struct {
			Suggest *string `xml:"suggest"`
			Label   *string `xml:"label"`
		} `xml:"result"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return wechatStructureXMLUnknown, wechatCategorySyntaxInvalid
	}
	if strings.TrimSpace(envelope.Encrypt) != "" {
		return wechatStructureXMLEncrypted, wechatCategoryEncryptedEnvelope
	}
	event := strings.TrimSpace(envelope.Event)
	trace := strings.TrimSpace(envelope.TraceID)
	if event != "" && event != "wxa_media_check" {
		return wechatStructureXMLOtherPush, wechatCategoryUnrelatedPush
	}
	if event == "" && trace == "" {
		return wechatStructureXMLOtherPush, wechatCategoryUnrelatedPush
	}
	if raw := strings.TrimSpace(envelope.ErrCode); raw != "" {
		errCode, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return wechatStructureXMLMediaError, wechatCategoryFieldTypeInvalid
		}
		if errCode == -1008 {
			return wechatStructureXMLMediaError, wechatCategoryPlatformDownloadError
		}
		if errCode != 0 {
			return wechatStructureXMLMediaError, wechatCategoryPlatformOtherError
		}
	}
	if trace == "" {
		return wechatStructureXMLMediaSuccess, wechatCategoryMissingTrace
	}
	if envelope.Result == nil {
		return wechatStructureXMLMediaSuccess, wechatCategoryMissingResult
	}
	if envelope.Result.Suggest == nil || strings.TrimSpace(*envelope.Result.Suggest) == "" {
		return wechatStructureXMLMediaSuccess, wechatCategoryMissingSuggest
	}
	if envelope.Result.Label == nil || strings.TrimSpace(*envelope.Result.Label) == "" {
		return wechatStructureXMLMediaSuccess, wechatCategoryMissingLabel
	}
	return wechatStructureXMLMediaSuccess, wechatCategoryStrictParseRejected
}
