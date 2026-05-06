package telemetry

import "context"

type spanContextKey struct{}

// ContextWithSpan stores span in context for downstream code.
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext returns span from context or a noop span.
func SpanFromContext(ctx context.Context) Span {
	if ctx == nil {
		return noopSpanInstance
	}

	span, ok := ctx.Value(spanContextKey{}).(Span)
	if !ok || span == nil {
		return noopSpanInstance
	}

	return span
}
