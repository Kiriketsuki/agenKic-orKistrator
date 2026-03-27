# Feature: Config Loader & Fallback Chains (T5, T6)

## Overview

**User Story**: As an orchestrator operator, I want to define tier-to-model mappings, provider
settings, and fallback chains in a YAML file that I can change without modifying code, so that
I can adapt the gateway to new providers and models without a redeployment.

**Problem**: Without an external configuration file, every change to model assignments, fallback
ordering, or per-provider settings requires a code edit and recompile. Operators cannot tune
cost/performance tradeoffs at runtime, and provider outages cannot be mitigated by adjusting
fallback chains without touching source. Additionally, there is no retry logic: if the primary
model returns an error, the request fails entirely instead of being retried with the next
provider in a configured chain.

**Out of Scope**:
- Dynamic hot-reload of `models.yaml` without restart (YAML is parsed once at startup)
- Provider API key management — keys are injected via environment variables, not stored in YAML
- Judge-router logic and task classification (T4)
- Cost reporting and aggregation (T7)
- Orchestrator wiring and end-to-end integration tests (T8, T9)

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should `models.yaml` include API keys, or should those always come from env vars? | T5 planning | [ ] |
| 2 | Should `ValidateConfig` cross-check that every model in a fallback chain appears in some provider's model list? | T5 planning | [ ] |
| 3 | Should `FallbackCompleter.chainForModel` treat an unknown-tier model as a single-entry chain (no fallback) or hard-fail immediately? | T6 planning | [ ] |
| 4 | Should transient errors (network timeout) trigger fallback, while permanent errors (invalid API key) short-circuit the chain? | T6 planning | [ ] |

---

## Scope

### Must-Have
- **`config/models.yaml`** — operator-editable YAML file with: top-level gateway settings
  (`litellm_base_url`, `timeout_seconds`), three tier definitions (`cheap`, `mid`, `frontier`)
  each with a `primary_model` and `fallback_chain`, per-provider `base_url` and `models` list,
  and a `cost_per_million_tokens` pricing table keyed by model name
- **`GatewayConfig`, `GatewaySettings`, `TokenCost` types** — strongly typed Go structs that
  hold the parsed config; must not duplicate any types already defined in `gateway.go`
- **`LoadConfig(path string) (GatewayConfig, error)`** — reads YAML from disk and returns a
  fully parsed `GatewayConfig`
- **`ValidateConfig(cfg GatewayConfig) error`** — validates required fields, positive timeout,
  at least one tier, and that every tier's `primary_model` is non-empty; returns a wrapped
  `ErrConfigInvalid` sentinel on failure
- **`FallbackCompleter` struct** — wraps a map of model-name → `Completer` instances plus a
  `GatewayConfig`; implements the `Completer` interface
- **`FallbackCompleter.Complete`** — resolves the model in the request to its tier, builds the
  ordered chain `[primary, ...fallback_chain]`, tries each in sequence, sets `FallbackUsed=true`
  when a fallback (index > 0) serves the request, returns `*FallbackError` if all fail
- **`FallbackCompleter.CompleteWithTier`** — resolves a `ModelTier` to its primary model and
  chain directly, overriding any model set on the request; validates tier before attempting
- **`ErrNoProvider` for unknown models** — `Complete` must return `ErrNoProvider` when the
  requested model is neither a primary model in any tier nor registered in the completers map

### Should-Have
- **`NewFallbackCompleter` constructor** — functional constructor accepting a completers map and
  a `GatewayConfig`; enables dependency injection and clean testing
- **Partial-chain degradation** — if a model in the chain has no registered `Completer`, it is
  skipped with a `ProviderError` rather than panicking; the chain continues to the next entry

### Nice-to-Have
- **Config schema documentation** — inline YAML comments in `models.yaml` explaining each field
- **`ValidateConfig` cross-checks fallback chains** — warns if a model in a fallback chain is
  not listed under any known provider

---

## Technical Plan

### Affected Components

| File | Change |
|:-----|:-------|
| `config/models.yaml` | New file — full gateway YAML schema |
| `internal/gateway/config.go` | New file — `GatewayConfig`, `GatewaySettings`, `TokenCost`, `LoadConfig`, `parseConfig`, `ValidateConfig` |
| `internal/gateway/fallback.go` | New file — `FallbackCompleter`, `NewFallbackCompleter`, `Complete`, `CompleteWithTier`, `chainForModel`, `tryChain` |
| `internal/gateway/config_test.go` | New file — table-driven tests for `parseConfig` and `ValidateConfig` |
| `internal/gateway/fallback_test.go` | New file — table-driven tests for all `FallbackCompleter` scenarios |
| `go.mod` / `go.sum` | Add `gopkg.in/yaml.v3` dependency |

### YAML Schema (`config/models.yaml`)

```yaml
gateway:
  litellm_base_url: "http://localhost:4000"   # LiteLLM proxy endpoint
  timeout_seconds: 30                          # per-request HTTP timeout

tiers:
  cheap:
    primary_model: "claude-haiku-4-5-20251001"
    fallback_chain:
      - "gpt-4o-mini"
      - "ollama/llama3"
  mid:
    primary_model: "claude-sonnet-4-6"
    fallback_chain:
      - "gpt-4o"
      - "gemini-pro"
  frontier:
    primary_model: "claude-opus-4-6"
    fallback_chain:
      - "gpt-4o"
      - "claude-sonnet-4-6"

providers:
  anthropic:
    base_url: "https://api.anthropic.com"
    models: ["claude-haiku-4-5-20251001", "claude-sonnet-4-6", "claude-opus-4-6"]
  openai:
    base_url: "https://api.openai.com"
    models: ["gpt-4o", "gpt-4o-mini"]
  ollama:
    base_url: "http://localhost:11434"
    models: ["ollama/llama3", "ollama/codellama"]
  gemini:
    base_url: "https://generativelanguage.googleapis.com"
    models: ["gemini-pro"]

cost_per_million_tokens:
  claude-haiku-4-5-20251001: { input: 0.80,  output: 4.00  }
  claude-sonnet-4-6:         { input: 3.00,  output: 15.00 }
  claude-opus-4-6:           { input: 15.00, output: 75.00 }
  gpt-4o:                    { input: 2.50,  output: 10.00 }
  gpt-4o-mini:               { input: 0.15,  output: 0.60  }
  gemini-pro:                { input: 1.25,  output: 5.00  }
  ollama/llama3:             { input: 0.0,   output: 0.0   }
  ollama/codellama:          { input: 0.0,   output: 0.0   }
```

**Design notes**:
- Provider API keys are NOT in this file — they must be injected via environment variables
  (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) read at provider construction time
- `base_url` for each provider overrides the LiteLLM default for direct HTTP usage
- Tier keys must match the `ModelTier` constants (`cheap`, `mid`, `frontier`)
- Cost values of `0.0` for Ollama models reflect local inference with no API billing

### Go Types

```go
// config.go

// GatewaySettings holds top-level connection settings for the gateway.
type GatewaySettings struct {
    LiteLLMBaseURL string `yaml:"litellm_base_url"`
    TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// TokenCost holds per-million-token pricing for a model.
type TokenCost struct {
    Input  float64 `yaml:"input"`
    Output float64 `yaml:"output"`
}

// GatewayConfig is the fully parsed gateway configuration.
// TierConfig and ProviderConfig are defined in gateway.go.
type GatewayConfig struct {
    Gateway              GatewaySettings           `yaml:"gateway"`
    Tiers                map[ModelTier]TierConfig  `yaml:"tiers"`
    Providers            map[string]ProviderConfig `yaml:"providers"`
    CostPerMillionTokens map[string]TokenCost      `yaml:"cost_per_million_tokens"`
}
```

**Note**: `TierConfig` and `ProviderConfig` are already defined in `gateway.go` (T1). The YAML
layer uses internal raw structs (`rawTierConfig`, `rawProviderConfig`) because `yaml.v3` cannot
unmarshal directly into `ModelTier` map keys; the `parseConfig` function performs the conversion.

### Go Functions

```go
// config.go

// LoadConfig reads and parses the YAML configuration at path.
// Returns a wrapped ErrConfigInvalid if the file is unreadable or malformed.
func LoadConfig(path string) (GatewayConfig, error)

// parseConfig parses raw YAML bytes into a GatewayConfig.
// Converts string tier keys to ModelTier and string provider keys to ProviderConfig.
// Not exported — called by LoadConfig and directly in tests.
func parseConfig(data []byte) (GatewayConfig, error)

// ValidateConfig checks that required fields are present and internally consistent.
// Returns a wrapped ErrConfigInvalid describing the first violation found.
// Rules:
//   - gateway.litellm_base_url must be non-empty
//   - gateway.timeout_seconds must be > 0
//   - at least one tier must be defined
//   - every tier key must be a valid ModelTier (cheap/mid/frontier)
//   - every tier must have a non-empty primary_model
func ValidateConfig(cfg GatewayConfig) error
```

```go
// fallback.go

// FallbackCompleter wraps a set of per-model Completers and applies the
// fallback chain defined in GatewayConfig when the primary model fails.
type FallbackCompleter struct {
    completers map[string]Completer
    config     GatewayConfig
}

// NewFallbackCompleter returns a FallbackCompleter backed by the given
// model-keyed completers and configuration.
func NewFallbackCompleter(completers map[string]Completer, config GatewayConfig) *FallbackCompleter

// Complete resolves the model in req to a tier, then attempts the primary
// model and each fallback in order. The first success is returned with
// FallbackUsed=true when a non-primary model served the request.
// Returns *FallbackError if all providers fail.
// Returns ErrNoProvider if the model is not found in any tier or completer map.
func (fc *FallbackCompleter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

// CompleteWithTier resolves tier to its primary model, then attempts the
// primary model and each fallback in order.
// Returns ErrInvalidTier for unrecognised or unconfigured tiers.
func (fc *FallbackCompleter) CompleteWithTier(ctx context.Context, tier ModelTier, req CompletionRequest) (CompletionResponse, error)
```

### Dependencies

| Package | Version | Purpose |
|:--------|:--------|:--------|
| `gopkg.in/yaml.v3` | latest | YAML parsing for `models.yaml` |
| `context` | stdlib | Context propagation through completion calls |
| `errors` | stdlib | `errors.As` for `ProviderError` unwrapping |
| `fmt` | stdlib | Error formatting with sentinel wrapping |
| `os` | stdlib | `os.ReadFile` in `LoadConfig` |

### Risks

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `yaml.v3` cannot unmarshal `ModelTier` map keys directly (string → custom type) | High (known Go limitation) | Use intermediate `rawGatewayConfig` with `map[string]...` keys; convert in `parseConfig` |
| Operator misconfigures fallback chain with a model name that has no registered `Completer` | Medium | `tryChain` records a `ProviderError` and skips the entry rather than panicking; `ValidateConfig` can optionally cross-check |
| All models in a tier chain use the same upstream provider (no actual redundancy) | Low-Medium | Document recommendation in `models.yaml` comments; operator responsibility |
| Context cancellation mid-chain stops all remaining fallback attempts | Low | `tryChain` must check `ctx.Err()` before each attempt and short-circuit on cancellation |

---

## Acceptance Scenarios

```gherkin
Feature: Config Loader & Fallback Chains
  As an orchestrator operator
  I want gateway configuration loaded from YAML and fallback retry on provider failure
  So that I can tune model assignments without code changes and survive provider outages

  Background:
    Given "config/models.yaml" defines three tiers: cheap (Haiku), mid (Sonnet), frontier (Opus)
    And each tier has a primary_model and a fallback_chain of two alternative models
    And four providers are configured: anthropic, openai, ollama, gemini

  Rule: Config loading

    Scenario: Valid config loads successfully
      Given "config/models.yaml" contains all required fields
      When LoadConfig is called with the path
      Then it returns a GatewayConfig with no error
      And cfg.Gateway.LiteLLMBaseURL equals "http://localhost:4000"
      And cfg.Gateway.TimeoutSeconds equals 30
      And cfg.Tiers["cheap"].PrimaryModel equals "claude-haiku-4-5-20251001"
      And cfg.Tiers["mid"].FallbackChain contains ["gpt-4o", "gemini-pro"]
      And cfg.CostPerMillionTokens["claude-opus-4-6"].Output equals 75.00

    Scenario: Config with missing litellm_base_url fails validation
      Given a GatewayConfig where gateway.litellm_base_url is empty
      When ValidateConfig is called
      Then it returns an error wrapping ErrConfigInvalid
      And the error message mentions "litellm_base_url"

    Scenario: Config with zero timeout fails validation
      Given a GatewayConfig where gateway.timeout_seconds is 0
      When ValidateConfig is called
      Then it returns an error wrapping ErrConfigInvalid
      And the error message mentions "timeout_seconds"

    Scenario: Config with no tiers fails validation
      Given a GatewayConfig with an empty tiers map
      When ValidateConfig is called
      Then it returns an error wrapping ErrConfigInvalid
      And the error message mentions "tier"

    Scenario: Config with a tier missing its primary_model fails validation
      Given a GatewayConfig where the "mid" tier has an empty primary_model
      When ValidateConfig is called
      Then it returns an error wrapping ErrConfigInvalid
      And the error message mentions "primary_model"

    Scenario: Config with malformed YAML returns parse error
      Given a YAML file containing invalid syntax
      When LoadConfig is called with that path
      Then it returns an error wrapping ErrConfigInvalid

  Rule: Fallback chain — primary succeeds

    Scenario: Primary model succeeds on first attempt
      Given a FallbackCompleter configured with the mid tier (primary: claude-sonnet-4-6)
      And the claude-sonnet-4-6 completer returns a successful response
      When Complete is called with Model "claude-sonnet-4-6"
      Then the response is returned with FallbackUsed equal to false
      And the ProviderName in the response is "anthropic"

  Rule: Fallback chain — primary fails, fallback succeeds

    Scenario: Primary provider returns HTTP 500, first fallback succeeds
      Given a FallbackCompleter configured with the mid tier
      And the claude-sonnet-4-6 completer returns a ProviderError (HTTP 500)
      And the gpt-4o completer returns a successful response
      When Complete is called with Model "claude-sonnet-4-6"
      Then the response is returned successfully
      And FallbackUsed is true
      And the ProviderName in the response is "openai"

    Scenario: First and second providers fail, third fallback succeeds
      Given a FallbackCompleter configured with the mid tier
      And claude-sonnet-4-6 returns a ProviderError
      And gpt-4o returns a ProviderError
      And gemini-pro returns a successful response
      When Complete is called with Model "claude-sonnet-4-6"
      Then the response is returned successfully
      And FallbackUsed is true
      And the ProviderName in the response is "gemini"

  Rule: Fallback chain — all providers fail

    Scenario: All providers in the chain fail
      Given a FallbackCompleter configured with the mid tier
      And claude-sonnet-4-6, gpt-4o, and gemini-pro all return ProviderErrors
      When Complete is called with Model "claude-sonnet-4-6"
      Then a *FallbackError is returned
      And the FallbackError.Errors slice contains three ProviderErrors
      And errors.Is(err, ErrAllProvidersFailed) returns true

  Rule: Unknown model

    Scenario: Request for a model not found in any tier or completer
      Given a FallbackCompleter with no completer registered for "unknown-model-xyz"
      And "unknown-model-xyz" is not the primary_model of any tier
      When Complete is called with Model "unknown-model-xyz"
      Then ErrNoProvider is returned
      And no completion attempt is made

  Rule: CompleteWithTier routing

    Scenario: CompleteWithTier resolves tier to primary model
      Given a FallbackCompleter with the cheap tier configured
      And the claude-haiku-4-5-20251001 completer returns a successful response
      When CompleteWithTier is called with tier "cheap" and an empty model field
      Then the request is forwarded to the claude-haiku-4-5-20251001 completer
      And the response is returned with FallbackUsed equal to false

    Scenario: CompleteWithTier with an invalid tier returns ErrInvalidTier
      Given a FallbackCompleter
      When CompleteWithTier is called with tier "premium"
      Then ErrInvalidTier is returned
      And no completion attempt is made

    Scenario: CompleteWithTier falls through to fallback when primary fails
      Given a FallbackCompleter with the cheap tier configured
      And claude-haiku-4-5-20251001 returns a ProviderError
      And gpt-4o-mini returns a successful response
      When CompleteWithTier is called with tier "cheap"
      Then the response is returned successfully
      And FallbackUsed is true
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T5a | Define `GatewaySettings`, `TokenCost`, `GatewayConfig` types in `config.go` | High | T1 | pending |
| T5b | Implement `parseConfig` (YAML → domain types, raw intermediate structs) | High | T5a | pending |
| T5c | Implement `LoadConfig(path)` (file I/O → `parseConfig`) | High | T5b | pending |
| T5d | Implement `ValidateConfig` (required fields, tier invariants) | High | T5a | pending |
| T5e | Write `config/models.yaml` with full schema (three tiers, four providers, pricing table) | High | T5a | pending |
| T5f | Write table-driven tests for `parseConfig` and `ValidateConfig` | High | T5b, T5d | pending |
| T6a | Implement `FallbackCompleter` struct and `NewFallbackCompleter` constructor | High | T1, T5 | pending |
| T6b | Implement `chainForModel` (model → tier lookup → ordered chain) | High | T6a | pending |
| T6c | Implement `tryChain` (iterate chain, accumulate `ProviderError`s, set `FallbackUsed`) | High | T6a | pending |
| T6d | Implement `Complete` (delegates to `chainForModel` + `tryChain`) | High | T6b, T6c | pending |
| T6e | Implement `CompleteWithTier` (tier → primary + chain, validates tier first) | High | T6c | pending |
| T6f | Write table-driven tests for all fallback scenarios (primary ok, partial fallback, total failure, unknown model, tier routing) | High | T6d, T6e | pending |

---

## Exit Criteria

- [ ] `LoadConfig("config/models.yaml")` returns a valid `GatewayConfig` with no error when the
  file contains all required fields
- [ ] `ValidateConfig` rejects configs missing `litellm_base_url`, non-positive `timeout_seconds`,
  empty tier map, or a tier without a `primary_model` — each with a wrapped `ErrConfigInvalid`
- [ ] `FallbackCompleter.Complete` returns `FallbackUsed=false` when the primary model succeeds
- [ ] `FallbackCompleter.Complete` returns `FallbackUsed=true` and the correct `ProviderName`
  when any fallback serves the request
- [ ] `FallbackCompleter.Complete` returns a `*FallbackError` (wrapping `ErrAllProvidersFailed`)
  when all providers in the chain fail, with one `ProviderError` per provider in the list
- [ ] `FallbackCompleter.Complete` returns `ErrNoProvider` for an unknown model
- [ ] `FallbackCompleter.CompleteWithTier` resolves tier to primary model and applies the same
  fallback logic; returns `ErrInvalidTier` for unrecognised tiers
- [ ] All tests pass under `go test ./internal/gateway/...` with `-race`
- [ ] No mutations of shared state (immutable request copies passed to each `Completer`)

---

## References

- Epic spec: `specs/model-gateway-spec.md`
- Gateway interface and types: `internal/gateway/gateway.go` (T1)
- Error sentinels (`ErrNoProvider`, `ErrAllProvidersFailed`, `ErrConfigInvalid`, `FallbackError`, `ProviderError`): `internal/gateway/errors.go` (T1)
- GitHub issue: #37

---

*Authored by: Clault KiperS 4.6*
