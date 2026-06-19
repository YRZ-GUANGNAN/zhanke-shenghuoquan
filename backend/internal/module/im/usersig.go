package im

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func generateUserSig(sdkAppID, secret, userID string, ttlSeconds int64) (string, error) {
	now := time.Now().Unix()
	toSign := fmt.Sprintf("TLS.identifier:%s\nTLS.sdkappid:%s\nTLS.time:%d\nTLS.expire:%d\n", userID, sdkAppID, now, ttlSeconds)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(toSign))
	payload := map[string]any{"TLS.ver": "2.0", "TLS.identifier": userID, "TLS.sdkappid": sdkAppID, "TLS.expire": ttlSeconds, "TLS.time": now, "TLS.sig": base64.StdEncoding.EncodeToString(mac.Sum(nil))}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err = writer.Write(raw); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(compressed.Bytes())
	encoded = strings.ReplaceAll(encoded, "+", "*")
	encoded = strings.ReplaceAll(encoded, "/", "-")
	encoded = strings.ReplaceAll(encoded, "=", "_")
	return encoded, nil
}
