package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const testAnthropicClaudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."

func TestResolveUnambiguousRoutes(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/chat/completions", "openai"},
		{"/v1/responses", "openai"},
		{"/v1/messages", "anthropic"},
		{"/health", "health"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "http://localhost"+tt.path, nil)
			name, _, err := reg.Resolve(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.expected {
				t.Errorf("path %s: expected provider %q, got %q", tt.path, tt.expected, name)
			}
		})
	}
}

func TestResolveModelsAmbiguous(t *testing.T) {
	reg := NewRegistry()

	t.Run("with anthropic-version header routes to anthropic", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://localhost/v1/models", nil)
		req.Header.Set("anthropic-version", "2023-06-01")
		name, _, _ := reg.Resolve(req)
		if name != "anthropic" {
			t.Errorf("expected anthropic, got %s", name)
		}
	})

	t.Run("without anthropic-version header routes to openai", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://localhost/v1/models", nil)
		name, _, _ := reg.Resolve(req)
		if name != "openai" {
			t.Errorf("expected openai, got %s", name)
		}
	})
}

func TestResolveUnknownPath(t *testing.T) {
	reg := NewRegistry()
	req, _ := http.NewRequest("GET", "http://localhost/v1/unknown", nil)
	_, _, err := reg.Resolve(req)
	if err == nil {
		t.Error("expected error for unknown path")
	}
}

func TestOpenAIUpstream(t *testing.T) {
	p := NewOpenAI()
	if p.Name() != "openai" {
		t.Errorf("expected name openai, got %s", p.Name())
	}
	if p.UpstreamHost() != "chatgpt.com" {
		t.Errorf("expected chatgpt.com, got %s", p.UpstreamHost())
	}
}

func TestAnthropicUpstream(t *testing.T) {
	p := NewAnthropic()
	if p.Name() != "anthropic" {
		t.Errorf("expected name anthropic, got %s", p.Name())
	}
	if p.UpstreamHost() != "api.anthropic.com" {
		t.Errorf("expected api.anthropic.com, got %s", p.UpstreamHost())
	}
}

func TestOpenAIInjectHeaders(t *testing.T) {
	p := NewOpenAI()
	req, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", nil)
	p.InjectHeaders(req, "test-token-123")

	auth := req.Header.Get("Authorization")
	if auth != "Bearer test-token-123" {
		t.Errorf("expected Bearer token, got %s", auth)
	}
}

func TestAnthropicInjectHeaders(t *testing.T) {
	p := NewAnthropic()
	req, _ := http.NewRequest("POST", "http://localhost/v1/messages", nil)
	p.InjectHeaders(req, "sk-ant-oat01-test-token")

	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer sk-ant-oat01-test-token" {
		t.Errorf("expected Bearer token, got %s", authHeader)
	}

	// Setup-tokens require the oauth beta header
	beta := req.Header.Get("anthropic-beta")
	if !strings.Contains(beta, "oauth-2025-04-20") {
		t.Errorf("expected oauth beta header, got %s", beta)
	}
}

func TestAnthropicFiltersContextOneMBeta(t *testing.T) {
	p := NewAnthropic()
	req, _ := http.NewRequest("POST", "http://localhost/v1/messages", nil)
	req.Header.Set("anthropic-beta", "context-1m-2025-08-07,some-other-beta")
	p.InjectHeaders(req, "sk-ant-oat01-test-token")

	beta := req.Header.Get("anthropic-beta")
	if strings.Contains(beta, "context-1m") {
		t.Errorf("expected context-1m to be filtered out, got %s", beta)
	}
	if !strings.Contains(beta, "some-other-beta") {
		t.Errorf("expected some-other-beta to be preserved, got %s", beta)
	}
	if !strings.Contains(beta, "oauth-2025-04-20") {
		t.Errorf("expected required oauth beta to be present, got %s", beta)
	}
}

func TestAnthropicInjectHeadersPreservesExistingVersion(t *testing.T) {
	p := NewAnthropic()
	req, _ := http.NewRequest("POST", "http://localhost/v1/messages", nil)
	req.Header.Set("anthropic-version", "2024-01-01")
	p.InjectHeaders(req, "sk-ant-oat01-test-token")

	version := req.Header.Get("anthropic-version")
	if version != "2024-01-01" {
		t.Errorf("expected existing version preserved, got %s", version)
	}
}

func TestAnthropicInjectHeadersAddsClaudeCodeSystemIdentityWhenMissing(t *testing.T) {
	p := NewAnthropic()
	req, _ := http.NewRequest(
		"POST",
		"http://localhost/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	p.InjectHeaders(req, "sk-ant-oat01-test-token")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("reading rewritten body: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decoding rewritten body: %v", err)
	}

	system, ok := payload["system"].(string)
	if !ok {
		t.Fatalf("expected system string, got %#v", payload["system"])
	}
	if system != testAnthropicClaudeCodeIdentity {
		t.Fatalf("expected Claude Code identity %q, got %q", testAnthropicClaudeCodeIdentity, system)
	}
}

func TestAnthropicInjectHeadersPrependsClaudeCodeSystemIdentity(t *testing.T) {
	p := NewAnthropic()
	req, _ := http.NewRequest(
		"POST",
		"http://localhost/v1/messages",
		strings.NewReader(`{"model":"claude-sonnet-4-6","system":"You are helpful.","messages":[{"role":"user","content":"hi"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	p.InjectHeaders(req, "sk-ant-oat01-test-token")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("reading rewritten body: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decoding rewritten body: %v", err)
	}

	system, ok := payload["system"].([]interface{})
	if !ok {
		t.Fatalf("expected system array, got %#v", payload["system"])
	}
	if len(system) != 2 {
		t.Fatalf("expected 2 system blocks, got %d", len(system))
	}

	first, ok := system[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first system block object, got %#v", system[0])
	}
	if first["type"] != "text" || first["text"] != testAnthropicClaudeCodeIdentity {
		t.Fatalf("expected first system block to be Claude Code identity, got %#v", first)
	}

	second, ok := system[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected second system block object, got %#v", system[1])
	}
	if second["type"] != "text" || second["text"] != "You are helpful." {
		t.Fatalf("expected second system block to preserve user system, got %#v", second)
	}
}

func TestUpstreamScheme(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
	}{
		{"openai", NewOpenAI()},
		{"anthropic", NewAnthropic()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := tt.provider.UpstreamScheme()
			if scheme != "https" {
				t.Errorf("expected https, got %s", scheme)
			}
		})
	}
}

func TestRegistryGet(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		name     string
		expected Provider
		isNil    bool
	}{
		{"openai", NewOpenAI(), false},
		{"anthropic", NewAnthropic(), false},
		{"unknown", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := reg.Get(tt.name)
			if tt.isNil {
				if p != nil {
					t.Errorf("expected nil for %q, got %v", tt.name, p)
				}
			} else {
				if p == nil {
					t.Errorf("expected provider for %q, got nil", tt.name)
				}
				if p.Name() != tt.name {
					t.Errorf("expected name %q, got %q", tt.name, p.Name())
				}
			}
		})
	}
}
