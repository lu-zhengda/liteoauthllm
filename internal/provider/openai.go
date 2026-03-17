package provider

import (
	"net/http"
	"strings"
)

// OpenAI forwards requests to chatgpt.com/backend-api using OAuth Bearer token.
type OpenAI struct{}

// NewOpenAI returns a new OpenAI provider.
func NewOpenAI() *OpenAI {
	return &OpenAI{}
}

func (o *OpenAI) Name() string          { return "openai" }
func (o *OpenAI) UpstreamHost() string   { return "chatgpt.com" }
func (o *OpenAI) UpstreamScheme() string { return "https" }

// RewritePath maps standard OpenAI API paths to the ChatGPT backend Codex paths.
// /v1/responses → /backend-api/codex/responses
// /v1/chat/completions → /backend-api/codex/chat/completions
func (o *OpenAI) RewritePath(path string) string {
	return "/backend-api/codex" + strings.TrimPrefix(path, "/v1")
}

// InjectHeaders sets the Authorization header to "Bearer <token>".
// Mutation is intentional: the proxy clones the request before calling this.
func (o *OpenAI) InjectHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}
