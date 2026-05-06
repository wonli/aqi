package telemetry

import "context"

type noopProvider struct{}

type noopSpan struct{}

var noopSpanInstance Span = noopSpan{}

// NewNoopProvider returns a provider that does nothing.
func NewNoopProvider() Provider {
	return noopProvider{}
}

func (noopProvider) Start(ctx context.Context, _ string, _ Fields) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	return ContextWithSpan(ctx, noopSpanInstance), noopSpanInstance
}

func (noopSpan) SetFields(Fields)             {}
func (noopSpan) RecordError(error)            {}
func (noopSpan) SetStatus(StatusCode, string) {}
func (noopSpan) End()                         {}
