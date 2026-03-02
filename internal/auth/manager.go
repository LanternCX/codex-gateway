package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

var (
	ErrNotLoggedIn        = errors.New("oauth token missing, run auth login")
	ErrTokenRefreshFailed = errors.New("oauth token refresh failed")
)

type RefreshFunc func(context.Context, Token) (Token, error)

type Manager struct {
	store       Store
	refresh     RefreshFunc
	logger      *slog.Logger
	now         func() time.Time
	refreshSkew time.Duration
}

type Option func(*Manager)

func WithNowFunc(fn func() time.Time) Option {
	return func(m *Manager) {
		if fn != nil {
			m.now = fn
		}
	}
}

func WithRefreshSkew(d time.Duration) Option {
	return func(m *Manager) {
		if d > 0 {
			m.refreshSkew = d
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

func NewManager(store Store, refreshFn RefreshFunc, opts ...Option) *Manager {
	m := &Manager{
		store:       store,
		refresh:     refreshFn,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:         time.Now,
		refreshSkew: 60 * time.Second,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	token, err := m.store.Load()
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", ErrNotLoggedIn
		}
		return "", fmt.Errorf("load token: %w", err)
	}

	if token.AccessToken == "" {
		return "", ErrNotLoggedIn
	}

	if !needsRefresh(token, m.now(), m.refreshSkew) {
		return token.AccessToken, nil
	}

	m.logger.DebugContext(ctx, "oauth access token requires refresh", "expires_at", token.ExpiresAt)

	return m.refreshToken(ctx, token)
}

func (m *Manager) ForceRefresh(ctx context.Context) (string, error) {
	token, err := m.store.Load()
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", ErrNotLoggedIn
		}
		return "", fmt.Errorf("load token: %w", err)
	}

	return m.refreshToken(ctx, token)
}

func (m *Manager) refreshToken(ctx context.Context, token Token) (string, error) {
	if token.AccessToken == "" {
		return "", ErrNotLoggedIn
	}

	if token.RefreshToken == "" {
		return "", ErrNotLoggedIn
	}

	if m.refresh == nil {
		return "", fmt.Errorf("%w: refresh function is not configured", ErrTokenRefreshFailed)
	}

	m.logger.InfoContext(ctx, "refreshing oauth token")

	refreshed, err := m.refresh(ctx, token)
	if err != nil {
		m.logger.WarnContext(ctx, "oauth token refresh failed", "error", err)
		return "", fmt.Errorf("%w: %v", ErrTokenRefreshFailed, err)
	}

	if refreshed.AccessToken == "" {
		return "", fmt.Errorf("%w: empty access token", ErrTokenRefreshFailed)
	}

	if refreshed.ExpiresAt.IsZero() {
		refreshed.ExpiresAt = m.now().Add(1 * time.Hour)
	}

	if err := m.store.Save(refreshed); err != nil {
		m.logger.ErrorContext(ctx, "saving refreshed token failed", "error", err)
		return "", fmt.Errorf("save refreshed token: %w", err)
	}

	m.logger.InfoContext(ctx, "oauth token refreshed", "expires_at", refreshed.ExpiresAt)

	return refreshed.AccessToken, nil
}

func needsRefresh(token Token, now time.Time, skew time.Duration) bool {
	if token.ExpiresAt.IsZero() {
		return false
	}

	return !token.ExpiresAt.After(now.Add(skew))
}
