package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"codex-gateway/internal/auth"
)

func WithOpenURLFunc(fn func(string) error) Option {
	return func(c *Client) {
		if fn != nil {
			c.openURL = fn
		}
	}
}

func WithListenFunc(fn func(network, address string) (net.Listener, error)) Option {
	return func(c *Client) {
		if fn != nil {
			c.listen = fn
		}
	}
}

func WithCallbackTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func (c *Client) AuthenticateWithCallback(ctx context.Context, out io.Writer) (auth.Token, error) {
	pkce, err := generatePKCE()
	if err != nil {
		return auth.Token{}, fmt.Errorf("generate pkce: %w", err)
	}

	state, err := generateState()
	if err != nil {
		return auth.Token{}, fmt.Errorf("generate oauth state: %w", err)
	}

	redirectPath := c.cfg.RedirectPath
	if redirectPath == "" {
		redirectPath = "/auth/callback"
	}
	if !strings.HasPrefix(redirectPath, "/") {
		redirectPath = "/" + redirectPath
	}

	redirectHost := c.cfg.RedirectHost
	if redirectHost == "" {
		redirectHost = "localhost"
	}

	listenAddr := net.JoinHostPort(redirectHost, strconv.Itoa(c.cfg.RedirectPort))
	ln, err := c.listen("tcp", listenAddr)
	if err != nil {
		return auth.Token{}, fmt.Errorf("start oauth callback listener: %w", err)
	}
	defer ln.Close()

	actualPort, err := listenerPort(ln.Addr())
	if err != nil {
		return auth.Token{}, err
	}

	urlHost := redirectHost
	if urlHost == "0.0.0.0" || urlHost == "::" {
		urlHost = "localhost"
	}

	redirectURI := fmt.Sprintf("http://%s:%d%s", urlHost, actualPort, redirectPath)
	authorizeURL := buildAuthorizeURL(c.cfg, redirectURI, pkce, state)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	var once sync.Once

	sendErr := func(err error) {
		once.Do(func() {
			errCh <- err
		})
	}

	sendCode := func(code string) {
		once.Do(func() {
			codeCh <- code
		})
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != redirectPath {
				http.NotFound(w, r)
				return
			}

			if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
				desc := r.URL.Query().Get("error_description")
				if desc == "" {
					desc = oauthErr
				}
				sendErr(fmt.Errorf("oauth callback error: %s", desc))
				writeCallbackHTML(w, false, desc)
				return
			}

			if r.URL.Query().Get("state") != state {
				sendErr(fmt.Errorf("invalid oauth callback state"))
				writeCallbackHTML(w, false, "Invalid state parameter")
				return
			}

			code := strings.TrimSpace(r.URL.Query().Get("code"))
			if code == "" {
				sendErr(fmt.Errorf("missing oauth authorization code"))
				writeCallbackHTML(w, false, "Missing authorization code")
				return
			}

			sendCode(code)
			writeCallbackHTML(w, true, "")
		}),
	}

	serveErrCh := make(chan error, 1)
	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
		}
		close(serveErrCh)
	}()

	if out != nil {
		fmt.Fprintf(out, "Open this URL and complete login: %s\n", authorizeURL)
		fmt.Fprintf(out, "Waiting for OAuth callback at %s\n", redirectURI)
	}

	if err := c.openURL(authorizeURL); err != nil && out != nil {
		fmt.Fprintf(out, "Could not open browser automatically: %v\n", err)
		fmt.Fprintln(out, "Please open the URL manually.")
	}

	timer := time.NewTimer(c.timeout)
	defer timer.Stop()

	var authCode string
	select {
	case authCode = <-codeCh:
	case err := <-errCh:
		_ = shutdownServer(server)
		return auth.Token{}, err
	case err := <-serveErrCh:
		if err != nil {
			return auth.Token{}, fmt.Errorf("oauth callback server failed: %w", err)
		}
		return auth.Token{}, fmt.Errorf("oauth callback server stopped unexpectedly")
	case <-timer.C:
		_ = shutdownServer(server)
		return auth.Token{}, fmt.Errorf("oauth callback timeout")
	case <-ctx.Done():
		_ = shutdownServer(server)
		return auth.Token{}, ctx.Err()
	}

	if err := shutdownServer(server); err != nil {
		return auth.Token{}, err
	}

	return c.exchangeAuthorizationCode(ctx, authCode, redirectURI, pkce.Verifier)
}

func listenerPort(addr net.Addr) (int, error) {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type %T", addr)
	}
	return tcp.Port, nil
}

func shutdownServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown oauth callback server: %w", err)
	}

	return nil
}

type pkceCodes struct {
	Verifier  string
	Challenge string
}

func generatePKCE() (pkceCodes, error) {
	verifier, err := randomBase64URL(32)
	if err != nil {
		return pkceCodes{}, err
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return pkceCodes{Verifier: verifier, Challenge: challenge}, nil
}

func generateState() (string, error) {
	return randomBase64URL(32)
}

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func buildAuthorizeURL(cfg Config, redirectURI string, pkce pkceCodes, state string) string {
	endpoint := strings.TrimSpace(cfg.AuthorizeEndpoint)
	if endpoint == "" {
		endpoint = "https://auth.openai.com/oauth/authorize"
	}

	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		clientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email", "offline_access"}
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	originator := strings.TrimSpace(cfg.Originator)
	if originator == "" {
		originator = "opencode"
	}
	params.Set("originator", originator)

	return endpoint + "?" + params.Encode()
}

func (c *Client) exchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (auth.Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", c.cfg.ClientID)
	form.Set("code_verifier", codeVerifier)
	if c.cfg.ClientSecret != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.Token{}, fmt.Errorf("create token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return auth.Token{}, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.Token{}, fmt.Errorf("read token exchange response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		tErr := parseTokenError(b)
		if tErr.Code != "" {
			return auth.Token{}, fmt.Errorf("token exchange failed: %s", tErr.Code)
		}
		return auth.Token{}, fmt.Errorf("token exchange failed: status %d", resp.StatusCode)
	}

	token := parseOAuthToken(b, c.now())
	if token.AccessToken == "" {
		return auth.Token{}, fmt.Errorf("token exchange returned empty access token")
	}

	return token, nil
}

func writeCallbackHTML(w http.ResponseWriter, success bool, reason string) {
	w.Header().Set("Content-Type", "text/html")
	if success {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><body><h2>Authorization Successful</h2><p>You can close this window and return to codex-gateway.</p></body></html>`))
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("<!doctype html><html><body><h2>Authorization Failed</h2><p>" + templateEscape(reason) + "</p></body></html>"))
}

func templateEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func openURLDefault(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return fmt.Errorf("unsupported platform for auto-open: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}

	return nil
}
