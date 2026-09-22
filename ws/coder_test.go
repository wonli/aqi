package ws

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"
)

type testBinaryCoder struct{}

func (testBinaryCoder) Decode(data []byte) (*Request, error) {
	parts := bytes.SplitN(data, []byte{0}, 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid test binary request")
	}

	return &Request{
		Id:     "binary-request",
		Action: string(parts[0]),
		Params: parts[1],
	}, nil
}

func (testBinaryCoder) Bind(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (testBinaryCoder) Encode(a *Action) ([]byte, error) {
	return []byte("binary:" + a.Action), nil
}

func TestJSONCoderDecodesAQIRequest(t *testing.T) {
	req, err := decodeRequest([]byte(`{"id":"req-1","action":"demo.echo","params":{"name":"aqi"}}`), defaultJSONCoder)
	require.NoError(t, err)
	require.Equal(t, "req-1", req.Id)
	require.Equal(t, "demo.echo", req.Action)
	require.JSONEq(t, `{"name":"aqi"}`, string(req.Params))

	var payload struct {
		Name string `json:"name"`
	}
	require.NoError(t, defaultJSONCoder.Bind(req.Params, &payload))
	require.Equal(t, "aqi", payload.Name)
}

func TestRouterCoderScopesRoutes(t *testing.T) {
	m := &ActionManager{routeMap: map[string]*route{}}
	root := Routers{manager: m}
	binary := root.Coder(testBinaryCoder{})

	root.Add("text.echo", func(*Context) {})
	binary.Add("binary.echo", func(*Context) {})

	require.Nil(t, m.route("text.echo").coder)
	require.IsType(t, testBinaryCoder{}, m.route("binary.echo").coder)
	require.NotNil(t, m.Coder())
}

func TestContextBindUsesRouteCoderAndRequestPayload(t *testing.T) {
	ctx := &Context{
		Params:  `{"name":"legacy"}`,
		request: &Request{Params: []byte(`{"name":"binary"}`)},
		route:   &route{coder: testBinaryCoder{}},
	}

	var payload struct {
		Name string `json:"name"`
	}
	require.NoError(t, ctx.Bind(&payload))
	require.Equal(t, "binary", payload.Name)
}

func TestReaderDecodesTextAndBinaryThroughSameRequestPipeline(t *testing.T) {
	m := InitManager()
	oldCoder := m.coder
	m.coder = nil
	defer func() { m.coder = oldCoder }()

	textAction := fmt.Sprintf("test.text.%d", time.Now().UnixNano())
	binaryAction := fmt.Sprintf("test.binary.%d", time.Now().UnixNano())
	var received []string
	handler := func(c *Context) {
		var payload struct {
			Name string `json:"name"`
		}
		require.NoError(t, c.Bind(&payload))
		received = append(received, payload.Name)
	}

	NewRouter().Add(textAction, handler)
	NewRouter().Coder(testBinaryCoder{}).Add(binaryAction, handler)

	serverConn, peerConn := net.Pipe()
	client := &Client{
		Conn:         serverConn,
		Send:         make(chan []byte, 2),
		RequestQueue: make(chan *Request, 2),
	}
	client.initContext(context.Background())

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		client.Reader()
	}()

	text := []byte(fmt.Sprintf(`{"id":"text-1","action":%q,"params":{"name":"text"}}`, textAction))
	require.NoError(t, wsutil.WriteClientMessage(peerConn, ws.OpText, text))
	binary := append(append([]byte(binaryAction), 0), []byte(`{"name":"binary"}`)...)
	require.NoError(t, wsutil.WriteClientMessage(peerConn, ws.OpBinary, binary))

	for i := 0; i < 2; i++ {
		select {
		case req := <-client.RequestQueue:
			dispatcher(client, req)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for decoded request")
		}
	}

	require.Equal(t, []string{"text", "binary"}, received)

	_ = peerConn.Close()
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("Reader did not exit")
	}
}

func TestReaderIgnoresBinaryWithoutCoderAndKeepsTextWorking(t *testing.T) {
	m := InitManager()
	oldCoder := m.coder
	m.coder = nil
	defer func() { m.coder = oldCoder }()

	serverConn, peerConn := net.Pipe()
	client := &Client{
		Conn:         serverConn,
		Send:         make(chan []byte, 1),
		RequestQueue: make(chan *Request, 1),
	}
	client.initContext(context.Background())

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		client.Reader()
	}()

	require.NoError(t, wsutil.WriteClientMessage(peerConn, ws.OpBinary, []byte("unsupported")))
	require.NoError(t, wsutil.WriteClientMessage(peerConn, ws.OpText, []byte(`{"id":"req-1","action":"text.still.works","params":{}}`)))

	select {
	case req := <-client.RequestQueue:
		require.Equal(t, "text.still.works", req.Action)
	case <-time.After(time.Second):
		t.Fatal("text request was not decoded after unsupported binary frame")
	}

	_ = peerConn.Close()
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("Reader did not exit")
	}
}

func TestContextSendUsesRouteCoderWithoutBinaryAPI(t *testing.T) {
	m := InitManager()
	oldCoder := m.coder
	m.coder = nil
	defer func() { m.coder = oldCoder }()

	textAction := fmt.Sprintf("send.text.%d", time.Now().UnixNano())
	binaryAction := fmt.Sprintf("send.binary.%d", time.Now().UnixNano())
	NewRouter().Add(textAction, func(*Context) {})
	NewRouter().Coder(testBinaryCoder{}).Add(binaryAction, func(*Context) {})

	serverConn, peerConn := net.Pipe()
	client := &Client{
		Conn: serverConn,
		Send: make(chan []byte, 2),
	}
	client.initContext(context.Background())
	ctx := &Context{Client: client}

	ctx.SendActionData(textAction, H{"kind": "text"})
	ctx.SendActionData(binaryAction, H{"kind": "binary"})

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		client.Write()
	}()

	frames := map[ws.OpCode]string{}
	for i := 0; i < 2; i++ {
		data, op, err := wsutil.ReadServerData(peerConn)
		require.NoError(t, err)
		frames[op] = string(data)
	}

	require.Contains(t, frames[ws.OpText], textAction)
	require.Equal(t, "binary:"+binaryAction, frames[ws.OpBinary])

	client.Disconnect()
	_ = peerConn.Close()
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("Writer did not exit")
	}
}

func TestBinaryResponseDisconnectsAfterSend(t *testing.T) {
	server, peer := net.Pipe()
	defer peer.Close()
	client := &Client{Conn: server, Disconnecting: true}
	client.initContext(context.Background())
	defer client.Disconnect()
	client.sendFrame(frame{op: ws.OpBinary, data: []byte("goodbye")})
	done := make(chan struct{})
	go func() { defer close(done); client.Write() }()
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	data, op, err := wsutil.ReadServerData(peer)
	require.NoError(t, err)
	require.Equal(t, ws.OpBinary, op)
	require.Equal(t, "goodbye", string(data))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("binary response did not disconnect")
	}
}
