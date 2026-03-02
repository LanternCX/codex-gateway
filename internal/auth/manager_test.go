package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManager_AccessTokenUsesCachedToken(t *testing.T) {
	now := time.Now().UTC()
	store := &stubStore{token: Token{AccessToken: "cached", ExpiresAt: now.Add(10 * time.Minute)}}
	mgr := NewManager(store, nil, WithNowFunc(func() time.Time { return now }), WithRefreshSkew(2*time.Minute))

	got, err := mgr.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("access token: %v", err)
	}

	if got != "cached" {
		t.Fatalf("unexpected token: %q", got)
	}

	if store.saveCalls != 0 {
		t.Fatalf("expected no save call, got %d", store.saveCalls)
	}
}

func TestManager_AccessTokenRefreshesExpiredToken(t *testing.T) {
	now := time.Now().UTC()
	store := &stubStore{token: Token{AccessToken: "expired", RefreshToken: "refresh", ExpiresAt: now.Add(10 * time.Second)}}
	refreshCalled := false

	mgr := NewManager(
		store,
		func(ctx context.Context, in Token) (Token, error) {
			refreshCalled = true
			if in.RefreshToken != "refresh" {
				t.Fatalf("unexpected refresh token: %q", in.RefreshToken)
			}
			return Token{AccessToken: "fresh", RefreshToken: "refresh2", ExpiresAt: now.Add(1 * time.Hour)}, nil
		},
		WithNowFunc(func() time.Time { return now }),
		WithRefreshSkew(1*time.Minute),
	)

	got, err := mgr.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("access token: %v", err)
	}

	if got != "fresh" {
		t.Fatalf("unexpected token: %q", got)
	}

	if !refreshCalled {
		t.Fatal("expected refresh to be called")
	}

	if store.saveCalls != 1 {
		t.Fatalf("expected one save call, got %d", store.saveCalls)
	}
}

func TestManager_AccessTokenRefreshFailure(t *testing.T) {
	now := time.Now().UTC()
	store := &stubStore{token: Token{AccessToken: "expired", RefreshToken: "refresh", ExpiresAt: now.Add(10 * time.Second)}}

	mgr := NewManager(
		store,
		func(ctx context.Context, in Token) (Token, error) {
			return Token{}, errors.New("boom")
		},
		WithNowFunc(func() time.Time { return now }),
		WithRefreshSkew(1*time.Minute),
	)

	_, err := mgr.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected refresh error")
	}

	if !errors.Is(err, ErrTokenRefreshFailed) {
		t.Fatalf("expected ErrTokenRefreshFailed, got %v", err)
	}
}

type stubStore struct {
	token     Token
	err       error
	saveCalls int
}

func (s *stubStore) Load() (Token, error) {
	if s.err != nil {
		return Token{}, s.err
	}
	return s.token, nil
}

func (s *stubStore) Save(token Token) error {
	s.saveCalls++
	s.token = token
	return nil
}
