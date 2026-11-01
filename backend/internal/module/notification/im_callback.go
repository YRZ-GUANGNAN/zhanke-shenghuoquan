package notification

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	IMCallbackC2CAfterSend   = "C2C.CallbackAfterSendMsg"
	IMCallbackGroupAfterSend = "Group.CallbackAfterSendMsg"
	IMCallbackC2C            = "c2c"
	IMCallbackGroup          = "group"
)

var (
	ErrIMCallbackAuth        = errors.New("invalid Tencent IM callback authentication")
	ErrIMCallbackPayload     = errors.New("invalid Tencent IM callback payload")
	ErrIMCallbackUnsupported = errors.New("unsupported Tencent IM callback command")
	ErrIMCallbackRelation    = errors.New("Tencent IM callback relation is unavailable")
	ErrIMCallbackRejected    = errors.New("Tencent IM callback message was not delivered")
)

type IMCallbackConfig struct {
	SDKAppID uint64
	Token    string
	Now      func() time.Time
	MaxSkew  time.Duration
}

type IMCallback struct {
	SDKAppID           uint64
	Command            string
	ConversationType   string
	SenderAccount      string
	RecipientAccount   string
	GroupID            string
	ProviderMessageID  string
	MsgSequence        uint32
	UnreadMessageCount int64
	OccurredAt         time.Time
	CallbackHash       string
}

// ParseIMCallback verifies Tencent's URL signature and extracts only callback
// metadata. MsgBody, CloudCustomData and other message content are skipped by
// the streaming decoder and are never retained or logged.
func ParseIMCallback(rawURL string, body []byte, cfg IMCallbackConfig) (IMCallback, error) {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.MaxSkew <= 0 || cfg.MaxSkew > time.Minute {
		cfg.MaxSkew = time.Minute
	}
	if cfg.SDKAppID == 0 || strings.TrimSpace(cfg.Token) == "" || len(body) == 0 || len(body) > 128<<10 {
		return IMCallback{}, ErrIMCallbackAuth
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return IMCallback{}, ErrIMCallbackAuth
	}
	query := parsed.Query()
	if err := verifyIMCallbackQuery(query, cfg); err != nil {
		return IMCallback{}, err
	}
	fields, err := decodeCallbackMetadata(body)
	if err != nil {
		return IMCallback{}, ErrIMCallbackPayload
	}
	command, err := requiredString(fields, "CallbackCommand", 96)
	if err != nil {
		return IMCallback{}, ErrIMCallbackPayload
	}
	if commands, ok := query["CallbackCommand"]; ok && (len(commands) != 1 || commands[0] != command) {
		return IMCallback{}, ErrIMCallbackPayload
	}
	sdkAppID := cfg.SDKAppID
	if raw, ok := fields["SDKAppID"]; ok {
		value, parseErr := parseUint(raw, 64)
		if parseErr != nil || value != cfg.SDKAppID {
			return IMCallback{}, ErrIMCallbackAuth
		}
		sdkAppID = value
	}
	callback := IMCallback{SDKAppID: sdkAppID, Command: command, CallbackHash: hashCallback(body)}
	switch command {
	case IMCallbackC2CAfterSend:
		callback.ConversationType = IMCallbackC2C
		callback.SenderAccount, err = requiredString(fields, "From_Account", 64)
		if err != nil {
			return IMCallback{}, ErrIMCallbackPayload
		}
		callback.RecipientAccount, err = requiredString(fields, "To_Account", 64)
		if err != nil || callback.SenderAccount == callback.RecipientAccount {
			return IMCallback{}, ErrIMCallbackPayload
		}
	case IMCallbackGroupAfterSend:
		callback.ConversationType = IMCallbackGroup
		callback.GroupID, err = requiredString(fields, "GroupId", 115)
		if err != nil {
			return IMCallback{}, ErrIMCallbackPayload
		}
		callback.SenderAccount, err = requiredString(fields, "From_Account", 64)
		if err != nil {
			return IMCallback{}, ErrIMCallbackPayload
		}
	default:
		return IMCallback{}, ErrIMCallbackUnsupported
	}
	if result, ok := fields["SendMsgResult"]; ok {
		value, parseErr := parseInt(result, 64)
		if parseErr != nil || value != 0 {
			return IMCallback{}, ErrIMCallbackRejected
		}
	}
	if callback.ConversationType == IMCallbackC2C {
		if _, ok := fields["SendMsgResult"]; !ok {
			return IMCallback{}, ErrIMCallbackPayload
		}
	}
	providerID, err := providerMessageID(fields, callback)
	if err != nil {
		return IMCallback{}, ErrIMCallbackPayload
	}
	callback.ProviderMessageID = providerID
	if raw, ok := fields["MsgSeq"]; ok {
		value, parseErr := parseUint(raw, 32)
		if parseErr != nil || value == 0 {
			return IMCallback{}, ErrIMCallbackPayload
		}
		callback.MsgSequence = uint32(value)
	}
	if raw, ok := fields["UnreadMsgNum"]; ok {
		value, parseErr := parseInt(raw, 64)
		if parseErr != nil || value < 0 {
			return IMCallback{}, ErrIMCallbackPayload
		}
		callback.UnreadMessageCount = value
	}
	callback.OccurredAt = callbackTime(fields, cfg.Now())
	if callback.OccurredAt.IsZero() {
		return IMCallback{}, ErrIMCallbackPayload
	}
	return callback, nil
}

func verifyIMCallbackQuery(query url.Values, cfg IMCallbackConfig) error {
	for _, key := range []string{"SdkAppid", "sdkappid", "SDKAppID", "RequestTime", "Sign", "CallbackCommand"} {
		if values, ok := query[key]; ok && len(values) != 1 {
			return ErrIMCallbackAuth
		}
	}
	appIDs := 0
	for _, key := range []string{"SdkAppid", "sdkappid", "SDKAppID"} {
		if _, ok := query[key]; ok {
			appIDs++
		}
	}
	if appIDs != 1 {
		return ErrIMCallbackAuth
	}
	appID, err := queryUint(query, "SdkAppid", "sdkappid", "SDKAppID")
	if err != nil || appID != cfg.SDKAppID {
		return ErrIMCallbackAuth
	}
	requestTime, err := queryInt(query, "RequestTime")
	if err != nil {
		return ErrIMCallbackAuth
	}
	now := cfg.Now().UTC()
	if now.IsZero() || time.Unix(requestTime, 0).UTC().Sub(now).Abs() > cfg.MaxSkew {
		return ErrIMCallbackAuth
	}
	sign := strings.TrimSpace(query.Get("Sign"))
	if len(sign) != sha256.Size*2 {
		return ErrIMCallbackAuth
	}
	provided, err := hex.DecodeString(sign)
	if err != nil {
		return ErrIMCallbackAuth
	}
	expected := sha256.Sum256([]byte(cfg.Token + strconv.FormatInt(requestTime, 10)))
	if subtle.ConstantTimeCompare(provided, expected[:]) != 1 {
		return ErrIMCallbackAuth
	}
	return nil
}

func providerMessageID(fields map[string]json.RawMessage, callback IMCallback) (string, error) {
	for _, key := range []string{"MsgKey", "MsgId"} {
		if raw, ok := fields[key]; ok {
			if value, err := parseString(raw, 128); err == nil && value != "" {
				return value, nil
			}
		}
	}
	if callback.ConversationType == IMCallbackGroup {
		if raw, ok := fields["MsgSeq"]; ok {
			if value, err := parseUint(raw, 32); err == nil && value > 0 {
				return callback.GroupID + ":" + strconv.FormatUint(value, 10), nil
			}
		}
	}
	return "", ErrIMCallbackPayload
}

func callbackTime(fields map[string]json.RawMessage, fallback time.Time) time.Time {
	if raw, ok := fields["MsgTime"]; ok {
		if value, err := parseInt(raw, 64); err == nil && value > 0 {
			return time.Unix(value, 0).UTC()
		}
		return time.Time{}
	}
	if raw, ok := fields["EventTime"]; ok {
		if value, err := parseInt(raw, 64); err == nil && value > 0 {
			return time.UnixMilli(value).UTC()
		}
		return time.Time{}
	}
	return fallback.UTC()
}

func hashCallback(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// decodeCallbackMetadata validates all keys/depth before retaining only known
// scalar metadata. Message values are discarded, including unknown fields.
func decodeCallbackMetadata(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := callbackJSONValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrIMCallbackPayload
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, ErrIMCallbackPayload
	}
	fields := make(map[string]json.RawMessage)
	for _, key := range []string{"CallbackCommand", "SDKAppID", "From_Account", "To_Account", "GroupId", "SendMsgResult", "MsgKey", "MsgId", "MsgSeq", "UnreadMsgNum", "MsgTime", "EventTime"} {
		if value, ok := object[key]; ok {
			raw, err := json.Marshal(value)
			if err != nil {
				return nil, ErrIMCallbackPayload
			}
			fields[key] = raw
		}
	}
	return fields, nil
}

func callbackJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 12 {
		return nil, ErrIMCallbackPayload
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, ErrIMCallbackPayload
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || key == "" || len(key) > 128 {
				return nil, ErrIMCallbackPayload
			}
			if _, exists := object[key]; exists {
				return nil, ErrIMCallbackPayload
			}
			value, err := callbackJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrIMCallbackPayload
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := callbackJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrIMCallbackPayload
		}
		return array, nil
	default:
		return nil, ErrIMCallbackPayload
	}
}

func requiredString(fields map[string]json.RawMessage, key string, max int) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", ErrIMCallbackPayload
	}
	return parseString(raw, max)
}

func parseString(raw json.RawMessage, max int) (string, error) {
	var value string
	if json.Unmarshal(raw, &value) != nil || value == "" || len(value) > max || strings.ContainsAny(value, "\r\n\x00") {
		return "", ErrIMCallbackPayload
	}
	return value, nil
}

func parseUint(raw json.RawMessage, bits int) (uint64, error) {
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, ErrIMCallbackPayload
	}
	return strconv.ParseUint(string(value), 10, bits)
}

func parseInt(raw json.RawMessage, bits int) (int64, error) {
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, ErrIMCallbackPayload
	}
	return strconv.ParseInt(string(value), 10, bits)
}

func queryUint(query url.Values, keys ...string) (uint64, error) {
	for _, key := range keys {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return strconv.ParseUint(value, 10, 64)
		}
	}
	return 0, ErrIMCallbackAuth
}

func queryInt(query url.Values, key string) (int64, error) {
	value := strings.TrimSpace(query.Get(key))
	if value == "" {
		return 0, ErrIMCallbackAuth
	}
	return strconv.ParseInt(value, 10, 64)
}
