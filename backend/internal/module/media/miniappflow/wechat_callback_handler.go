package miniappflow

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxWechatReviewCallbackBodyBytes int64 = 256 << 10

type wechatReviewResultApplier interface {
	ApplyWechatReviewResult(context.Context, string, string, int) error
	ApplyWechatReviewFailure(context.Context, string, string) error
}

type WechatCallbackHandler struct {
	service       wechatReviewResultApplier
	callbackToken string
}

func NewWechatCallbackHandler(service *Service, callbackToken string) *WechatCallbackHandler {
	return &WechatCallbackHandler{service: service, callbackToken: callbackToken}
}

func (h *WechatCallbackHandler) Register(group *gin.RouterGroup) {
	group.GET("/wechat/callback", h.verify)
	group.POST("/wechat/callback", h.callback)
}

func (h *WechatCallbackHandler) verify(c *gin.Context) {
	if !h.valid(c) {
		c.Status(http.StatusForbidden)
		return
	}
	c.String(http.StatusOK, c.Query("echostr"))
}

type wechatCallbackDisposition uint8

const (
	wechatCallbackApplyResult wechatCallbackDisposition = iota
	wechatCallbackApplyFailure
	wechatCallbackIgnore
)

type wechatReviewResult struct {
	MsgType   *string
	Event     *string
	ErrCode   *int64
	Encrypted bool
	TraceID   string
	Result    struct {
		Suggest string             `json:"suggest" xml:"suggest"`
		Label   *wechatReviewLabel `json:"label" xml:"label"`
	}
}

type decodedWechatCallback struct {
	Disposition wechatCallbackDisposition
	Value       wechatReviewResult
	FailureCode string
}

var errInvalidWechatReviewCallback = errors.New("invalid wechat review callback")

type wechatReviewLabel int

func parseWechatReviewLabel(raw string) (wechatReviewLabel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errInvalidWechatReviewCallback
	}
	for index := range raw {
		if raw[index] < '0' || raw[index] > '9' {
			return 0, errInvalidWechatReviewCallback
		}
	}
	value, err := strconv.ParseInt(raw, 10, strconv.IntSize)
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	return wechatReviewLabel(value), nil
}

func decodeWechatReviewResult(body []byte) (wechatReviewResult, error) {
	decoded, err := decodeWechatReviewCallback(body)
	if err != nil || decoded.Disposition != wechatCallbackApplyResult {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	return decoded.Value, nil
}

func decodeWechatReviewCallback(body []byte) (decodedWechatCallback, error) {
	value, err := decodeWechatReviewEnvelope(body)
	if err != nil || value.Encrypted {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	msgType, event := "", ""
	if value.MsgType != nil {
		msgType = strings.TrimSpace(*value.MsgType)
	}
	if value.Event != nil {
		event = strings.TrimSpace(*value.Event)
	}

	if value.Event != nil && event != "wxa_media_check" {
		return decodedWechatCallback{Disposition: wechatCallbackIgnore, Value: value}, nil
	}
	if value.MsgType != nil && msgType != "event" {
		if event == "wxa_media_check" {
			return decodedWechatCallback{}, errInvalidWechatReviewCallback
		}
		return decodedWechatCallback{Disposition: wechatCallbackIgnore, Value: value}, nil
	}
	if value.ErrCode != nil && *value.ErrCode != 0 {
		if strings.TrimSpace(value.TraceID) == "" {
			return decodedWechatCallback{}, errInvalidWechatReviewCallback
		}
		failureCode := wechatMediaPlatformFailureCode
		if *value.ErrCode == -1008 {
			failureCode = wechatMediaDownloadFailureCode
		}
		return decodedWechatCallback{
			Disposition: wechatCallbackApplyFailure,
			Value:       value,
			FailureCode: failureCode,
		}, nil
	}

	legacy := value.MsgType == nil && value.Event == nil
	if legacy {
		if validateWechatReviewResult(value) != nil {
			return decodedWechatCallback{}, errInvalidWechatReviewCallback
		}
		return decodedWechatCallback{Disposition: wechatCallbackApplyResult, Value: value}, nil
	}
	if value.MsgType == nil || value.Event == nil || msgType != "event" ||
		event != "wxa_media_check" || strings.TrimSpace(value.TraceID) == "" {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	if validateWechatReviewResult(value) != nil {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	return decodedWechatCallback{Disposition: wechatCallbackApplyResult, Value: value}, nil
}

func decodeWechatReviewEnvelope(body []byte) (wechatReviewResult, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	if json.Valid(body) {
		return decodeWechatReviewJSONEnvelope(body)
	}
	return decodeWechatReviewXMLEnvelope(body)
}

func decodeWechatReviewJSONEnvelope(body []byte) (wechatReviewResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	var value wechatReviewResult
	seenMsgType := false
	seenEvent := false
	seenErrCode := false
	seenEncrypt := false
	seenTraceID := false
	seenResult := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok {
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
		switch key {
		case "MsgType":
			if seenMsgType {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenMsgType = true
			value.MsgType = new(string)
			*value.MsgType, err = decodeWechatJSONString(decoder)
		case "Event":
			if seenEvent {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenEvent = true
			value.Event = new(string)
			*value.Event, err = decodeWechatJSONString(decoder)
		case "errcode":
			if seenErrCode {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenErrCode = true
			var parsed int64
			parsed, err = decodeWechatJSONInt64(decoder)
			if err == nil {
				value.ErrCode = &parsed
			}
		case "Encrypt":
			if seenEncrypt {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenEncrypt = true
			_, err = decodeWechatJSONString(decoder)
			value.Encrypted = err == nil
		case "trace_id":
			if seenTraceID {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenTraceID = true
			value.TraceID, err = decodeWechatJSONString(decoder)
		case "result":
			if seenResult {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			seenResult = true
			err = decodeWechatReviewJSONResult(decoder, &value)
		default:
			err = skipWechatJSONValue(decoder)
		}
		if err != nil {
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	return value, nil
}

func decodeWechatReviewJSONResult(decoder *json.Decoder, value *wechatReviewResult) error {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errInvalidWechatReviewCallback
	}
	seenSuggest := false
	seenLabel := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok {
			return errInvalidWechatReviewCallback
		}
		switch key {
		case "suggest":
			if seenSuggest {
				return errInvalidWechatReviewCallback
			}
			seenSuggest = true
			value.Result.Suggest, err = decodeWechatJSONString(decoder)
		case "label":
			if seenLabel {
				return errInvalidWechatReviewCallback
			}
			seenLabel = true
			var label wechatReviewLabel
			label, err = decodeWechatReviewJSONLabel(decoder)
			if err == nil {
				value.Result.Label = &label
			}
		default:
			err = skipWechatJSONValue(decoder)
		}
		if err != nil {
			return errInvalidWechatReviewCallback
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return errInvalidWechatReviewCallback
	}
	return nil
}

func decodeWechatJSONString(decoder *json.Decoder) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", errInvalidWechatReviewCallback
	}
	value, ok := token.(string)
	if !ok {
		return "", errInvalidWechatReviewCallback
	}
	return value, nil
}

func decodeWechatReviewJSONLabel(decoder *json.Decoder) (wechatReviewLabel, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	switch value := token.(type) {
	case string:
		return parseWechatReviewLabel(value)
	case json.Number:
		return parseWechatReviewLabel(string(value))
	default:
		return 0, errInvalidWechatReviewCallback
	}
}

func parseWechatSignedInteger(raw string) (int64, error) {
	if raw == "" {
		return 0, errInvalidWechatReviewCallback
	}
	index := 0
	if raw[0] == '-' {
		index = 1
		if len(raw) == 1 {
			return 0, errInvalidWechatReviewCallback
		}
	}
	for ; index < len(raw); index++ {
		if raw[index] < '0' || raw[index] > '9' {
			return 0, errInvalidWechatReviewCallback
		}
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	return parsed, nil
}

func decodeWechatJSONInt64(decoder *json.Decoder) (int64, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	value, ok := token.(json.Number)
	if !ok {
		return 0, errInvalidWechatReviewCallback
	}
	return parseWechatSignedInteger(string(value))
}

func skipWechatJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return errInvalidWechatReviewCallback
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		for decoder.More() {
			key, err := decoder.Token()
			if _, ok := key.(string); err != nil || !ok {
				return errInvalidWechatReviewCallback
			}
			if err := skipWechatJSONValue(decoder); err != nil {
				return err
			}
		}
		if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
			return errInvalidWechatReviewCallback
		}
	case '[':
		for decoder.More() {
			if err := skipWechatJSONValue(decoder); err != nil {
				return err
			}
		}
		if token, err = decoder.Token(); err != nil || token != json.Delim(']') {
			return errInvalidWechatReviewCallback
		}
	default:
		return errInvalidWechatReviewCallback
	}
	return nil
}

func decodeWechatReviewXMLEnvelope(body []byte) (wechatReviewResult, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, err := decoder.Token()
		if err != nil {
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
		switch value := token.(type) {
		case xml.StartElement:
			if isWechatReviewCriticalXMLName(value.Name.Local) {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			result, err := decodeWechatReviewXMLRoot(decoder, value)
			if err != nil {
				return wechatReviewResult{}, err
			}
			if err := requireWechatXMLEOF(decoder); err != nil {
				return wechatReviewResult{}, err
			}
			return result, nil
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
		case xml.Comment:
			// Comments carry no protocol meaning.
		case xml.Directive, xml.ProcInst:
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		default:
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
	}
}

func decodeWechatReviewXMLRoot(decoder *xml.Decoder, root xml.StartElement) (wechatReviewResult, error) {
	var value wechatReviewResult
	seenMsgType := false
	seenEvent := false
	seenErrCode := false
	seenEncrypt := false
	seenTraceID := false
	seenResult := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "MsgType":
				if seenMsgType {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenMsgType = true
				value.MsgType = new(string)
				*value.MsgType, err = decodeWechatXMLScalar(decoder, element)
			case "Event":
				if seenEvent {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenEvent = true
				value.Event = new(string)
				*value.Event, err = decodeWechatXMLScalar(decoder, element)
			case "errcode":
				if seenErrCode {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenErrCode = true
				var raw string
				raw, err = decodeWechatXMLScalar(decoder, element)
				if err == nil {
					var parsed int64
					parsed, err = parseWechatSignedInteger(strings.TrimSpace(raw))
					if err == nil {
						value.ErrCode = &parsed
					}
				}
			case "Encrypt":
				if seenEncrypt {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenEncrypt = true
				_, err = decodeWechatXMLScalar(decoder, element)
				value.Encrypted = err == nil
			case "trace_id":
				if seenTraceID {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenTraceID = true
				value.TraceID, err = decodeWechatXMLScalar(decoder, element)
			case "result":
				if seenResult {
					return wechatReviewResult{}, errInvalidWechatReviewCallback
				}
				seenResult = true
				err = decodeWechatReviewXMLResult(decoder, element, &value)
			case "suggest", "label":
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			default:
				err = skipWechatUnknownXMLElement(decoder, element)
			}
			if err != nil {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
		case xml.EndElement:
			if element.Name != root.Name {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
			return value, nil
		case xml.CharData:
			if strings.TrimSpace(string(element)) != "" {
				return wechatReviewResult{}, errInvalidWechatReviewCallback
			}
		case xml.Comment:
			// Comments carry no protocol meaning.
		case xml.Directive, xml.ProcInst:
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		default:
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
	}
}

func decodeWechatReviewXMLResult(decoder *xml.Decoder, root xml.StartElement, value *wechatReviewResult) error {
	seenSuggest := false
	seenLabel := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return errInvalidWechatReviewCallback
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "suggest":
				if seenSuggest {
					return errInvalidWechatReviewCallback
				}
				seenSuggest = true
				value.Result.Suggest, err = decodeWechatXMLScalar(decoder, element)
			case "label":
				if seenLabel {
					return errInvalidWechatReviewCallback
				}
				seenLabel = true
				var raw string
				raw, err = decodeWechatXMLScalar(decoder, element)
				if err == nil {
					var label wechatReviewLabel
					label, err = parseWechatReviewLabel(strings.TrimSpace(raw))
					if err == nil {
						value.Result.Label = &label
					}
				}
			case "MsgType", "Event", "errcode", "Encrypt", "trace_id", "result":
				return errInvalidWechatReviewCallback
			default:
				err = skipWechatUnknownXMLElement(decoder, element)
			}
			if err != nil {
				return errInvalidWechatReviewCallback
			}
		case xml.EndElement:
			if element.Name != root.Name {
				return errInvalidWechatReviewCallback
			}
			return nil
		case xml.CharData:
			if strings.TrimSpace(string(element)) != "" {
				return errInvalidWechatReviewCallback
			}
		case xml.Comment:
			// Comments carry no protocol meaning.
		case xml.Directive, xml.ProcInst:
			return errInvalidWechatReviewCallback
		default:
			return errInvalidWechatReviewCallback
		}
	}
}

func decodeWechatXMLScalar(decoder *xml.Decoder, root xml.StartElement) (string, error) {
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", errInvalidWechatReviewCallback
		}
		switch element := token.(type) {
		case xml.CharData:
			value.Write(element)
		case xml.EndElement:
			if element.Name != root.Name {
				return "", errInvalidWechatReviewCallback
			}
			return value.String(), nil
		case xml.Comment:
			// Comments carry no protocol meaning.
		case xml.StartElement, xml.Directive, xml.ProcInst:
			return "", errInvalidWechatReviewCallback
		default:
			return "", errInvalidWechatReviewCallback
		}
	}
}

func skipWechatUnknownXMLElement(decoder *xml.Decoder, root xml.StartElement) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return errInvalidWechatReviewCallback
		}
		switch element := token.(type) {
		case xml.StartElement:
			if isWechatReviewCriticalXMLName(element.Name.Local) {
				return errInvalidWechatReviewCallback
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && element.Name != root.Name {
				return errInvalidWechatReviewCallback
			}
		case xml.Directive, xml.ProcInst:
			return errInvalidWechatReviewCallback
		}
	}
	return nil
}

func requireWechatXMLEOF(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errInvalidWechatReviewCallback
		}
		switch value := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return errInvalidWechatReviewCallback
			}
		case xml.Comment:
			// Comments carry no protocol meaning.
		case xml.Directive, xml.ProcInst, xml.StartElement, xml.EndElement:
			return errInvalidWechatReviewCallback
		default:
			return errInvalidWechatReviewCallback
		}
	}
}

func isWechatReviewCriticalXMLName(name string) bool {
	switch name {
	case "MsgType", "Event", "errcode", "Encrypt", "trace_id", "result", "suggest", "label":
		return true
	default:
		return false
	}
}

func validateWechatReviewResult(value wechatReviewResult) error {
	if strings.TrimSpace(value.TraceID) == "" || strings.TrimSpace(value.Result.Suggest) == "" || value.Result.Label == nil {
		return errInvalidWechatReviewCallback
	}
	return nil
}

func (h *WechatCallbackHandler) callback(c *gin.Context) {
	if !h.valid(c) {
		c.Status(http.StatusForbidden)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWechatReviewCallbackBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logWechatCallbackCategory(wechatStructureUnrecognized, wechatCategoryBodyReadError)
		c.Status(http.StatusBadRequest)
		return
	}
	decoded, err := decodeWechatReviewCallback(body)
	if err != nil {
		logWechatCallbackRejection(body)
		c.Status(http.StatusBadRequest)
		return
	}
	switch decoded.Disposition {
	case wechatCallbackIgnore:
		structure, category := diagnoseWechatCallback(body)
		logWechatCallbackCategory(structure, category)
		c.String(http.StatusOK, "success")
		return
	case wechatCallbackApplyFailure:
		structure, category := diagnoseWechatCallback(body)
		logWechatCallbackCategory(structure, category)
		err = h.service.ApplyWechatReviewFailure(
			c.Request.Context(), decoded.Value.TraceID, decoded.FailureCode,
		)
	case wechatCallbackApplyResult:
		err = h.service.ApplyWechatReviewResult(
			c.Request.Context(), decoded.Value.TraceID,
			decoded.Value.Result.Suggest, int(*decoded.Value.Result.Label),
		)
	default:
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}

func (h *WechatCallbackHandler) valid(c *gin.Context) bool {
	return VerifyWechatReviewSignature(
		h.callbackToken,
		c.Query("timestamp"),
		c.Query("nonce"),
		c.Query("signature"),
	)
}
