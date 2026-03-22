package gateway

import "errors"

var (
	// ErrNoProvider is returned when no provider is configured for the requested model.
	ErrNoProvider = errors.New("gateway: no provider configured for model")

	// ErrAllProvidersFailed is returned when every provider in the fallback chain fails.
	ErrAllProvidersFailed = errors.New("gateway: all providers in fallback chain failed")

	// ErrInvalidTier is returned when an unrecognised ModelTier is used.
	ErrInvalidTier = errors.New("gateway: invalid model tier")

	// ErrClassificationFailed is returned when the judge-router cannot classify a task.
	ErrClassificationFailed = errors.New("gateway: task classification failed")

	// ErrProviderUnavailable is returned when a specific provider cannot be reached.
	ErrProviderUnavailable = errors.New("gateway: provider unavailable")

	// ErrRateLimited is returned when a provider responds with rate-limit status.
	ErrRateLimited = errors.New("gateway: provider rate limited")

	// ErrConfigInvalid is returned when the gateway configuration is malformed.
	ErrConfigInvalid = errors.New("gateway: invalid configuration")

	// ErrCostTrackerFull is returned when the cost tracker's capacity is exceeded.
	ErrCostTrackerFull = errors.New("gateway: cost tracker capacity exceeded")
)

// ProviderError wraps an error with the provider and operation that produced it.
type ProviderError struct {
	Op       string // e.g. "Route", "Complete", "GetCostReport"
	Provider string
	Err      error
}

func (e *ProviderError) Error() string {
	msg := "gateway: "
	if e.Op != "" {
		msg += e.Op + ": "
	}
	msg += "provider " + e.Provider + ": " + e.Err.Error()
	return msg
}

func (e *ProviderError) Unwrap() error { return e.Err }

// FallbackError aggregates errors from all providers in a fallback chain.
type FallbackError struct {
	Errors []ProviderError
}

func (e *FallbackError) Error() string {
	msg := "gateway: all providers failed:"
	for _, pe := range e.Errors {
		msg += "\n  " + pe.Error()
	}
	return msg
}

func (e *FallbackError) Unwrap() error { return ErrAllProvidersFailed }
