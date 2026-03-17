package provider

import (
	"errors"
	"net/http"
	"strings"
)

// ErrUnknownRoute is returned by Registry.Resolve when no provider matches the request path.
var ErrUnknownRoute = errors.New("unknown route")

// Provider describes the upstream LLM API a request should be forwarded to.
type Provider interface {
	Name() string
	UpstreamHost() string
	UpstreamScheme() string
	// RewritePath transforms the client request path to the upstream path.
	// For example, OpenAI rewrites /v1/responses to /backend-api/codex/responses.
	RewritePath(path string) string
	// InjectHeaders mutates req in-place to add auth and required API headers.
	// Mutation is intentional: this follows the standard Go reverse-proxy pattern
	// where the request is cloned by the proxy before being forwarded.
	InjectHeaders(req *http.Request, token string)
}

// Registry maps incoming request paths to the correct Provider.
type Registry struct {
	openai    Provider
	anthropic Provider
}

// NewRegistry returns a Registry pre-populated with the built-in providers.
func NewRegistry() *Registry {
	return &Registry{
		openai:    NewOpenAI(),
		anthropic: NewAnthropic(),
	}
}

// Resolve returns the provider name, the Provider, and any error.
// The special name "health" is returned for /health with a nil Provider.
// ErrUnknownRoute is returned when no rule matches.
func (r *Registry) Resolve(req *http.Request) (string, Provider, error) {
	path := req.URL.Path

	switch {
	case path == "/health":
		return "health", nil, nil
	case strings.HasPrefix(path, "/v1/chat/completions"):
		return "openai", r.openai, nil
	case strings.HasPrefix(path, "/v1/responses"):
		return "openai", r.openai, nil
	case strings.HasPrefix(path, "/v1/messages"):
		return "anthropic", r.anthropic, nil
	case strings.HasPrefix(path, "/v1/models"):
		// /v1/models is ambiguous: Anthropic clients set the anthropic-version header.
		if req.Header.Get("anthropic-version") != "" {
			return "anthropic", r.anthropic, nil
		}
		return "openai", r.openai, nil
	default:
		return "", nil, ErrUnknownRoute
	}
}

// Get returns the named provider, or nil if unknown.
func (r *Registry) Get(name string) Provider {
	switch name {
	case "openai":
		return r.openai
	case "anthropic":
		return r.anthropic
	default:
		return nil
	}
}
