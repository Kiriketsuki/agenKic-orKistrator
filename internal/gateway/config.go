package gateway

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
type GatewayConfig struct {
	Gateway              GatewaySettings           `yaml:"gateway"`
	Tiers                map[ModelTier]TierConfig  `yaml:"tiers"`
	Providers            map[string]ProviderConfig `yaml:"providers"`
	CostPerMillionTokens map[string]TokenCost      `yaml:"cost_per_million_tokens"`
}

// rawTierConfig is the YAML shape for a tier entry.
type rawTierConfig struct {
	PrimaryModel  string   `yaml:"primary_model"`
	FallbackChain []string `yaml:"fallback_chain"`
}

// rawProviderConfig is the YAML shape for a provider entry.
type rawProviderConfig struct {
	BaseURL string   `yaml:"base_url"`
	Models  []string `yaml:"models"`
}

// rawGatewayConfig mirrors the YAML file before conversion to domain types.
type rawGatewayConfig struct {
	Gateway              GatewaySettings              `yaml:"gateway"`
	Tiers                map[string]rawTierConfig     `yaml:"tiers"`
	Providers            map[string]rawProviderConfig `yaml:"providers"`
	CostPerMillionTokens map[string]TokenCost         `yaml:"cost_per_million_tokens"`
}

// LoadConfig reads and parses the YAML configuration at path.
func LoadConfig(path string) (GatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GatewayConfig{}, fmt.Errorf("%w: read config %q: %v", ErrConfigInvalid, path, err)
	}
	return parseConfig(data)
}

// parseConfig parses raw YAML bytes into a GatewayConfig.
func parseConfig(data []byte) (GatewayConfig, error) {
	var raw rawGatewayConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return GatewayConfig{}, fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}

	cfg := GatewayConfig{
		Gateway:              raw.Gateway,
		Tiers:                make(map[ModelTier]TierConfig, len(raw.Tiers)),
		Providers:            make(map[string]ProviderConfig, len(raw.Providers)),
		CostPerMillionTokens: raw.CostPerMillionTokens,
	}

	for tierName, rt := range raw.Tiers {
		tier := ModelTier(tierName)
		cfg.Tiers[tier] = TierConfig{
			Tier:          tier,
			PrimaryModel:  rt.PrimaryModel,
			FallbackChain: rt.FallbackChain,
		}
	}

	for name, rp := range raw.Providers {
		cfg.Providers[name] = ProviderConfig{
			Name:    name,
			BaseURL: rp.BaseURL,
			Models:  rp.Models,
		}
	}

	if cfg.CostPerMillionTokens == nil {
		cfg.CostPerMillionTokens = make(map[string]TokenCost)
	}

	return cfg, nil
}

// ValidateConfig checks that required fields are present and internally consistent.
func ValidateConfig(cfg GatewayConfig) error {
	if cfg.Gateway.LiteLLMBaseURL == "" {
		return fmt.Errorf("%w: gateway.litellm_base_url is required", ErrConfigInvalid)
	}
	if cfg.Gateway.TimeoutSeconds <= 0 {
		return fmt.Errorf("%w: gateway.timeout_seconds must be positive", ErrConfigInvalid)
	}
	if len(cfg.Tiers) == 0 {
		return fmt.Errorf("%w: at least one tier must be defined", ErrConfigInvalid)
	}

	for tier, tc := range cfg.Tiers {
		if !tier.Valid() {
			return fmt.Errorf("%w: unknown tier %q", ErrConfigInvalid, tier)
		}
		if tc.PrimaryModel == "" {
			return fmt.Errorf("%w: tier %q has no primary_model", ErrConfigInvalid, tier)
		}
	}

	return nil
}
