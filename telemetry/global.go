package telemetry

import "context"

var globalProvider Provider = NewNoopProvider()

// SetProvider updates the global telemetry provider.
func SetProvider(provider Provider) {
	if provider == nil {
		globalProvider = NewNoopProvider()
		return
	}

	globalProvider = provider
}

// GetProvider returns the active global telemetry provider.
func GetProvider() Provider {
	if globalProvider == nil {
		return NewNoopProvider()
	}

	return globalProvider
}

// Start starts a span with the active global provider.
func Start(ctx context.Context, name string, fields Fields) (context.Context, Span) {
	return GetProvider().Start(ctx, name, fields)
}
