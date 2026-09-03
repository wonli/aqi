package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	gobwasws "github.com/gobwas/ws"
	"github.com/stretchr/testify/require"

	"github.com/wonli/aqi/telemetry"
	"github.com/wonli/aqi/ws"
)

var middlewareTestRouteID atomic.Uint64

type mockProvider struct {
	name   string
	fields telemetry.Fields
	span   *mockSpan
}

type mockSpan struct {
	fields    telemetry.Fields
	errors    []string
	status    telemetry.StatusCode
	statusMsg string
	ended     bool
}

func (p *mockProvider) Start(ctx context.Context, name string, fields telemetry.Fields) (context.Context, telemetry.Span) {
	p.name = name
	p.fields = cloneFields(fields)
	p.span = &mockSpan{fields: telemetry.Fields{}}
	for key, value := range fields {
		p.span.fields[key] = value
	}
	return telemetry.ContextWithSpan(ctx, p.span), p.span
}

func (s *mockSpan) SetFields(fields telemetry.Fields) {
	if s.fields == nil {
		s.fields = telemetry.Fields{}
	}
	for key, value := range fields {
		s.fields[key] = value
	}
}

func (s *mockSpan) RecordError(err error) {
	if err != nil {
		s.errors = append(s.errors, err.Error())
	}
}

func (s *mockSpan) SetStatus(code telemetry.StatusCode, msg string) {
	s.status = code
	s.statusMsg = msg
}

func (s *mockSpan) End() { s.ended = true }

func TestTelemetryObserveRecordsFields(t *testing.T) {
	oldProvider := telemetry.GetProvider()
	defer telemetry.SetProvider(oldProvider)
	provider := &mockProvider{}
	telemetry.SetProvider(provider)

	ws.NewServer(http.NewServeMux())
	action := middlewareTestAction("telemetry.observe")
	router := ws.NewRouter().Use(Telemetry())
	router.Add(action, func(a *ws.Context) {
		a.Observe(map[string]any{"shop_id": "shop-1", "count": 2})
		a.SendOk()
	})

	client := &ws.Client{ClientId: "client-1", AppId: "app", Platform: "ios", Send: make(chan ws.Message, 1)}
	ws.Dispatcher(client, fmt.Sprintf(`{"id":"req-1","action":%q,"params":"{}"}`, action))

	require.Equal(t, action, provider.name)
	require.Equal(t, action, provider.span.fields["action"])
	require.Equal(t, "req-1", provider.span.fields["request_id"])
	require.Equal(t, "client-1", provider.span.fields["client_id"])
	require.Equal(t, "shop-1", provider.span.fields["shop_id"])
	require.Equal(t, 2, provider.span.fields["count"])
	require.Equal(t, 0, provider.span.fields["response_code"])
	require.Equal(t, telemetry.StatusOK, provider.span.status)
	require.True(t, provider.span.ended)
}

func TestRecoveryRecordsPanicOnSpan(t *testing.T) {
	oldProvider := telemetry.GetProvider()
	defer telemetry.SetProvider(oldProvider)
	provider := &mockProvider{}
	telemetry.SetProvider(provider)

	ws.NewServer(http.NewServeMux())
	action := middlewareTestAction("telemetry.panic")
	router := ws.NewRouter().Use(Telemetry(), Recovery())
	router.Add(action, func(a *ws.Context) { panic("boom") })

	client := &ws.Client{Send: make(chan ws.Message, 1)}
	ws.Dispatcher(client, fmt.Sprintf(`{"id":"req-2","action":%q,"params":"{}"}`, action))

	require.Equal(t, telemetry.StatusError, provider.span.status)
	require.Equal(t, "服务维护中", provider.span.fields["response_message"])
	require.Equal(t, -30, provider.span.fields["response_code"])
	require.Contains(t, provider.span.errors, "panic: boom")
	require.True(t, provider.span.ended)
}

func TestRecoveryStopsHandlersAfterPanicAndQueuesErrorResponse(t *testing.T) {
	oldProvider := telemetry.GetProvider()
	defer telemetry.SetProvider(oldProvider)
	provider := &mockProvider{}
	telemetry.SetProvider(provider)

	ws.NewServer(http.NewServeMux())
	action := middlewareTestAction("telemetry.panic.abort")
	afterPanicCalled := false
	router := ws.NewRouter().Use(Telemetry(), Recovery())
	router.Add(action,
		func(a *ws.Context) { panic("stop here") },
		func(a *ws.Context) { afterPanicCalled = true },
	)

	client := &ws.Client{Send: make(chan ws.Message, 2)}
	ws.Dispatcher(client, fmt.Sprintf(`{"id":"req-3","action":%q,"params":"{}"}`, action))

	require.False(t, afterPanicCalled, "handlers after a recovered panic must not run")
	require.NotNil(t, provider.span)
	require.Equal(t, telemetry.StatusError, provider.span.status)
	require.Contains(t, provider.span.errors, "panic: stop here")
	select {
	case msg := <-client.Send:
		require.Equal(t, gobwasws.OpText, msg.Op)
		require.Contains(t, string(msg.Data), `"code":-30`)
		require.Contains(t, string(msg.Data), "服务维护中")
	default:
		t.Fatal("recovery did not queue an error response")
	}
}

func TestRecoveryPassesThroughWithoutPanic(t *testing.T) {
	ws.NewServer(http.NewServeMux())
	action := middlewareTestAction("recovery.pass")
	called := false
	router := ws.NewRouter().Use(Recovery())
	router.Add(action, func(a *ws.Context) {
		called = true
		a.SendOk()
	})

	client := &ws.Client{Send: make(chan ws.Message, 1)}
	ws.Dispatcher(client, fmt.Sprintf(`{"id":"req-4","action":%q,"params":"{}"}`, action))

	require.True(t, called)
	select {
	case msg := <-client.Send:
		require.Equal(t, gobwasws.OpText, msg.Op)
		require.Contains(t, string(msg.Data), `"code":0`)
	default:
		t.Fatal("downstream handler did not queue its response")
	}
}

func TestSpanFromContextFallsBackToNoop(t *testing.T) {
	span := telemetry.SpanFromContext(nil)
	require.NotNil(t, span)
	require.NotPanics(t, func() {
		span.RecordError(errors.New("boom"))
		span.SetStatus(telemetry.StatusError, "bad")
		span.End()
	})
}

func middlewareTestAction(prefix string) string {
	return fmt.Sprintf("%s.%d", prefix, middlewareTestRouteID.Add(1))
}

func cloneFields(fields telemetry.Fields) telemetry.Fields {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(telemetry.Fields, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}
