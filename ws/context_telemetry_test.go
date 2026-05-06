package ws

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wonli/aqi/telemetry"
)

func TestObserveWithoutProviderDoesNotPanic(t *testing.T) {
	oldProvider := telemetry.GetProvider()
	defer telemetry.SetProvider(oldProvider)

	telemetry.SetProvider(telemetry.NewNoopProvider())

	ctx := &Context{}
	require.NotPanics(t, func() {
		ctx.Observe(map[string]any{"k": "v"})
	})
}
