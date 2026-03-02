package oauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBuildAuthorizeURL_DefaultOriginator(t *testing.T) {
	raw := buildAuthorizeURL(
		Config{
			ClientID:          "app_EMoamEEZ73f0CkXaXp7hrann",
			AuthorizeEndpoint: "https://auth.openai.com/oauth/authorize",
			Scopes:            []string{"openid", "profile", "email", "offline_access"},
		},
		"http://localhost:1455/auth/callback",
		pkceCodes{Verifier: "v", Challenge: "challenge"},
		"state-123",
	)

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}

	if got := u.Query().Get("originator"); got != "opencode" {
		t.Fatalf("expected originator opencode, got %q", got)
	}
}

func TestClient_AuthenticateWithCallbackSuccess(t *testing.T) {
	var tokenCalls int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected token path: %s", r.URL.Path)
		}

		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected content-type: %q", got)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if got := r.Form.Get("grant_type"); got != "authorization_code" {
			t.Fatalf("unexpected grant_type: %q", got)
		}

		if got := r.Form.Get("code"); got != "auth-code" {
			t.Fatalf("unexpected code: %q", got)
		}

		if got := r.Form.Get("code_verifier"); got == "" {
			t.Fatal("expected code_verifier to be present")
		}

		atomic.AddInt32(&tokenCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-123","refresh_token":"refresh-456","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	client := NewClient(Config{
		ClientID:          "client-id",
		AuthorizeEndpoint: "https://auth.example.com/oauth/authorize",
		TokenEndpoint:     tokenSrv.URL + "/oauth/token",
		RedirectHost:      "127.0.0.1",
		RedirectPort:      0,
		RedirectPath:      "/auth/callback",
		Scopes:            []string{"openid", "profile"},
	},
		WithHTTPClient(tokenSrv.Client()),
		WithOpenURLFunc(func(rawAuthorizeURL string) error {
			u, err := url.Parse(rawAuthorizeURL)
			if err != nil {
				t.Fatalf("parse authorize url: %v", err)
			}

			q := u.Query()
			redirectURI := q.Get("redirect_uri")
			state := q.Get("state")
			if redirectURI == "" || state == "" {
				t.Fatalf("authorize url missing redirect/state: %s", rawAuthorizeURL)
			}

			go func() {
				_, _ = http.Get(redirectURI + "?code=auth-code&state=" + url.QueryEscape(state))
			}()

			return nil
		}),
	)

	var out strings.Builder
	token, err := client.AuthenticateWithCallback(context.Background(), &out)
	if err != nil {
		t.Fatalf("authenticate with callback: %v", err)
	}

	if token.AccessToken != "access-123" {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}

	if token.RefreshToken != "refresh-456" {
		t.Fatalf("unexpected refresh token: %q", token.RefreshToken)
	}

	if atomic.LoadInt32(&tokenCalls) != 1 {
		t.Fatalf("expected one token call, got %d", tokenCalls)
	}

	if !strings.Contains(out.String(), "Open this URL") {
		t.Fatalf("expected login instructions in output, got: %s", out.String())
	}
}

func TestClient_AuthenticateWithCallbackError(t *testing.T) {
	var tokenCalls int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		http.Error(w, "should not call token endpoint", http.StatusInternalServerError)
	}))
	defer tokenSrv.Close()

	client := NewClient(Config{
		ClientID:          "client-id",
		AuthorizeEndpoint: "https://auth.example.com/oauth/authorize",
		TokenEndpoint:     tokenSrv.URL + "/oauth/token",
		RedirectHost:      "127.0.0.1",
		RedirectPort:      0,
		RedirectPath:      "/auth/callback",
	},
		WithHTTPClient(tokenSrv.Client()),
		WithOpenURLFunc(func(rawAuthorizeURL string) error {
			u, err := url.Parse(rawAuthorizeURL)
			if err != nil {
				t.Fatalf("parse authorize url: %v", err)
			}

			q := u.Query()
			redirectURI := q.Get("redirect_uri")
			state := q.Get("state")
			go func() {
				_, _ = http.Get(redirectURI + "?error=access_denied&error_description=denied&state=" + url.QueryEscape(state))
			}()
			return nil
		}),
	)

	_, err := client.AuthenticateWithCallback(context.Background(), io.Discard)
	if err == nil {
		t.Fatal("expected callback error")
	}

	if !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got: %v", err)
	}

	if atomic.LoadInt32(&tokenCalls) != 0 {
		t.Fatalf("expected no token exchange call, got %d", tokenCalls)
	}
}
