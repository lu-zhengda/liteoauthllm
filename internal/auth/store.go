package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Token holds the OAuth credentials for a single provider.
type Token struct {
	Version      int    `json:"version"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

// Store manages per-provider token files on disk.
type Store struct {
	dir string
}

// NewStore returns a Store that persists tokens under dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Read loads and parses the token file for the given provider.
// Returns an error if the file does not exist or is malformed.
func (s *Store) Read(provider string) (Token, error) {
	path := s.path(provider)
	data, err := os.ReadFile(path)
	if err != nil {
		return Token{}, fmt.Errorf("reading token for %s: %w", provider, err)
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return Token{}, fmt.Errorf("parsing token for %s: %w", provider, err)
	}

	return token, nil
}

// Write serialises token to disk using an atomic temp-file rename.
// The file is written with 0600 permissions; the directory is created
// (mode 0700) if it does not already exist.
func (s *Store) Write(provider string, token Token) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}

	tmpPath := s.path(provider) + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp token file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path(provider)); err != nil {
		os.Remove(tmpPath) // best-effort cleanup; ignore secondary error
		return fmt.Errorf("renaming token file: %w", err)
	}

	return nil
}

// Delete removes the token file for the given provider.
// Returns an error if no token file exists.
func (s *Store) Delete(provider string) error {
	path := s.path(provider)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("no token found for %s", provider)
	}
	return os.Remove(path)
}

// List returns the provider names for which a token file exists.
// Returns nil if the directory cannot be read.
func (s *Store) List() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}

	var providers []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		providers = append(providers, name)
	}
	return providers
}

// Dir returns the directory where token files are stored.
// Used by Task 7 (token refresh with file lock).
func (s *Store) Dir() string {
	return s.dir
}

// path builds the full file path for a provider's token file.
func (s *Store) path(provider string) string {
	return filepath.Join(s.dir, provider+".json")
}
