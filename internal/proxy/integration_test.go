package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lu-zhengda/liteoauthllm/internal/auth"
	"github.com/lu-zhengda/liteoauthllm/internal/provider"
)

func TestIntegrationOpenAIProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer openai-test-token" {
			t.Errorf("missing or wrong auth header: %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/backend-api/codex/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"test","choices":[{"message":{"content":"hello"}}]}`)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	store := auth.NewStore(dir)
	store.Write("openai", auth.Token{Version: 1, AccessToken: "openai-test-token", ExpiresAt: 9999999999})

	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)
	srv.SetUpstreamOverride("openai", upstream.URL)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "hello") {
		t.Errorf("expected response to contain 'hello', got %s", body)
	}
}

func TestIntegrationAnthropicProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-ant-oat01-test-token" {
			t.Errorf("missing or wrong Authorization: %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"msg_123","content":[{"type":"text","text":"hello"}]}`)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	store := auth.NewStore(dir)
	store.Write("anthropic", auth.Token{Version: 1, AccessToken: "sk-ant-oat01-test-token"})

	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)
	srv.SetUpstreamOverride("anthropic", upstream.URL)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6-20250514","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "hello") {
		t.Errorf("expected response to contain 'hello', got %s", body)
	}
}

func TestIntegrationModelsRouting(t *testing.T) {
	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"gpt-4o"}]}`)
	}))
	defer openaiUpstream.Close()

	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"claude-sonnet-4-6-20250514"}]}`)
	}))
	defer anthropicUpstream.Close()

	dir := t.TempDir()
	store := auth.NewStore(dir)
	store.Write("openai", auth.Token{Version: 1, AccessToken: "oa-token", ExpiresAt: 9999999999})
	store.Write("anthropic", auth.Token{Version: 1, AccessToken: "ant-token"})

	reg := provider.NewRegistry()
	srv := NewServer(reg, store, false)
	srv.SetUpstreamOverride("openai", openaiUpstream.URL)
	srv.SetUpstreamOverride("anthropic", anthropicUpstream.URL)

	t.Run("default routes to openai", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), "gpt-4o") {
			t.Errorf("expected OpenAI models, got %s", w.Body.String())
		}
	})

	t.Run("anthropic-version header routes to anthropic", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("anthropic-version", "2023-06-01")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), "claude") {
			t.Errorf("expected Anthropic models, got %s", w.Body.String())
		}
	})
}
