package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const refreshSafetyMargin = 30 * time.Second

// NeedsRefresh returns true when the token is expired or will expire within
// the safety margin. A zero ExpiresAt (setup-token / no expiry) is treated as
// never needing a refresh.
func NeedsRefresh(token Token) bool {
	if token.ExpiresAt == 0 {
		return false
	}
	expiresAt := time.Unix(token.ExpiresAt, 0)
	return time.Until(expiresAt) < refreshSafetyMargin
}

// RefreshOpenAIToken acquires an exclusive file lock, re-checks whether the
// token still needs refreshing (a concurrent goroutine may have already done
// it), and if so exchanges the stored refresh_token for a new token pair.
func RefreshOpenAIToken(store *Store, lockDir, tokenURL, clientID string) error {
	lockPath := filepath.Join(lockDir, "openai.lock")

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("creating lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) //nolint:errcheck

	// Re-read token under lock — another goroutine may have refreshed already.
	token, err := store.Read("openai")
	if err != nil {
		return fmt.Errorf("reading token: %w", err)
	}

	if !NeedsRefresh(token) {
		return nil // Already refreshed by another goroutine.
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {token.RefreshToken},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("parsing refresh response: %w", err)
	}

	newToken := Token{
		Version:      1,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix(),
	}

	return store.Write("openai", newToken)
}
