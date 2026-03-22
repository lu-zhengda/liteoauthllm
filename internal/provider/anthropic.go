package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	anthropicDefaultVersion = "2023-06-01"
	// anthropicOAuthBeta is required when authenticating with setup-tokens via Bearer auth.
	// Without this header, the API returns "OAuth authentication is currently not supported."
	anthropicOAuthBeta = "oauth-2025-04-20"
	// Claude Code OAuth requests must include this identity in the system prompt.
	// Without it, newer Sonnet/Opus models return a generic invalid_request_error.
	anthropicClaudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."
)

// Anthropic forwards requests to api.anthropic.com using Bearer auth with setup-tokens.
type Anthropic struct{}

// NewAnthropic returns a new Anthropic provider.
func NewAnthropic() *Anthropic {
	return &Anthropic{}
}

func (a *Anthropic) Name() string           { return "anthropic" }
func (a *Anthropic) UpstreamHost() string   { return "api.anthropic.com" }
func (a *Anthropic) UpstreamScheme() string { return "https" }

// RewritePath returns the path as-is — Anthropic API paths don't need rewriting.
func (a *Anthropic) RewritePath(path string) string { return path }

// InjectHeaders sets required Anthropic API headers for setup-token auth:
//   - Authorization: Bearer <setup-token>
//   - anthropic-version: defaults to 2023-06-01 if not already set
//   - anthropic-beta: injects oauth-2025-04-20 (required for Bearer auth) and
//     filters out context-1m-2025-08-07 (rejected with OAuth tokens)
//
// Mutation is intentional: follows the standard Go reverse proxy pattern.
func (a *Anthropic) InjectHeaders(req *http.Request, token string) {
	// Delete any client-sent x-api-key (SDK sends a dummy key) since setup-tokens
	// are rejected via x-api-key — only Bearer auth works for them.
	req.Header.Del("x-api-key")
	// Setup-tokens require Bearer auth + the oauth beta header
	req.Header.Set("Authorization", "Bearer "+token)

	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", anthropicDefaultVersion)
	}

	// Merge the required oauth beta with any client-sent betas,
	// filtering out disallowed values like context-1m
	existingBeta := req.Header.Get("anthropic-beta")
	merged := mergeAnthropicBetas(existingBeta, anthropicOAuthBeta)
	req.Header.Set("anthropic-beta", merged)

	injectAnthropicClaudeCodeIdentity(req)
}

// mergeAnthropicBetas combines required betas with existing client-sent betas,
// deduplicating and filtering out disallowed values (context-1m-2025-08-07).
func mergeAnthropicBetas(existing, required string) string {
	disallowed := map[string]bool{"context-1m-2025-08-07": true}
	seen := map[string]bool{}
	var result []string

	for _, b := range splitBetas(required) {
		if !disallowed[b] && !seen[b] {
			seen[b] = true
			result = append(result, b)
		}
	}
	for _, b := range splitBetas(existing) {
		if !disallowed[b] && !seen[b] {
			seen[b] = true
			result = append(result, b)
		}
	}

	return strings.Join(result, ",")
}

// splitBetas splits a comma-separated beta string into trimmed, non-empty tokens.
func splitBetas(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, b := range strings.Split(s, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			out = append(out, b)
		}
	}
	return out
}

func injectAnthropicClaudeCodeIdentity(req *http.Request) {
	if req.Method != http.MethodPost || !strings.HasPrefix(req.URL.Path, "/v1/messages") || req.Body == nil {
		return
	}
	contentType := strings.ToLower(req.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return
	}
	if len(body) == 0 {
		setRequestBody(req, body)
		return
	}

	rewritten, changed := ensureAnthropicClaudeCodeIdentity(body)
	if !changed {
		rewritten = body
	}
	setRequestBody(req, rewritten)
}

func ensureAnthropicClaudeCodeIdentity(body []byte) ([]byte, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}

	system, hasSystem := payload["system"]
	if !hasSystem {
		payload["system"] = anthropicClaudeCodeIdentity
	} else {
		updated, changed := prependAnthropicClaudeCodeIdentity(system)
		if !changed {
			return nil, false
		}
		payload["system"] = updated
	}

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return rewritten, true
}

func prependAnthropicClaudeCodeIdentity(system interface{}) (interface{}, bool) {
	if hasAnthropicClaudeCodeIdentity(system) {
		return nil, false
	}

	switch v := system.(type) {
	case nil:
		return anthropicClaudeCodeIdentity, true
	case string:
		if strings.TrimSpace(v) == "" {
			return anthropicClaudeCodeIdentity, true
		}
		return []interface{}{
			map[string]interface{}{"type": "text", "text": anthropicClaudeCodeIdentity},
			map[string]interface{}{"type": "text", "text": v},
		}, true
	case []interface{}:
		updated := make([]interface{}, 0, len(v)+1)
		updated = append(updated, map[string]interface{}{"type": "text", "text": anthropicClaudeCodeIdentity})
		updated = append(updated, v...)
		return updated, true
	case map[string]interface{}:
		return []interface{}{
			map[string]interface{}{"type": "text", "text": anthropicClaudeCodeIdentity},
			v,
		}, true
	default:
		return nil, false
	}
}

func hasAnthropicClaudeCodeIdentity(system interface{}) bool {
	switch v := system.(type) {
	case string:
		return strings.Contains(v, anthropicClaudeCodeIdentity)
	case []interface{}:
		for _, block := range v {
			if hasAnthropicClaudeCodeIdentity(block) {
				return true
			}
		}
	case map[string]interface{}:
		text, _ := v["text"].(string)
		return text == anthropicClaudeCodeIdentity
	}
	return false
}

func setRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}
