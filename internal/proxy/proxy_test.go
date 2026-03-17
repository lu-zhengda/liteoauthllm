package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lu-zhengda/liteoauthllm/internal/auth"
	"github.com/lu-zhengda/liteoauthllm/internal/provider"
)

func TestHealthEndpoint(t *testing.T) {
	store := auth.NewStore(t.TempDir())
	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	store := auth.NewStore(t.TempDir())
	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)

	req := httptest.NewRequest("GET", "/v1/unknown", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestMissingTokenReturnsOpenAI401(t *testing.T) {
	store := auth.NewStore(t.TempDir())
	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in OpenAI format")
	}
	if _, ok := errObj["message"]; !ok {
		t.Error("expected message in error")
	}
}

func TestMissingTokenReturnsAnthropic401(t *testing.T) {
	store := auth.NewStore(t.TempDir())
	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("anthropic-version", "2023-06-01")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["type"] != "error" {
		t.Errorf("expected type 'error' in Anthropic format, got %v", resp["type"])
	}
}

func TestProxyForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			t.Errorf("expected Bearer token, got %s", authHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"id":"chatcmpl-123","choices":[]}`)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	store := auth.NewStore(dir)
	store.Write("openai", auth.Token{
		Version:     1,
		AccessToken: "test-access-token",
		ExpiresAt:   9999999999,
	})

	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)
	srv.SetUpstreamOverride("openai", upstream.URL)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
