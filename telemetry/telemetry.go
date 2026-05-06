package telemetry

import "context"

// Fields stores span fields in a transport-agnostic way.
type Fields map[string]any

// StatusCode is a transport-agnostic span status.
type StatusCode string

const (
	StatusUnset StatusCode = "unset"
	StatusOK    StatusCode = "ok"
	StatusError StatusCode = "error"
)

// Span is the minimal tracing surface needed by aqi.
type Span interface {
	SetFields(fields Fields)
	RecordError(err error)
	SetStatus(code StatusCode, msg string)
	End()
}

// Provider starts spans for a request.
type Provider interface {
	Start(ctx context.Context, name string, fields Fields) (context.Context, Span)
}
