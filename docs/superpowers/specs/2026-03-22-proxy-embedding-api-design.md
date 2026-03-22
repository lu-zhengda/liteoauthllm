# Proxy Embedding API — Design Spec

## Summary

Add `POST /v1/embeddings` to liteoauthllm, supporting both OpenAI and Anthropic embedding models through the existing OAuth token proxy. OpenAI feasibility is unknown (Codex OAuth may not expose embeddings), so the work is phased: research + POC first, then Anthropic support, then finalization.

## Motivation

Users who rely on liteoauthllm for LLM inference also need embeddings for RAG, search, and classification. Today they must configure separate API keys for embedding access. Proxying the embedding API through the same OAuth tokens removes that friction.

## Phased Approach

### Phase 1 — Research + OpenAI POC

**Goal:** Determine whether the Codex OAuth token grants embedding access, and discover the correct upstream endpoint.

**Research (before code):**

1. Search GitHub for projects using the same Codex OAuth client ID (`app_EMoamEEZ73f0CkXaXp7hrann`) — look for embedding-related paths or documentation.
2. Search for community-documented `chatgpt.com/backend-api` endpoints (reverse-engineering repos, wikis, blog posts).
3. Check OpenAI's official Codex CLI source code for any embedding references.
4. Search forums (Reddit, HN, OpenAI community) for discussions about embedding access via ChatGPT subscription tokens.
5. Verify Anthropic's exact embedding API path and whether `sk-ant-oat01-` setup-tokens grant embedding access (this is a Phase 2 blocker).

**Endpoint testing (informed by research):**

- Try candidate endpoints discovered from research first.
- If no leads, try obvious guesses:
  - `chatgpt.com/backend-api/codex/embeddings`
  - `chatgpt.com/backend-api/embeddings`
- Test whether the OAuth token works against `api.openai.com/v1/embeddings` directly.
- Document all attempts (status codes, error messages) in a markdown file.

**Decision gate:** If no working OpenAI endpoint is found, proceed with Anthropic-only and document the limitation.

### Phase 2 — Anthropic Support + Routing

**Goal:** Add full embedding proxy with dual-provider routing.

**Routing for `POST /v1/embeddings`:**

Resolution order:
1. If `anthropic-version` header is present -> Anthropic.
2. If no disambiguating header -> read request body, extract `model` field:
   - Model starts with `voyage-` -> Anthropic.
   - Anything else -> OpenAI (default).
3. If body parsing fails, no `model` field, or empty body -> OpenAI (default).

**Body peek implementation:**

- Triggered only when header sniffing is inconclusive (no `anthropic-version` header).
- Read the request body into a byte buffer, capped at `maxBodyPeekSize` (10 MB) via `io.LimitReader` to prevent memory exhaustion from oversized payloads.
- Decode a minimal JSON struct (`struct{ Model string }`) to extract the model name.
- Reassign `req.Body` to a new `io.NopCloser(bytes.NewReader(buf))` so upstream receives the complete body.
- If JSON decode fails, body is empty, or `model` field is absent -> default to OpenAI. The proxy does not reject malformed bodies; that is the upstream's responsibility. Verbose-mode logging should flag body-peek parse failures for debugging.
- Concurrency safety: body peeking operates exclusively on the per-request `*http.Request` object. No shared mutable state (buffers, readers) is used across goroutines.

**Method handling:** The proxy does not enforce HTTP method restrictions on any route, including `/v1/embeddings`. Non-POST requests will match the route and be forwarded to the upstream, which will reject invalid methods. This is consistent with existing behavior for all other routes. Body peeking on a request with no body (e.g., GET) simply yields an empty read, defaulting to OpenAI.

**Registry change (`provider.go`):**

Add `/v1/embeddings` to the `Resolve()` switch. Unlike other routes, this one may require body peeking. To avoid changing `Resolve()`'s error semantics, extract body peeking into a separate function:

- `Resolve()` returns a dedicated sentinel error `provider.ErrNeedsBodyPeek` when it encounters `/v1/embeddings` without an `anthropic-version` header. The provider name and instance are not returned in this case.
- `ServeHTTP()` in `proxy.go` uses `errors.Is(err, provider.ErrNeedsBodyPeek)` to distinguish this from `ErrUnknownRoute` (which maps to 404). On `ErrNeedsBodyPeek`, it calls a new `resolveFromBody(req *http.Request) (string, Provider, error)` function that performs the body peek and returns the final provider name + instance. This function defaults to OpenAI on any failure (empty body, malformed JSON, missing model, oversized body) and never returns an error.
- This keeps `Resolve()` pure (header-only, no I/O) and avoids changing its error contract for existing routes.

All other routes remain header-only.

### Phase 3 — Finalize

Based on POC results:
- If OpenAI works: enable and document.
- If OpenAI does not work: remove experimental OpenAI embedding code, ship Anthropic-only, document the limitation in README.

## Provider Changes

### OpenAI (`openai.go`)

- `RewritePath()`: `/v1/embeddings` -> path TBD from POC (likely `/backend-api/codex/embeddings`).
- `InjectHeaders()`: No change — same `Authorization: Bearer <token>`.
- If the upstream turns out to be `api.openai.com` instead of `chatgpt.com`, two contingency options:
  - **Option A: Second provider instance** — register an `openai-embeddings` provider with `UpstreamHost()` returning `api.openai.com`. The token store would need to resolve both `openai` and `openai-embeddings` to the same stored token (add a provider-alias mechanism or have `Resolve()` return the token key separately from the provider).
  - **Option B: Path-aware `UpstreamHost()`** — pass the request path to `UpstreamHost(path)`, changing the interface for all providers. This is more invasive.
  - **Decision:** Deferred to POC results. Option A is preferred for its lower blast radius.

### Anthropic (`anthropic.go`)

- `RewritePath()`: `/v1/embeddings` passes through unchanged (assumption — Anthropic's Voyager embedding API path needs verification in Phase 1).
- `InjectHeaders()`: No change — same `Authorization: Bearer <token>` + `anthropic-version` + `anthropic-beta`.
- Target models: `voyage-3`, `voyage-3-lite`, `voyage-code-3`, etc.

### Provider Interface

No changes to the `Provider` interface. Body-peek routing logic lives in `Registry.Resolve()`, not in individual providers.

## Proxy Layer Changes

### `proxy.go`

- `ServeHTTP()` flow is unchanged: resolve -> load token -> check expiry -> inject headers -> forward.
- When `Resolve()` triggers body peeking, the body is already buffered and reassigned — the reverse proxy forwards it transparently.
- Embedding responses are regular JSON (not SSE), so existing `FlushInterval: -1` is harmless.

### `errors.go`

- No changes. The proxy does not reject malformed request bodies — that is the upstream's responsibility. This is consistent with all existing routes.

### Config

- No new config fields. The feature is always-on once the route exists.
- Verbose mode logging covers embedding requests via the existing log line.

## Testing

### Unit Tests

- **Routing** (`provider_test.go`):
  - `/v1/embeddings` with `anthropic-version` header -> Anthropic.
  - `/v1/embeddings` with `voyage-3` model in body, no header -> Anthropic.
  - `/v1/embeddings` with `text-embedding-3-small` model in body -> OpenAI.
  - `/v1/embeddings` with no header and no model -> OpenAI (default).
  - `/v1/embeddings` with malformed JSON body -> OpenAI (default).
  - `/v1/embeddings` with empty body (no content) -> OpenAI (default).
  - Body is correctly reassigned after peeking (upstream receives full body).
  - Body exceeding `maxBodyPeekSize` -> OpenAI (default), no OOM.

- **Proxy** (`proxy_test.go`):
  - Embedding request forwarding with correct auth injection and path rewriting.

### Integration Tests (`integration_test.go`)

- OpenAI embedding -> correct upstream host + rewritten path + auth header.
- Anthropic embedding with `anthropic-version` header -> correct upstream + headers.
- Anthropic embedding with `voyage-` model (no header) -> correct routing via body peek.

### E2E Tests (`test/e2e/`)

- Only after POC confirms working endpoints.
- Anthropic: send `voyage-3` embedding request through proxy, verify vectors returned.
- OpenAI: if feasible, send embedding request through proxy, verify vectors returned.

### POC Validation

- All endpoint attempts and responses documented in `docs/poc-embedding-results.md`.

## Open Questions

1. Does the Codex OAuth token grant access to any embedding endpoint?
2. If yes, what is the upstream host and path?
3. Are there rate limits or model restrictions specific to OAuth-based embedding access?
4. Does Anthropic's Voyager embedding API work with the setup-token (`sk-ant-oat01-`) authentication method?
