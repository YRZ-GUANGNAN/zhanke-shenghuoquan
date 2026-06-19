package user

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const tokenIssuer = "campus-circle"

var (
	errInvalidToken = errors.New("invalid token")
	errTokenExpired = errors.New("token expired")
)

type Claims struct {
	Issuer       string `json:"iss"`
	Subject      string `json:"sub"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
	JWTID        string `json:"jti"`
	TokenVersion uint64 `json:"ver"`
}

func generateToken(secret string, userID uint, tokenVersion uint64, ttl time.Duration) (string, error) {
	now := time.Now()
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", err
	}
	header, err := encodeJSON(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := encodeJSON(Claims{
		Issuer: tokenIssuer, Subject: strconv.FormatUint(uint64(userID), 10),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(),
		JWTID: base64.RawURLEncoding.EncodeToString(jtiBytes), TokenVersion: tokenVersion,
	})
	if err != nil {
		return "", err
	}
	payload := header + "." + claims
	return payload + "." + sign(secret, payload), nil
}

func parseToken(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := decodeJSON(parts[0], &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, errInvalidToken
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(secret, payload)), []byte(parts[2])) {
		return Claims{}, errInvalidToken
	}
	var claims Claims
	if err := decodeJSON(parts[1], &claims); err != nil {
		return Claims{}, errInvalidToken
	}
	now := time.Now().Unix()
	if claims.Issuer != tokenIssuer || claims.Subject == "" || claims.JWTID == "" || claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return Claims{}, errInvalidToken
	}
	if now >= claims.ExpiresAt {
		return Claims{}, errTokenExpired
	}
	if claims.IssuedAt > now+60 {
		return Claims{}, errInvalidToken
	}
	return claims, nil
}

func (c Claims) UserID() (uint, error) {
	id, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil || id == 0 {
		return 0, errInvalidToken
	}
	return uint(id), nil
}

func encodeJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeJSON(value string, target any) error {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
