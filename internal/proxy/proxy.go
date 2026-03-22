package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/lu-zhengda/liteoauthllm/internal/auth"
	"github.com/lu-zhengda/liteoauthllm/internal/provider"
)

const (
	openaiTokenURL = "https://auth.openai.com/oauth/token"
	openaiClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

// Server is an HTTP reverse proxy that authenticates requests, injects provider
// tokens, and forwards traffic to the appropriate upstream LLM API.
type Server struct {
	registry          *provider.Registry
	store             *auth.Store
	verbose           bool
	upstreamOverrides map[string]string
}

// NewServer returns a Server ready to handle requests. Set verbose to true
// to emit per-request log lines with method, path, provider, latency, and status.
func NewServer(registry *provider.Registry, store *auth.Store, verbose bool) *Server {
	return &Server{
		registry:          registry,
		store:             store,
		verbose:           verbose,
		upstreamOverrides: make(map[string]string),
	}
}

// SetUpstreamOverride allows tests to redirect upstream traffic to a mock server
// instead of the real LLM API endpoint.
func (s *Server) SetUpstreamOverride(providerName, rawURL string) {
	s.upstreamOverrides[providerName] = rawURL
}

// ServeHTTP implements http.Handler. It resolves the incoming request to a
// provider, validates and injects the stored token, then proxies to upstream.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	name, prov, err := s.registry.Resolve(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if name == "health" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
		return
	}

	token, err := s.store.Read(name)
	if err != nil {
		msg := fmt.Sprintf("%s token not found. Run: liteoauthllm login %s", capitalize(name), name)
		writeAuthError(w, name, msg)
		return
	}

	if auth.NeedsRefresh(token) && name == "openai" {
		if err := auth.RefreshOpenAIToken(
			s.store,
			s.store.Dir(),
			openaiTokenURL,
			openaiClientID,
		); err != nil {
			msg := "OpenAI token refresh failed. Run: liteoauthllm login openai"
			writeAuthError(w, name, msg)
			return
		}
		// Re-read the freshly-refreshed token.
		token, _ = s.store.Read(name)
	} else if auth.NeedsRefresh(token) {
		msg := fmt.Sprintf("%s token expired. Run: liteoauthllm login %s", capitalize(name), name)
		writeAuthError(w, name, msg)
		return
	}

	// InjectHeaders mutates the request — this is the standard Go reverse proxy pattern.
	// The request is consumed by the proxy and not reused after this point.
	prov.InjectHeaders(r, token.AccessToken)

	var upstreamURL string
	if override, ok := s.upstreamOverrides[name]; ok {
		upstreamURL = override
	} else {
		upstreamURL = prov.UpstreamScheme() + "://" + prov.UpstreamHost()
	}

	target, parseErr := url.Parse(upstreamURL)
	if parseErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.URL.Path = prov.RewritePath(req.URL.Path)
		},
		// FlushInterval -1 enables immediate flushing for SSE streaming responses.
		FlushInterval: -1,
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			// Client disconnects (context canceled / deadline exceeded) are
			// normal in eval pipelines and streaming — suppress the noisy
			// default "http: proxy error" log line.
			if req.Context().Err() != nil || err == context.Canceled {
				return
			}
			log.Printf("proxy error: %s %s %s: %v", req.Method, req.URL.Path, name, err)
			rw.WriteHeader(http.StatusBadGateway)
		},
	}

	if s.verbose {
		rec := &statusRecorder{ResponseWriter: w, statusCode: 200}
		proxy.ServeHTTP(rec, r)
		log.Printf("→ %s %s %s %s %d", r.Method, r.URL.Path, name, time.Since(start).Round(time.Millisecond), rec.statusCode)
	} else {
		proxy.ServeHTTP(w, r)
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code written
// by the upstream response, used for verbose request logging.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// capitalize returns s with the first byte uppercased.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
