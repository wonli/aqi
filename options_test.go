package aqi

import "testing"

func TestWebSocketMaxFrameSizeOptionAllowsGobwasDefault(t *testing.T) {
	if err := WebSocketMaxFrameSize(0)(&AppConfig{}); err != nil {
		t.Fatalf("expected zero to keep gobwas unlimited behavior: %v", err)
	}
}
