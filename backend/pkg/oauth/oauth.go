// Package oauth 提供第三方 OAuth 登录能力，当前支持 GitHub OAuth App 流程。
//
// OAuth 流程：
//  1. 前端引导用户跳转至 GetAuthURL() 返回的 GitHub 授权页面
//  2. 用户授权后 GitHub 回调 RedirectURI，携带 code 参数
//  3. 后端用 ExchangeCode(code) 换取 AccessToken
//  4. 用 AccessToken 调用 GetUser() 获取 GitHub 用户信息
//  5. 根据 GitHub 用户信息创建或匹配本站用户，签发 JWT
package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubUser 是 GitHub API /user 端点返回的用户信息。
type GitHubUser struct {
	ID        int64  `json:"id"`         // GitHub 用户唯一 ID
	Login     string `json:"login"`      // GitHub 用户名
	Name      string `json:"name"`       // 显示名称
	Email     string `json:"email"`      // 公开邮箱
	AvatarURL string `json:"avatar_url"` // 头像 URL
}

// GitHubOAuth 封装 GitHub OAuth App 的授权流程。
type GitHubOAuth struct {
	ClientID     string       // GitHub OAuth App Client ID
	ClientSecret string       // GitHub OAuth App Client Secret
	RedirectURI  string       // 授权回调地址
	FrontendURL  string       // 前端地址，登录成功后重定向到此 URL 并携带 token
	httpClient   *http.Client // 支持代理的 HTTP 客户端
}

// NewGitHubOAuth 创建 GitHub OAuth 客户端。
// httpProxy 为空时直连，否则通过指定的 HTTP 代理访问 GitHub API。
func NewGitHubOAuth(clientID, clientSecret, redirectURI, frontendURL, httpProxy string) *GitHubOAuth {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if httpProxy != "" {
		if proxyURL, err := url.Parse(httpProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &GitHubOAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		FrontendURL:  frontendURL,
		httpClient:   &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}
}

// GetAuthURL 构建 GitHub 授权页面 URL。
// state 参数用于防 CSRF 攻击，前后端各生成一份做比对。
func (g *GitHubOAuth) GetAuthURL(state string) string {
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.RedirectURI)
	q.Set("scope", "user:email") // 请求读取用户邮箱权限
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeCode 用授权码换取 Access Token。
// POST https://github.com/login/oauth/access_token
func (g *GitHubOAuth) ExchangeCode(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", g.ClientID)
	data.Set("client_secret", g.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", g.RedirectURI)

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 GitHub Token 失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 GitHub Token 响应失败: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("GitHub OAuth 错误: %s", result.Error)
	}
	return result.AccessToken, nil
}

// GetFrontendURL 返回配置的前端地址，用于 OAuth 成功后的重定向。
func (g *GitHubOAuth) GetFrontendURL() string {
	return g.FrontendURL
}

// SetHTTPClient 替换底层 HTTP 客户端（用于测试或自定义传输，如代理）。
func (g *GitHubOAuth) SetHTTPClient(c *http.Client) {
	g.httpClient = c
}

// GetUser 用 Access Token 获取 GitHub 用户信息。
// GET https://api.github.com/user (Authorization: Bearer {token})
func (g *GitHubOAuth) GetUser(accessToken string) (*GitHubUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("解析 GitHub 用户信息失败: %w", err)
	}
	return &user, nil
}
