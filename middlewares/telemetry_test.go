package middlewares

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wonli/aqi/telemetry"
	"github.com/wonli/aqi/ws"
)

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
	p.span = &mockSpan{
		fields: telemetry.Fields{},
	}

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
	if err == nil {
		return
	}

	s.errors = append(s.errors, err.Error())
}

func (s *mockSpan) SetStatus(code telemetry.StatusCode, msg string) {
	s.status = code
	s.statusMsg = msg
}

func (s *mockSpan) End() {
	s.ended = true
}

func TestTelemetryObserveRecordsFields(t *testing.T) {
	oldProvider := telemetry.GetProvider()
	defer telemetry.SetProvider(oldProvider)

	provider := &mockProvider{}
	telemetry.SetProvider(provider)

	ws.NewServer(http.NewServeMux())
	router := ws.NewRouter().Use(Telemetry())
	router.Add("telemetry.observe.test", func(a *ws.Context) {
		a.Observe(map[string]any{
			"shop_id": "shop-1",
			"count":   2,
		})
		a.SendOk()
	})

	client := &ws.Client{
		ClientId: "client-1",
		AppId:    "app",
		Platform: "ios",
		Send:     make(chan []byte, 1),
	}

	ws.Dispatcher(client, `{"id":"req-1","action":"telemetry.observe.test","params":"{}"}`)

	require.Equal(t, "telemetry.observe.test", provider.name)
	require.Equal(t, "telemetry.observe.test", provider.span.fields["action"])
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
	router := ws.NewRouter().Use(Telemetry(), Recovery())
	router.Add("telemetry.panic.test", func(a *ws.Context) {
		panic("boom")
	})

	client := &ws.Client{
		Send: make(chan []byte, 1),
	}

	ws.Dispatcher(client, `{"id":"req-2","action":"telemetry.panic.test","params":"{}"}`)

	require.Equal(t, telemetry.StatusError, provider.span.status)
	require.Equal(t, "服务维护中", provider.span.fields["response_message"])
	require.Equal(t, -30, provider.span.fields["response_code"])
	require.Contains(t, provider.span.errors, "panic: boom")
	require.True(t, provider.span.ended)
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
