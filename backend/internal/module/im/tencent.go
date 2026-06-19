package im

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AccountImporter interface {
	ImportAccount(context.Context, string) error
}

type TencentRESTClient struct {
	sdkAppID, secret, adminUser, endpoint string
	httpClient                            *http.Client
}

func NewTencentRESTClient(sdkAppID, secret, adminUser, endpoint string) (*TencentRESTClient, error) {
	if sdkAppID == "" || secret == "" || adminUser == "" {
		return nil, errors.New("Tencent IM SDKAppID, secret and administrator are required")
	}
	if endpoint == "" {
		endpoint = "https://console.tim.qq.com"
	}
	return &TencentRESTClient{
		sdkAppID: sdkAppID, secret: secret, adminUser: adminUser, endpoint: strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (c *TencentRESTClient) ImportAccount(ctx context.Context, imUserID string) error {
	adminSig, err := generateUserSig(c.sdkAppID, c.secret, c.adminUser, 300)
	if err != nil {
		return err
	}
	var randomBytes [4]byte
	if _, err := crand.Read(randomBytes[:]); err != nil {
		return err
	}
	query := url.Values{
		"sdkappid": {c.sdkAppID}, "identifier": {c.adminUser}, "usersig": {adminSig},
		"random": {fmt.Sprintf("%d", binary.BigEndian.Uint32(randomBytes[:]))}, "contenttype": {"json"},
	}
	body, err := json.Marshal(map[string]string{"Identifier": imUserID})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v4/im_open_login_svc/account_import?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Tencent IM account import: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		ActionStatus string `json:"ActionStatus"`
		ErrorCode    int    `json:"ErrorCode"`
		ErrorInfo    string `json:"ErrorInfo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Tencent IM account import response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || result.ActionStatus != "OK" || result.ErrorCode != 0 {
		return fmt.Errorf("Tencent IM account import failed: http=%d code=%d info=%s", resp.StatusCode, result.ErrorCode, result.ErrorInfo)
	}
	return nil
}
