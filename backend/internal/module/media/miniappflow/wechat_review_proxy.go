package miniappflow

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"

	"github.com/gin-gonic/gin"
)

const (
	WechatReviewContentPath = "/api/v1/miniapp/media/wechat/review-content"

	wechatReviewProxyMaxFuture = 36 * time.Minute
	wechatReviewProxyMaxToken  = 2048
	wechatReviewProxyMaxBytes  = uint64(10 << 20)
	wechatReviewProxyKDFDomain = "campus/wechat-review-proxy/v1\x00"
	wechatReviewProxyAAD       = "campus/wechat-review-proxy:v1"
)

var errInvalidWechatReviewProxy = errors.New("invalid wechat review proxy capability")

type WechatReviewContentStore interface {
	HeadObject(context.Context, string, string) (coremedia.CompletionObject, error)
	OpenObject(context.Context, string, string) (io.ReadCloser, error)
}

type WechatReviewProxyConfig struct {
	PublicBaseURL string
	AppSecret     string
}

type WechatReviewProxy struct {
	publicBaseURL  string
	store          WechatReviewContentStore
	codec          *wechatReviewProxyCodec
	now            func() time.Time
	logStreamError func()
}

type wechatReviewProxyClaims struct {
	Version   int    `json:"version"`
	ExpiresAt int64  `json:"expiresAt"`
	ObjectKey string `json:"objectKey"`
	VersionID string `json:"versionId"`
}

type wechatReviewProxyCodec struct {
	aead cipher.AEAD
}

func NewWechatReviewProxy(config WechatReviewProxyConfig, store WechatReviewContentStore) (*WechatReviewProxy, error) {
	parsed, err := url.Parse(strings.TrimSpace(config.PublicBaseURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		parsed.RawPath != "" || (parsed.Path != "" && parsed.Path != "/") || isNilWechatReviewValue(store) {
		return nil, errInvalidWechatReviewProxy
	}
	codec, err := newWechatReviewProxyCodec(config.AppSecret)
	if err != nil {
		return nil, err
	}
	return &WechatReviewProxy{
		publicBaseURL: parsed.Scheme + "://" + parsed.Host,
		store:         store,
		codec:         codec,
		now:           func() time.Time { return time.Now().UTC() },
		logStreamError: func() {
			log.Print("wechat_review_proxy category=stream_error")
		},
	}, nil
}

func isNilWechatReviewValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func newWechatReviewProxyCodec(appSecret string) (*wechatReviewProxyCodec, error) {
	appSecret = strings.TrimSpace(appSecret)
	if appSecret == "" {
		return nil, errInvalidWechatReviewProxy
	}
	sum := sha256.Sum256([]byte(wechatReviewProxyKDFDomain + appSecret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &wechatReviewProxyCodec{aead: aead}, nil
}

func (c *wechatReviewProxyCodec) seal(claims wechatReviewProxyClaims) (string, error) {
	plaintext, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, []byte(wechatReviewProxyAAD))
	raw := make([]byte, 0, len(nonce)+len(sealed))
	raw = append(raw, nonce...)
	raw = append(raw, sealed...)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (c *wechatReviewProxyCodec) open(token string) (wechatReviewProxyClaims, error) {
	if token == "" || len(token) > wechatReviewProxyMaxToken {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < c.aead.NonceSize()+c.aead.Overhead() {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	plaintext, err := c.aead.Open(
		nil,
		raw[:c.aead.NonceSize()],
		raw[c.aead.NonceSize():],
		[]byte(wechatReviewProxyAAD),
	)
	if err != nil {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return decodeWechatReviewProxyClaims(plaintext)
}

func decodeWechatReviewProxyClaims(raw []byte) (wechatReviewProxyClaims, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	var claims wechatReviewProxyClaims
	seen := map[string]bool{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok || seen[key] {
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
		seen[key] = true
		switch key {
		case "version":
			err = decoder.Decode(&claims.Version)
		case "expiresAt":
			err = decoder.Decode(&claims.ExpiresAt)
		case "objectKey":
			err = decoder.Decode(&claims.ObjectKey)
		case "versionId":
			err = decoder.Decode(&claims.VersionID)
		default:
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
		if err != nil {
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return claims, nil
}

func (p *WechatReviewProxy) SignedGetURL(ctx context.Context, key, versionID string, ttl time.Duration) (string, error) {
	if ctx == nil {
		return "", errInvalidWechatReviewProxy
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	if ttl <= 0 || ttl > WechatReviewURLTTL || !validWechatReviewObjectIdentity(key, versionID) {
		return "", errInvalidWechatReviewProxy
	}
	claims := wechatReviewProxyClaims{
		Version:   1,
		ExpiresAt: p.now().UTC().Add(ttl).Unix(),
		ObjectKey: key,
		VersionID: versionID,
	}
	token, err := p.codec.seal(claims)
	if err != nil {
		return "", err
	}
	query := url.Values{"token": {token}}
	return p.publicBaseURL + WechatReviewContentPath + "?" + query.Encode(), nil
}

func (p *WechatReviewProxy) openClaims(token string) (wechatReviewProxyClaims, error) {
	claims, err := p.codec.open(token)
	if err != nil || !validWechatReviewProxyClaims(claims, p.now().UTC()) {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return claims, nil
}

func validWechatReviewProxyClaims(claims wechatReviewProxyClaims, now time.Time) bool {
	return claims.Version == 1 && claims.ExpiresAt > now.Unix() &&
		claims.ExpiresAt <= now.Add(wechatReviewProxyMaxFuture).Unix() &&
		validWechatReviewObjectIdentity(claims.ObjectKey, claims.VersionID)
}

func validWechatReviewObjectIdentity(key, versionID string) bool {
	if key == "" || len(key) > 512 || key != strings.TrimSpace(key) ||
		versionID == "" || len(versionID) > 128 || versionID != strings.TrimSpace(versionID) ||
		strings.Contains(key, "..") || strings.Contains(key, "\\") || strings.Contains(key, "//") || path.Clean(key) != key ||
		!strings.HasPrefix(key, "final/") || strings.ContainsAny(key, "\r\n\x00") ||
		strings.ContainsAny(versionID, "\r\n\x00") {
		return false
	}
	switch strings.ToLower(path.Ext(key)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func (p *WechatReviewProxy) Register(group *gin.RouterGroup) {
	group.Any("/wechat/review-content", p.serve)
}

func (p *WechatReviewProxy) serve(c *gin.Context) {
	if c.Request.Method != http.MethodHead && c.Request.Method != http.MethodGet {
		c.Header("Allow", "HEAD, GET")
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	values := c.Request.URL.Query()
	tokens, ok := values["token"]
	if !ok || len(values) != 1 || len(tokens) != 1 || tokens[0] == "" || len(tokens[0]) > wechatReviewProxyMaxToken {
		c.Status(http.StatusNotFound)
		return
	}
	claims, err := p.openClaims(tokens[0])
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	object, err := p.store.HeadObject(c.Request.Context(), claims.ObjectKey, claims.VersionID)
	if err != nil {
		p.writeStorageError(c, err)
		return
	}
	if object.Key != claims.ObjectKey || object.VersionID != claims.VersionID ||
		!allowedWechatReviewMIME(object.ContentType) || object.Size == 0 || object.Size > wechatReviewProxyMaxBytes {
		c.Status(http.StatusNotFound)
		return
	}
	if c.Request.Method == http.MethodHead {
		writeWechatReviewContentHeaders(c, object)
		c.Status(http.StatusOK)
		return
	}
	body, err := p.store.OpenObject(c.Request.Context(), claims.ObjectKey, claims.VersionID)
	bodyNil := isNilWechatReviewValue(body)
	if err != nil || bodyNil {
		if !bodyNil {
			_ = body.Close()
		}
		if err == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		p.writeStorageError(c, err)
		return
	}
	defer body.Close()
	firstSize := int64(32 << 10)
	if int64(object.Size) < firstSize {
		firstSize = int64(object.Size)
	}
	first := make([]byte, firstSize)
	if _, err := io.ReadFull(body, first); err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	writeWechatReviewContentHeaders(c, object)
	c.Status(http.StatusOK)
	written, err := c.Writer.Write(first)
	if err != nil || written != len(first) {
		p.logStreamError()
		return
	}
	remaining := int64(object.Size) - int64(len(first))
	if remaining > 0 {
		if _, err := io.CopyN(c.Writer, body, remaining); err != nil {
			p.logStreamError()
		}
	}
}

func (p *WechatReviewProxy) writeStorageError(c *gin.Context, err error) {
	if errors.Is(err, coremedia.ErrObjectNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusServiceUnavailable)
}

func writeWechatReviewContentHeaders(c *gin.Context, object coremedia.CompletionObject) {
	c.Header("Content-Type", object.ContentType)
	c.Header("Content-Length", strconv.FormatUint(object.Size, 10))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
}

func allowedWechatReviewMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

var _ WechatReviewURLSigner = (*WechatReviewProxy)(nil)
