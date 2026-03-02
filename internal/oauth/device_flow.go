package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codex-gateway/internal/auth"
)

type Config struct {
	ClientID                    string
	ClientSecret                string
	AuthorizeEndpoint           string
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
	RedirectHost                string
	RedirectPort                int
	RedirectPath                string
	Originator                  string
	Scopes                      []string
	Audience                    string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithNowFunc(now func() time.Time) Option {
	return func(c *Client) {
		if now != nil {
			c.now = now
		}
	}
}

func WithSleepFunc(sleep func(time.Duration)) Option {
	return func(c *Client) {
		if sleep != nil {
			c.sleep = sleep
		}
	}
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	now        func() time.Time
	sleep      func(time.Duration)
	listen     func(network, address string) (net.Listener, error)
	openURL    func(string) error
	timeout    time.Duration
}

func NewClient(cfg Config, opts ...Option) *Client {
	c := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		now:     time.Now,
		sleep:   time.Sleep,
		listen:  net.Listen,
		openURL: openURLDefault,
		timeout: 5 * time.Minute,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Authenticate(ctx context.Context, out io.Writer) (auth.Token, error) {
	deviceCode, err := c.requestDeviceCode(ctx)
	if err != nil {
		return auth.Token{}, err
	}

	if out != nil {
		fmt.Fprintf(out, "Open this URL and complete login: %s\n", deviceCode.VerificationURI)
		if deviceCode.VerificationURIComplete != "" {
			fmt.Fprintf(out, "Direct URL: %s\n", deviceCode.VerificationURIComplete)
		}
		fmt.Fprintf(out, "User code: %s\n", deviceCode.UserCode)
	}

	interval := time.Duration(deviceCode.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	expiresAt := c.now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		token, pollErr := c.pollToken(ctx, deviceCode.DeviceCode)
		if pollErr == nil {
			return token, nil
		}

		switch pollErr.Code {
		case "authorization_pending":
			if !expiresAt.IsZero() && !c.now().Before(expiresAt) {
				return auth.Token{}, fmt.Errorf("oauth device flow timed out")
			}
			c.sleep(interval)
		case "slow_down":
			interval += time.Second
			c.sleep(interval)
		case "access_denied":
			return auth.Token{}, fmt.Errorf("access denied during oauth login")
		case "expired_token":
			return auth.Token{}, fmt.Errorf("oauth device code expired")
		default:
			if pollErr.Description != "" {
				return auth.Token{}, fmt.Errorf("oauth token polling failed: %s (%s)", pollErr.Code, pollErr.Description)
			}
			return auth.Token{}, fmt.Errorf("oauth token polling failed: %s", pollErr.Code)
		}
	}
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (auth.Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", c.cfg.ClientID)
	if c.cfg.ClientSecret != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.Token{}, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return auth.Token{}, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.Token{}, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errResp := parseTokenError(b)
		if errResp.Code != "" {
			return auth.Token{}, fmt.Errorf("refresh failed: %s", errResp.Code)
		}
		return auth.Token{}, fmt.Errorf("refresh failed: status %d", resp.StatusCode)
	}

	return parseOAuthToken(b, c.now()), nil
}

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

type tokenErrorResponse struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (c *Client) requestDeviceCode(ctx context.Context) (deviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	if len(c.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	if c.cfg.Audience != "" {
		form.Set("audience", c.cfg.Audience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.DeviceAuthorizationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return deviceCodeResponse{}, fmt.Errorf("create device authorization request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return deviceCodeResponse{}, fmt.Errorf("device authorization request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return deviceCodeResponse{}, fmt.Errorf("read device authorization response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return deviceCodeResponse{}, fmt.Errorf("device authorization failed: status %d", resp.StatusCode)
	}

	var code deviceCodeResponse
	if err := json.Unmarshal(b, &code); err != nil {
		return deviceCodeResponse{}, fmt.Errorf("parse device authorization response: %w", err)
	}

	if code.DeviceCode == "" || code.UserCode == "" || code.VerificationURI == "" {
		return deviceCodeResponse{}, fmt.Errorf("device authorization response missing required fields")
	}

	return code, nil
}

func (c *Client) pollToken(ctx context.Context, deviceCode string) (auth.Token, *tokenErrorResponse) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", deviceCode)
	form.Set("client_id", c.cfg.ClientID)
	if c.cfg.ClientSecret != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.Token{}, &tokenErrorResponse{Code: "request_creation_failed", Description: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return auth.Token{}, &tokenErrorResponse{Code: "request_failed", Description: err.Error()}
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.Token{}, &tokenErrorResponse{Code: "read_failed", Description: err.Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		tErr := parseTokenError(b)
		if tErr.Code == "" {
			tErr.Code = fmt.Sprintf("status_%d", resp.StatusCode)
		}
		return auth.Token{}, &tErr
	}

	return parseOAuthToken(b, c.now()), nil
}

func parseTokenError(b []byte) tokenErrorResponse {
	var out tokenErrorResponse
	_ = json.Unmarshal(b, &out)
	return out
}

func parseOAuthToken(b []byte, now time.Time) auth.Token {
	var tr tokenResponse
	_ = json.Unmarshal(b, &tr)

	expiresAt := time.Time{}
	if tr.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return auth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scope:        tr.Scope,
		ExpiresAt:    expiresAt,
	}
}
