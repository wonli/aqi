package aqi

import "testing"

func TestWebSocketMaxFrameSizeOptionStoresConfig(t *testing.T) {
	config := &AppConfig{}
	if err := WebSocketMaxFrameSize(3 << 20)(config); err != nil {
		t.Fatalf("configure websocket max frame size: %v", err)
	}
	if config.WebSocketMaxFrameSize != 3<<20 {
		t.Fatalf("expected websocket max frame size %d, got %d", 3<<20, config.WebSocketMaxFrameSize)
	}
}

func TestWebSocketMaxFrameSizeOptionKeepsGobwasDefault(t *testing.T) {
	config := &AppConfig{}
	if err := WebSocketMaxFrameSize(0)(config); err != nil {
		t.Fatalf("configure gobwas default max frame size: %v", err)
	}
	if config.WebSocketMaxFrameSize != 0 {
		t.Fatalf("expected zero to keep gobwas unlimited behavior, got %d", config.WebSocketMaxFrameSize)
	}
}
