# Feature: LiteLLM Client & Provider Adapters (T2, T3)

## Overview

**User Story**: As a gateway developer, I want a LiteLLM proxy client and a set of provider format adapters so that the orchestrator can send completion requests to any supported AI provider (Anthropic, OpenAI, Ollama) through a single, consistent Go interface without per-provider calling code scattered across the codebase.

**Problem**: Each AI provider exposes a different HTTP API shape — Anthropic uses `messages` with `anthropic-version` headers; OpenAI-compatible providers (Gemini, Ollama) accept `/chat/completions` but differ in model naming, temperature constraints, and capability flags. Without an adapter layer, every new provider requires invasive changes to request-building logic. Without a unified HTTP client, rate-limit retries, timeout enforcement, and error normalisation must be reimplemented for every call site.

**Out of Scope**:
- Judge-Router classification logic (T4)
- Config loading and fallback chain orchestration (T5, T6)
- Cost tracking (T7)
- Wiring into the orchestrator supervisor (T8)
- Streaming completions (not required for MVP)
- Gemini-native format translation (Gemini is accessed via LiteLLM's OpenAI-compatible endpoint — no bespoke adapter needed at this stage)

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the LiteLLM client retry on 429 internally, or delegate retry to the fallback chain (T6)? Internal retry with backoff is simpler but risks masking cascading failures. | T2 spec | [ ] |
| 2 | Does the Anthropic adapter need to translate `system` messages into the top-level `system` field (Anthropic API v1 format) or can we rely on LiteLLM to handle that translation? | T3 spec | [x] Resolved: LiteLLM accepts system messages in the messages array and translates for Anthropic. `buildRequest` prepends `{role: "system"}` when `SystemPrompt` is set. |
| 3 | Should the `ollama/` prefix be stripped before forwarding to LiteLLM, or should LiteLLM receive the full prefixed name? LiteLLM's Ollama support expects the prefix. | T3 spec | [x] Resolved: Acceptance scenario (line 208) is controlling — prefix is stripped by `OllamaAdapter.FormatRequest` (not `ParseModelName`, which only returns `bool` for matching). |
| 4 | What is the canonical timeout for completion calls? Gateway-level timeout vs per-request override? | T2 spec | [ ] |

---

## Scope

### Must-Have
- **LiteLLM proxy client** implementing `gateway.Completer` — a single Go struct that POSTs to `/chat/completions` on the configured LiteLLM base URL and returns a `CompletionResponse`
- **Functional options** for client construction: `WithBaseURL(url)`, `WithTimeout(d)`, `WithHTTPClient(c)` — allows test injection of a custom `http.Client` without global state
- **Error normalisation** — HTTP 429 maps to `gateway.ErrRateLimited`; HTTP 5xx maps to `gateway.ErrProviderUnavailable`; non-JSON or structurally incomplete responses map to a descriptive error; network errors are wrapped and returned as-is
- **`FormatAdapter` interface** — `Name() string`, `FormatRequest(req CompletionRequest) CompletionRequest`, `ParseModelName(model string) bool` — each provider implements this to adjust provider-specific fields (temperature clamping, model name normalization) on the typed request before serialization. Note: `FormatRequest` intentionally omits an error return — current adapters only perform infallible field adjustments. When a future adapter needs to reject a request, evolve to `(CompletionRequest, error)` and update `Registry.Resolve` accordingly. Blast radius at time of writing: 3 adapters, 1 call site.
- **Anthropic adapter** — handles all `claude-*` model names; clamps `Temperature` to `[0.0, 1.0]` (Anthropic rejects values above 1.0); passes `MaxTokens` as `max_tokens`
- **OpenAI adapter** — handles `gpt-*`, `o1-*`, `o3-*` model names; zeros `Temperature` for reasoning models (`o1-*`, `o3-*`) because they do not support sampling temperature; passes `MaxTokens` as `max_tokens`
- **Ollama adapter** — handles `ollama/*` model names; strips the `ollama/` prefix before forwarding to LiteLLM; passes all fields through without modification (Ollama models do not have special temperature constraints at the gateway level)
- **Adapter registry** — `Registry` struct with ordered adapter list and two methods: `Find(model string) (FormatAdapter, error)` returns the first matching adapter or `ErrNoProvider`; `Resolve(model string, req CompletionRequest) (CompletionRequest, error)` calls `Find` then `FormatRequest`. `Registry` implements the `gateway.AdapterResolver` interface, which `LiteLLMClient` depends on via dependency inversion

### Should-Have
- Request/response logging at `DEBUG` level using `log/slog` — logs model name, token counts, and HTTP status without logging message content
- Context propagation — `context.Context` cancellation and deadline respected in HTTP calls

### Nice-to-Have
- Retry-with-backoff on 429 inside the client (configurable attempt count; default 0 = no retry, delegates to T6 fallback chain)
- Connection pooling tuning via `WithHTTPClient` (callers can pass a pre-configured `http.Transport`)

---

## Technical Plan

### Affected Components

| Path | Change |
|:-----|:-------|
| `internal/gateway/litellm.go` | New: LiteLLM proxy client (`LiteLLMClient`) and OpenAI-compatible request/response structs |
| `internal/gateway/litellm_test.go` | New: unit tests for `LiteLLMClient` using `httptest.NewServer` |
| `internal/gateway/providers/anthropic.go` | New: `AnthropicAdapter` implementing `FormatAdapter` |
| `internal/gateway/providers/openai.go` | New: `OpenAIAdapter` implementing `FormatAdapter` |
| `internal/gateway/providers/ollama.go` | New: `OllamaAdapter` implementing `FormatAdapter` |
| `internal/gateway/providers/provider.go` | New: `FormatAdapter` interface, `Registry` with `Find`/`Resolve`, `DefaultRegistry` |
| `internal/gateway/providers/provider_test.go` | New: registry routing tests + per-adapter formatting and temperature constraint tests |

### API Contracts

**FormatAdapter interface** (lives in `internal/gateway/providers/`):

```go
// FormatAdapter adjusts a CompletionRequest for a specific provider's quirks.
type FormatAdapter interface {
    // Name returns the provider identifier (e.g. "anthropic", "openai", "ollama").
    Name() string

    // FormatRequest returns a (possibly modified) copy of req tailored for this
    // provider. It must not mutate the original request. Note: Go's shallow copy
    // (out := req) shares reference-type fields like Metadata — implementations
    // must not write to shared maps or slices.
    //
    // Intentionally omits an error return — current adapters only perform
    // infallible field adjustments (temperature clamping, model name stripping).
    // When a future adapter needs to reject a request, evolve this signature to
    // (CompletionRequest, error) and update Registry.Resolve accordingly.
    // Blast radius at time of writing: 3 adapters, 1 call site.
    FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest

    // ParseModelName returns true when this adapter should handle the given model
    // name. The model name is matched before FormatRequest is applied.
    ParseModelName(model string) bool
}
```

**Registry** (lives in `internal/gateway/providers/`):

```go
// Find returns the FormatAdapter whose ParseModelName reports true for model.
// If no adapter matches, it returns gateway.ErrNoProvider.
func (r *Registry) Find(model string) (FormatAdapter, error)

// Resolve implements gateway.AdapterResolver. It finds the adapter for the
// given model and applies its FormatRequest transformation.
func (r *Registry) Resolve(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error)
```

**AdapterResolver interface** (lives in `internal/gateway/gateway.go` — dependency inversion):

```go
// AdapterResolver finds and applies the format adapter for a model name.
// Implementations should return ErrNoProvider if no adapter handles the model.
type AdapterResolver interface {
    Resolve(model string, req CompletionRequest) (CompletionRequest, error)
}
```

**LiteLLM client** (lives in `internal/gateway/litellm.go`):

```go
// LiteLLMClient implements gateway.Completer by proxying requests through a
// LiteLLM sidecar over its OpenAI-compatible HTTP API.
type LiteLLMClient struct { /* unexported fields */ }

// NewLiteLLMClient constructs a client with the given functional options.
// Defaults: baseURL = "http://localhost:4000", timeout = 30s.
func NewLiteLLMClient(opts ...Option) *LiteLLMClient

// Option is a functional option for LiteLLMClient.
type Option func(*LiteLLMClient)

func WithBaseURL(url string) Option
func WithTimeout(d time.Duration) Option
func WithHTTPClient(c *http.Client) Option

// Complete implements gateway.Completer.
func (c *LiteLLMClient) Complete(ctx context.Context, req gateway.CompletionRequest) (gateway.CompletionResponse, error)

// Provider implements gateway.Completer. Returns "litellm".
func (c *LiteLLMClient) Provider() string
```

**OpenAI-compatible wire types** (unexported, used within `litellm.go`):

```go
type chatCompletionRequest struct {
    Model       string               `json:"model"`
    Messages    []chatMessage        `json:"messages"`
    MaxTokens   int                  `json:"max_tokens,omitempty"`
    Temperature *float64             `json:"temperature,omitempty"`
}

type chatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type chatCompletionResponse struct {
    ID      string   `json:"id"`
    Choices []choice `json:"choices"`
    Usage   usage    `json:"usage"`
    Model   string   `json:"model"`
}

type choice struct {
    Message      chatMessage `json:"message"`
    FinishReason string      `json:"finish_reason"`
}

type usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
}
```

### Temperature Constraint Rules

| Adapter | Model pattern | Temperature behaviour |
|:--------|:-------------|:----------------------|
| Anthropic | `claude-*` | Clamp to `[0.0, 1.0]`; values `< 0` use provider default (omit field) |
| OpenAI | `gpt-*` | Pass through; values `< 0` use provider default (omit field) |
| OpenAI | `o1-*`, `o3-*` | Always zero the temperature field (reasoning models ignore it but reject non-zero) |
| Ollama | `ollama/*` | Pass through unchanged |

### Dependencies

- `net/http` (stdlib) — HTTP transport
- `encoding/json` (stdlib) — request/response serialisation
- `context` (stdlib) — cancellation/deadline propagation
- `log/slog` (stdlib, Go 1.21+) — structured debug logging
- No external dependencies required for this feature

**LiteLLM sidecar** must be deployed and reachable at the configured base URL. This is an operational dependency, not a code dependency.

### Risks

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| LiteLLM response schema changes across versions | Medium | Pin LiteLLM version in sidecar deployment; write a response schema validation test against a live or recorded fixture |
| Ollama prefix stripping breaks if LiteLLM adds native `ollama/` routing | Low | Guard with a version flag; adapter can be made a no-op if LiteLLM handles routing internally |
| Reasoning model temperature zeroing masks legitimate temperature values in logs | Low | Log the original requested temperature alongside the clamped value at DEBUG level |
| `httptest.Server` tests may not catch real LiteLLM serialisation edge cases | Medium | Add a manual integration test target (`go test -tags=integration`) that calls a real LiteLLM instance |

---

## Acceptance Scenarios

```gherkin
Feature: LiteLLM Client & Provider Adapters
  As a gateway developer
  I want a unified LiteLLM proxy client and provider adapters
  So that all AI provider calls go through a single, consistent interface

  Background:
    Given a LiteLLM proxy is running at "http://localhost:4000"
    And an AdapterRegistry is populated with Anthropic, OpenAI, and Ollama adapters

  Rule: Successful completion returns normalised response

    Scenario: Complete a request to a Claude model
      Given a CompletionRequest with model "claude-haiku-4-5" and one user message
      And the LiteLLM proxy responds with HTTP 200 and a valid chat completion JSON
      When LiteLLMClient.Complete is called
      Then a CompletionResponse is returned with non-empty Content
      And InputTokens and OutputTokens are populated from the response usage field
      And ProviderName is "litellm"
      And FallbackUsed is false

    Scenario: Complete a request to an Ollama model
      Given a CompletionRequest with model "ollama/llama3" and one user message
      And the LiteLLM proxy responds with HTTP 200
      When LiteLLMClient.Complete is called
      Then the HTTP request body sent to LiteLLM has model set to "llama3" (prefix stripped)
      And a CompletionResponse is returned with non-empty Content

  Rule: Rate limiting returns ErrRateLimited

    Scenario: Provider responds with HTTP 429
      Given a CompletionRequest with model "claude-sonnet-4-6"
      And the LiteLLM proxy responds with HTTP 429
      When LiteLLMClient.Complete is called
      Then an error is returned that matches gateway.ErrRateLimited
      And the error wraps the HTTP 429 status

  Rule: Server errors return ErrProviderUnavailable

    Scenario: Provider responds with HTTP 500
      Given a CompletionRequest with model "gpt-4o"
      And the LiteLLM proxy responds with HTTP 500
      When LiteLLMClient.Complete is called
      Then an error is returned that matches gateway.ErrProviderUnavailable
      And the error includes the HTTP status code

    Scenario: Provider responds with HTTP 503
      Given a CompletionRequest with model "claude-opus-4-6"
      And the LiteLLM proxy responds with HTTP 503
      When LiteLLMClient.Complete is called
      Then an error is returned that matches gateway.ErrProviderUnavailable

  Rule: Malformed response is an error

    Scenario: Provider responds with HTTP 200 but invalid JSON
      Given a CompletionRequest with model "gpt-4o"
      And the LiteLLM proxy responds with HTTP 200 and body "not json"
      When LiteLLMClient.Complete is called
      Then an error is returned describing a JSON parse failure
      And the error does not match gateway.ErrRateLimited or gateway.ErrProviderUnavailable

    Scenario: Provider responds with HTTP 200 but empty choices array
      Given a CompletionRequest with model "claude-haiku-4-5"
      And the LiteLLM proxy responds with HTTP 200 and a JSON body where choices is []
      When LiteLLMClient.Complete is called
      Then an error is returned indicating no choices in response

  Rule: Provider adapter routes to correct adapter by model prefix

    Scenario: Model "claude-haiku-4-5" routes to Anthropic adapter
      When AdapterRegistry.Lookup is called with "claude-haiku-4-5"
      Then the returned adapter has Name() == "anthropic"

    Scenario: Model "gpt-4o" routes to OpenAI adapter
      When AdapterRegistry.Lookup is called with "gpt-4o"
      Then the returned adapter has Name() == "openai"

    Scenario: Model "o1-mini" routes to OpenAI adapter
      When AdapterRegistry.Lookup is called with "o1-mini"
      Then the returned adapter has Name() == "openai"

    Scenario: Model "ollama/llama3" routes to Ollama adapter
      When AdapterRegistry.Lookup is called with "ollama/llama3"
      Then the returned adapter has Name() == "ollama"

    Scenario: Unknown model prefix returns ErrNoProvider
      When AdapterRegistry.Lookup is called with "unknown-model-xyz"
      Then an error is returned that matches gateway.ErrNoProvider

  Rule: Anthropic adapter enforces temperature constraints

    Scenario: Temperature above 1.0 is clamped to 1.0
      Given an AnthropicAdapter
      And a CompletionRequest with Temperature set to 1.5
      When FormatRequest is called
      Then the formatted request has temperature set to 1.0

    Scenario: Negative temperature is omitted
      Given an AnthropicAdapter
      And a CompletionRequest with Temperature set to -1
      When FormatRequest is called
      Then the formatted request does not include the temperature field

    Scenario: Temperature of 0.7 is preserved
      Given an AnthropicAdapter
      And a CompletionRequest with Temperature set to 0.7
      When FormatRequest is called
      Then the formatted request has temperature set to 0.7

  Rule: OpenAI adapter zeroes temperature for reasoning models

    Scenario: o1 reasoning model gets temperature zeroed
      Given an OpenAIAdapter
      And a CompletionRequest with model "o1-mini" and Temperature set to 0.9
      When FormatRequest is called
      Then the formatted request has temperature set to 0.0

    Scenario: o3 reasoning model gets temperature zeroed
      Given an OpenAIAdapter
      And a CompletionRequest with model "o3-mini" and Temperature set to 0.5
      When FormatRequest is called
      Then the formatted request has temperature set to 0.0

    Scenario: gpt-4o preserves requested temperature
      Given an OpenAIAdapter
      And a CompletionRequest with model "gpt-4o" and Temperature set to 0.8
      When FormatRequest is called
      Then the formatted request has temperature set to 0.8

  Rule: Context cancellation aborts the HTTP call

    Scenario: Context is cancelled before response arrives
      Given a CompletionRequest with model "claude-haiku-4-5"
      And the context is cancelled before the LiteLLM proxy responds
      When LiteLLMClient.Complete is called
      Then an error is returned wrapping context.Canceled
```

---

## Task Breakdown

| ID  | Task | Priority | Dependencies | Status |
|:----|:-----|:---------|:-------------|:-------|
| T2.1 | Define OpenAI-compatible wire structs (`chatCompletionRequest`, `chatCompletionResponse`, etc.) in `litellm.go` | High | T1 | pending |
| T2.2 | Implement `LiteLLMClient` struct, `NewLiteLLMClient`, and functional options (`WithBaseURL`, `WithTimeout`, `WithHTTPClient`) | High | T2.1 | pending |
| T2.3 | Implement `LiteLLMClient.Complete`: marshal request, POST, unmarshal response, map to `CompletionResponse` | High | T2.2 | pending |
| T2.4 | Implement HTTP error mapping (429 → `ErrRateLimited`, 5xx → `ErrProviderUnavailable`, empty choices → error) | High | T2.3 | pending |
| T2.5 | Write unit tests for `LiteLLMClient` using `httptest.NewServer` covering: success, 429, 5xx, bad JSON, empty choices, context cancel | High | T2.4 | pending |
| T3.1 | Define `FormatAdapter` interface in `internal/gateway/providers/` | High | T1 | pending |
| T3.2 | Implement `AnthropicAdapter` with temperature clamping and `ParseModelName` identity pass-through | High | T3.1 | pending |
| T3.3 | Implement `OpenAIAdapter` with reasoning model temperature zeroing (`o1-*`, `o3-*`) | High | T3.1 | pending |
| T3.4 | Implement `OllamaAdapter` with `ollama/` prefix stripping in `FormatRequest` | High | T3.1 | pending |
| T3.5 | Implement `Registry` with `Find` and `Resolve` methods, plus `DefaultRegistry` factory | High | T3.2–T3.4 | pending |
| T3.6 | Write unit tests for all adapters: temperature constraints, model name parsing, field preservation, unknown model error | High | T3.5 | pending |
| T3.7 | Integrate adapter resolution into `LiteLLMClient.Complete`: inject `AdapterResolver` via `WithAdapterResolver` option, call `Resolve` before `buildRequest` | High | T2.3, T3.5 | pending |

---

## Exit Criteria

- [ ] `LiteLLMClient` compiles and implements `gateway.Completer`
- [ ] All acceptance scenarios pass as Go unit tests (no external service required — `httptest.NewServer` used throughout)
- [ ] `FormatAdapter` interface is implemented by all three adapters (Anthropic, OpenAI, Ollama)
- [ ] `AdapterRegistry.Lookup` correctly routes all model prefixes defined in acceptance scenarios
- [ ] Temperature clamping: Anthropic adapter rejects Temperature > 1.0 by clamping; OpenAI adapter zeros temperature for o1/o3 models — both verified in tests
- [ ] Error sentinel values (`ErrRateLimited`, `ErrProviderUnavailable`, `ErrNoProvider`) are returned correctly and unwrappable with `errors.Is`
- [ ] No external dependencies added beyond stdlib
- [ ] `go vet ./internal/gateway/...` passes clean

---

## References

- Epic spec: `model-gateway-spec.md`
- Gateway types and interfaces: `internal/gateway/gateway.go`
- Error sentinels: `internal/gateway/errors.go`
- GitHub issue: #33
- LiteLLM OpenAI-compatible proxy docs: https://docs.litellm.ai/docs/proxy/quick_start
- Anthropic temperature constraints: https://docs.anthropic.com/en/api/messages (temperature range 0–1)
- OpenAI reasoning model limitations: https://platform.openai.com/docs/guides/reasoning (temperature must be 1 or omitted for o1/o3)

---

*Authored by: Clault KiperS 4.6*
