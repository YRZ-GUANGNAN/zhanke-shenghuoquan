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

type WorkGroupManager interface {
	CreateWorkGroup(context.Context, string, string, string, []string) error
	AddWorkGroupMembers(context.Context, string, []string) error
	RemoveWorkGroupMembers(context.Context, string, []string) error
	SetWorkGroupReadOnly(context.Context, string, bool) error
	DismissWorkGroup(context.Context, string) error
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

type TencentRESTError struct {
	Operation string
	HTTPCode  int
	Code      int
	Info      string
}

func (e *TencentRESTError) Error() string {
	return fmt.Sprintf("Tencent IM %s failed: http=%d code=%d info=%s", e.Operation, e.HTTPCode, e.Code, e.Info)
}

func (c *TencentRESTClient) CreateWorkGroup(
	ctx context.Context,
	groupID, name, owner string,
	members []string,
) error {
	memberList := make([]map[string]string, 0, len(members))
	for _, member := range uniqueNonEmpty(members) {
		if member != owner {
			memberList = append(memberList, map[string]string{"Member_Account": member})
		}
	}
	body := map[string]any{
		"Type":       "Private",
		"GroupId":    groupID,
		"Name":       name,
		"MemberList": memberList,
	}
	if owner = strings.TrimSpace(owner); owner != "" {
		body["Owner_Account"] = owner
	}
	err := c.groupCall(ctx, "create_group", body, nil)
	var restErr *TencentRESTError
	if errors.As(err, &restErr) && (restErr.Code == 10021 || restErr.Code == 10025) {
		return c.AddWorkGroupMembers(ctx, groupID, append(members, owner))
	}
	return err
}

func (c *TencentRESTClient) AddWorkGroupMembers(
	ctx context.Context,
	groupID string,
	members []string,
) error {
	memberList := make([]map[string]string, 0, len(members))
	for _, member := range uniqueNonEmpty(members) {
		memberList = append(memberList, map[string]string{"Member_Account": member})
	}
	var result struct {
		MemberList []struct {
			Account string `json:"Member_Account"`
			Result  int    `json:"Result"`
		} `json:"MemberList"`
	}
	if err := c.groupCall(ctx, "add_group_member", map[string]any{
		"GroupId": groupID, "MemberList": memberList,
	}, &result); err != nil {
		return err
	}
	for _, member := range result.MemberList {
		if member.Result != 1 && member.Result != 2 {
			return fmt.Errorf("Tencent IM add group member failed: member=%s result=%d", member.Account, member.Result)
		}
	}
	return nil
}

func (c *TencentRESTClient) RemoveWorkGroupMembers(
	ctx context.Context,
	groupID string,
	members []string,
) error {
	err := c.groupCall(ctx, "delete_group_member", map[string]any{
		"GroupId": groupID, "Silence": 1, "MemberToDel_Account": uniqueNonEmpty(members),
	}, nil)
	var restErr *TencentRESTError
	if errors.As(err, &restErr) && restErr.Code == 10010 {
		return nil
	}
	return err
}

func (c *TencentRESTClient) SetWorkGroupReadOnly(
	ctx context.Context,
	groupID string,
	readOnly bool,
) error {
	mute := "Off"
	if readOnly {
		mute = "On"
	}
	return c.groupCall(ctx, "modify_group_base_info", map[string]string{
		"GroupId": groupID, "MuteAllMember": mute,
	}, nil)
}

func (c *TencentRESTClient) DismissWorkGroup(ctx context.Context, groupID string) error {
	err := c.groupCall(ctx, "destroy_group", map[string]string{"GroupId": groupID}, nil)
	var restErr *TencentRESTError
	if errors.As(err, &restErr) && restErr.Code == 10010 {
		return nil
	}
	return err
}

func (c *TencentRESTClient) groupCall(
	ctx context.Context,
	operation string,
	body any,
	result any,
) error {
	return c.restCall(ctx, "group_open_http_svc", operation, body, result)
}

func (c *TencentRESTClient) restCall(
	ctx context.Context,
	service, operation string,
	body any,
	result any,
) error {
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
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	target := c.endpoint + "/v4/" + service + "/" + operation + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Tencent IM %s: %w", operation, err)
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("Tencent IM %s response: %w", operation, err)
	}
	var base struct {
		ActionStatus string `json:"ActionStatus"`
		ErrorCode    int    `json:"ErrorCode"`
		ErrorInfo    string `json:"ErrorInfo"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return fmt.Errorf("Tencent IM %s response: %w", operation, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || base.ActionStatus != "OK" || base.ErrorCode != 0 {
		return &TencentRESTError{
			Operation: operation, HTTPCode: resp.StatusCode, Code: base.ErrorCode, Info: base.ErrorInfo,
		}
	}
	if result != nil {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("Tencent IM %s response payload: %w", operation, err)
		}
	}
	return nil
}

func uniqueNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

var _ WorkGroupManager = (*TencentRESTClient)(nil)
