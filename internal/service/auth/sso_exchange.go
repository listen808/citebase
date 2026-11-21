package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SSOIdentity 通过授权码向 IdP 换取到的外部身份。
type SSOIdentity struct {
	Username  string
	GivenName string
	Email     string
}

// ExchangeSSOCode 用 IdP 授权码换取真实身份（authorization_code 换 access_token，再取身份信息）。
//
// 这条链路的意义在于「由服务端向 IdP 核实 code」：旧实现把用户自行提交的 code 直接当作
// 用户名并签发 JWT，任何人都能凭任意字符串接管任意账号，因此这里一律 fail-closed——
// 未配置 token_url、换取失败、响应无身份字段，都必须拒绝登录。
func ExchangeSSOCode(ctx context.Context, extData, code string) (*SSOIdentity, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("缺少授权码")
	}
	ext, err := parseExtConfig(extData)
	if err != nil {
		// 配置坏掉时 fail-closed：不能带着空配置去换授权码
		return nil, err
	}
	if ext.TokenURL == "" {
		return nil, errors.New("SSO 配置缺少 token_url，服务端无法校验授权码，已拒绝登录")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	if ext.ClientID != "" {
		form.Set("client_id", ext.ClientID)
	}
	if ext.ClientSecret != "" {
		form.Set("client_secret", ext.ClientSecret)
	}
	if ext.CallbackURL != "" {
		form.Set("redirect_uri", ext.CallbackURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ext.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构造授权码交换请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := newAuthHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("向 IdP 交换授权码失败: %w", err)
	}
	defer resp.Body.Close()
	payload, err := readAuthBody(resp)
	if err != nil {
		return nil, err
	}
	tokenResp, err := parseAuthJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("IdP 令牌响应无法解析: %w", err)
	}
	if authRejected(tokenResp) {
		return nil, fmt.Errorf("IdP 拒绝授权码: %s", authRejectReason(tokenResp))
	}

	accessToken := ssoFirstStr(mapField(tokenResp), tokenResp, "access_token")
	if accessToken == "" {
		return nil, errors.New("IdP 未返回 access_token")
	}

	if ext.UserInfoURL == "" {
		// 少数 IdP 直接在令牌响应里返回身份字段
		return identityFromResponse(tokenResp, ext.UserField)
	}

	infoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ext.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 userinfo 请求失败: %w", err)
	}
	infoReq.Header.Set("Authorization", "Bearer "+accessToken)
	infoResp, err := newAuthHTTPClient().Do(infoReq)
	if err != nil {
		return nil, fmt.Errorf("读取 userinfo 失败: %w", err)
	}
	defer infoResp.Body.Close()
	infoBody, err := readAuthBody(infoResp)
	if err != nil {
		return nil, err
	}
	info, err := parseAuthJSON(infoBody)
	if err != nil {
		return nil, fmt.Errorf("userinfo 响应无法解析: %w", err)
	}
	if authRejected(info) {
		return nil, fmt.Errorf("userinfo 返回失败: %s", authRejectReason(info))
	}
	return identityFromResponse(info, ext.UserField)
}

func identityFromResponse(info map[string]interface{}, userField string) (*SSOIdentity, error) {
	dataMap := mapField(info)
	name := ssoFirstStr(dataMap, info, "username")
	if name == "" && userField != "" {
		name = ssoFirstStr(dataMap, info, userField)
	}
	if name == "" {
		name = ssoFirstStr(dataMap, info, "sub")
	}
	if name == "" {
		return nil, errors.New("IdP 响应中不含用户身份字段，拒绝登录")
	}
	return &SSOIdentity{
		Username:  name,
		GivenName: ssoFirstStr(dataMap, info, "given_name"),
		Email:     ssoFirstStr(dataMap, info, "email"),
	}, nil
}

func readAuthBody(resp *http.Response) ([]byte, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IdP 返回状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 IdP 响应失败: %w", err)
	}
	return body, nil
}

func parseAuthJSON(body []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// mapField 取出嵌套的 data 对象（很多 IdP 把负载包在 data 里）。
func mapField(m map[string]interface{}) map[string]interface{} {
	if inner, ok := m["data"].(map[string]interface{}); ok {
		return inner
	}
	return map[string]interface{}{}
}
