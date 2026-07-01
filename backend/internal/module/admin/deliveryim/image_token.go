package deliveryim

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errInvalidImageToken = errors.New("invalid delivery conversation image token")

type ImageTokenCodec struct{ key []byte }

func NewImageTokenCodec(secret string) (*ImageTokenCodec, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 16 {
		return nil, errInvalidImageToken
	}
	return &ImageTokenCodec{key: []byte(secret)}, nil
}

func (c *ImageTokenCodec) Encode(conversationID string, slot uint16) string {
	payload := []byte(fmt.Sprintf("%s:%d", conversationID, slot))
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write(payload)
	signature := mac.Sum(nil)[:16]
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signature)
}

func (c *ImageTokenCodec) Decode(token, expectedConversationID string) (uint16, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errInvalidImageToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errInvalidImageToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, errInvalidImageToken
	}
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)[:16]) {
		return 0, errInvalidImageToken
	}
	fields := strings.Split(string(payload), ":")
	if len(fields) != 2 || fields[0] != expectedConversationID {
		return 0, errInvalidImageToken
	}
	slot, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return 0, errInvalidImageToken
	}
	return uint16(slot), nil
}
