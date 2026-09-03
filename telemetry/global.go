package telemetry

import (
	"context"
	"sync/atomic"
)

type providerHolder struct {
	provider Provider
}

var globalProvider atomic.Pointer[providerHolder]

// SetProvider updates the global telemetry provider.
func SetProvider(provider Provider) {
	if provider == nil {
		provider = NewNoopProvider()
	}

	globalProvider.Store(&providerHolder{provider: provider})
}

// GetProvider returns the active global telemetry provider.
func GetProvider() Provider {
	holder := globalProvider.Load()
	if holder == nil || holder.provider == nil {
		return NewNoopProvider()
	}

	return holder.provider
}

// Start starts a span with the active global provider.
func Start(ctx context.Context, name string, fields Fields) (context.Context, Span) {
	return GetProvider().Start(ctx, name, fields)
}
