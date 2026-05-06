package ws

import "github.com/wonli/aqi/telemetry"

// Observe records a whitelist of business fields on the current span.
func (c *Context) Observe(fields map[string]any) {
	if c == nil || len(fields) == 0 {
		return
	}

	telemetry.SpanFromContext(c.Context()).SetFields(fields)
}
