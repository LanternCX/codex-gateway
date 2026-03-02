package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_AuthenticateSuccess(t *testing.T) {
	var pollCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_code":"dev-code","user_code":"ABCD-EFGH","verification_uri":"https://verify.example.com","verification_uri_complete":"https://verify.example.com/complete","expires_in":600,"interval":1}`))
		case "/token":
			if atomic.AddInt32(&pollCount, 1) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-123","refresh_token":"refresh-456","token_type":"Bearer","scope":"openid","expires_in":3600}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	now := time.Now().UTC()
	slept := false
	client := NewClient(Config{
		ClientID:                    "client-id",
		DeviceAuthorizationEndpoint: srv.URL + "/device",
		TokenEndpoint:               srv.URL + "/token",
		Scopes:                      []string{"openid"},
	},
		WithHTTPClient(srv.Client()),
		WithNowFunc(func() time.Time { return now }),
		WithSleepFunc(func(d time.Duration) { slept = true }),
	)

	var out strings.Builder
	token, err := client.Authenticate(context.Background(), &out)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if token.AccessToken != "access-123" {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}

	if !slept {
		t.Fatal("expected polling sleep to be used")
	}

	if !strings.Contains(out.String(), "ABCD-EFGH") {
		t.Fatalf("expected user code in output, got: %s", out.String())
	}
}

func TestClient_AuthenticateTerminalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_code":"dev-code","user_code":"ABCD-EFGH","verification_uri":"https://verify.example.com","expires_in":600,"interval":1}`))
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"access_denied"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(Config{
		ClientID:                    "client-id",
		DeviceAuthorizationEndpoint: srv.URL + "/device",
		TokenEndpoint:               srv.URL + "/token",
	},
		WithHTTPClient(srv.Client()),
		WithSleepFunc(func(d time.Duration) {}),
	)

	_, err := client.Authenticate(context.Background(), &strings.Builder{})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected access denied error, got: %v", err)
	}
}
