package ws

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestClientJSONExcludesRuntimeState(t *testing.T) {
	client := &Client{
		ClientId:       "client-1",
		Send:           make(chan []byte),
		RequestQueue:   make(chan string),
		ValidCacheData: func() {},
		Keys:           map[string]any{"bad": func() {}},
	}
	client.Hub = &Hubc{Clients: map[*Client]struct{}{client: {}}}

	data, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client: %v", err)
	}

	encoded := string(data)
	for _, field := range []string{"Hub", "Send", "RequestQueue", "ValidCacheData", "Keys"} {
		if strings.Contains(encoded, `"`+field+`"`) {
			t.Fatalf("runtime field %s leaked into JSON: %s", field, encoded)
		}
	}
}
