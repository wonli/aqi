package ws

import (
	"context"
	"testing"
)

func TestClientContextFollowsConnectionLifetime(t *testing.T) {
	type contextKey string
	const key contextKey = "request-value"

	requestCtx, cancelRequest := context.WithCancel(context.WithValue(context.Background(), key, "preserved"))
	client := &Client{}
	client.initContext(requestCtx)

	cancelRequest()
	select {
	case <-client.Context().Done():
		t.Fatal("connection context was canceled when the upgrade request ended")
	default:
	}
	if got := client.Context().Value(key); got != "preserved" {
		t.Fatalf("connection context did not preserve request value: got %v", got)
	}

	client.cancel()
	select {
	case <-client.Context().Done():
	default:
		t.Fatal("connection context was not canceled when the connection ended")
	}
}
