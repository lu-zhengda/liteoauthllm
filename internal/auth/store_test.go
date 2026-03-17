package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadToken(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	token := Token{
		Version:      1,
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    1710000000,
	}

	if err := store.Write("openai", token); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got, err := store.Read("openai")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if got.AccessToken != token.AccessToken {
		t.Errorf("expected access token %q, got %q", token.AccessToken, got.AccessToken)
	}
	if got.RefreshToken != token.RefreshToken {
		t.Errorf("expected refresh token %q, got %q", token.RefreshToken, got.RefreshToken)
	}
	if got.ExpiresAt != token.ExpiresAt {
		t.Errorf("expected expires_at %d, got %d", token.ExpiresAt, got.ExpiresAt)
	}
}

func TestReadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Read("openai")
	if err == nil {
		t.Error("expected error reading nonexistent token")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	token := Token{Version: 1, AccessToken: "test"}
	if err := store.Write("openai", token); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("openai"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	_, err := store.Read("openai")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	err := store.Delete("openai")
	if err == nil {
		t.Error("expected error deleting nonexistent token")
	}
}

func TestWriteCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "tokens")
	store := NewStore(dir)

	token := Token{Version: 1, AccessToken: "test"}
	if err := store.Write("openai", token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	token := Token{Version: 1, AccessToken: "secret"}
	if err := store.Write("openai", token); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "openai.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

func TestListProviders(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Write("openai", Token{Version: 1, AccessToken: "a"})
	store.Write("anthropic", Token{Version: 1, AccessToken: "b"})

	providers := store.List()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}
