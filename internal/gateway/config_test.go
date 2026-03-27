package gateway

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

const testYAML = `
gateway:
  litellm_base_url: "http://localhost:4000"
  timeout_seconds: 30

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
    models: ["ollama/llama3"]

cost_per_million_tokens:
  claude-haiku-4-5-20251001: { input: 0.80, output: 4.00 }
  claude-sonnet-4-6: { input: 3.00, output: 15.00 }
  claude-opus-4-6: { input: 15.00, output: 75.00 }
  gpt-4o: { input: 2.50, output: 10.00 }
  gpt-4o-mini: { input: 0.15, output: 0.60 }
  ollama/llama3: { input: 0.0, output: 0.0 }
`

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig([]byte(testYAML))
	if err != nil {
		t.Fatalf("parseConfig error: %v", err)
	}

	if cfg.Gateway.LiteLLMBaseURL != "http://localhost:4000" {
		t.Errorf("LiteLLMBaseURL = %q, want http://localhost:4000", cfg.Gateway.LiteLLMBaseURL)
	}
	if cfg.Gateway.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.Gateway.TimeoutSeconds)
	}

	if len(cfg.Tiers) != 3 {
		t.Fatalf("len(Tiers) = %d, want 3", len(cfg.Tiers))
	}

	cheap, ok := cfg.Tiers[TierCheap]
	if !ok {
		t.Fatal("missing tier cheap")
	}
	if cheap.PrimaryModel != "claude-haiku-4-5-20251001" {
		t.Errorf("cheap.PrimaryModel = %q", cheap.PrimaryModel)
	}
	if len(cheap.FallbackChain) != 2 {
		t.Errorf("cheap fallback chain len = %d, want 2", len(cheap.FallbackChain))
	}

	if len(cfg.Providers) != 3 {
		t.Fatalf("len(Providers) = %d, want 3", len(cfg.Providers))
	}

	cost, ok := cfg.CostPerMillionTokens["claude-haiku-4-5-20251001"]
	if !ok {
		t.Fatal("missing cost for claude-haiku-4-5-20251001")
	}
	if cost.Input != 0.80 {
		t.Errorf("haiku input cost = %v, want 0.80", cost.Input)
	}
	if cost.Output != 4.00 {
		t.Errorf("haiku output cost = %v, want 4.00", cost.Output)
	}
}

func TestParseConfigInvalidYAML(t *testing.T) {
	// An unclosed quoted string causes a genuine yaml parse error.
	_, err := parseConfig([]byte("gateway:\n  litellm_base_url: \"unclosed string\n"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !errors.Is(err, ErrConfigInvalid) {
		t.Errorf("error = %v, want wrapping ErrConfigInvalid", err)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*GatewayConfig)
		wantErr bool
	}{
		{
			name:    "valid config",
			mutate:  func(_ *GatewayConfig) {},
			wantErr: false,
		},
		{
			name:    "missing litellm_base_url",
			mutate:  func(c *GatewayConfig) { c.Gateway.LiteLLMBaseURL = "" },
			wantErr: true,
		},
		{
			name:    "zero timeout",
			mutate:  func(c *GatewayConfig) { c.Gateway.TimeoutSeconds = 0 },
			wantErr: true,
		},
		{
			name:    "negative timeout",
			mutate:  func(c *GatewayConfig) { c.Gateway.TimeoutSeconds = -1 },
			wantErr: true,
		},
		{
			name:    "no tiers",
			mutate:  func(c *GatewayConfig) { c.Tiers = nil },
			wantErr: true,
		},
		{
			name: "tier missing primary model",
			mutate: func(c *GatewayConfig) {
				tc := c.Tiers[TierCheap]
				tc.PrimaryModel = ""
				c.Tiers[TierCheap] = tc
			},
			wantErr: true,
		},
		{
			name: "unknown tier key",
			mutate: func(c *GatewayConfig) {
				c.Tiers[ModelTier("bogus")] = TierConfig{
					Tier:         ModelTier("bogus"),
					PrimaryModel: "some-model",
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig([]byte(testYAML))
			if err != nil {
				t.Fatalf("parseConfig: %v", err)
			}
			tt.mutate(&cfg)
			err = ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrConfigInvalid) {
				t.Errorf("error = %v, want wrapping ErrConfigInvalid", err)
			}
		})
	}
}

// repoRoot returns the repository root by walking up from the test file location.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	// internal/gateway/config_test.go → repo root is two dirs up
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(repoRoot(t), "config", "models.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(%q) error: %v", path, err)
	}
	if cfg.Gateway.LiteLLMBaseURL != "http://localhost:4000" {
		t.Errorf("LiteLLMBaseURL = %q, want http://localhost:4000", cfg.Gateway.LiteLLMBaseURL)
	}
	if cfg.Gateway.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.Gateway.TimeoutSeconds)
	}
	if len(cfg.Tiers) != 3 {
		t.Errorf("len(Tiers) = %d, want 3", len(cfg.Tiers))
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/models.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !errors.Is(err, ErrConfigInvalid) {
		t.Errorf("error = %v, want wrapping ErrConfigInvalid", err)
	}
}
