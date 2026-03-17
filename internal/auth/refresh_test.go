package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		token    Token
		expected bool
	}{
		{
			name:     "no expiry (setup-token)",
			token:    Token{ExpiresAt: 0},
			expected: false,
		},
		{
			name:     "not expired",
			token:    Token{ExpiresAt: time.Now().Add(1 * time.Hour).Unix()},
			expected: false,
		},
		{
			name:     "expired",
			token:    Token{ExpiresAt: time.Now().Add(-1 * time.Hour).Unix()},
			expected: true,
		},
		{
			name:     "within 30s safety margin",
			token:    Token{ExpiresAt: time.Now().Add(20 * time.Second).Unix()},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsRefresh(tt.token)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old-refresh-token" {
			t.Errorf("expected old refresh token, got %s", r.FormValue("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer mockServer.Close()

	dir := t.TempDir()
	store := NewStore(dir)
	store.Write("openai", Token{
		Version:      1,
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour).Unix(),
	})

	err := RefreshOpenAIToken(store, dir, mockServer.URL, "test-client-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := store.Read("openai")
	if err != nil {
		t.Fatal(err)
	}

	if updated.AccessToken != "new-access-token" {
		t.Errorf("expected new access token, got %s", updated.AccessToken)
	}
	if updated.RefreshToken != "new-refresh-token" {
		t.Errorf("expected new refresh token, got %s", updated.RefreshToken)
	}
}
