package aqi

import "testing"

func TestWebSocketMaxMessageSizeOptionRejectsNonPositiveValues(t *testing.T) {
	if err := WebSocketMaxMessageSize(0)(&AppConfig{}); err == nil {
		t.Fatal("expected non-positive websocket max message size to be rejected")
	}
}
