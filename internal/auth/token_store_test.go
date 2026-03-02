package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStore_SaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oauth-token.json")
	store := NewFileStore(path)

	expiresAt := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	want := Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Scope:        "openid",
		ExpiresAt:    expiresAt,
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("save token: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load token: %v", err)
	}

	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("unexpected token: %+v", got)
	}

	if !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expiry: got %v want %v", got.ExpiresAt, expiresAt)
	}
}

func TestFileStore_LoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oauth-token.json")
	store := NewFileStore(path)

	_, err := store.Load()
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}
