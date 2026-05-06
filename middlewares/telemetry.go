package middlewares

import (
	"time"

	"github.com/wonli/aqi/telemetry"
	"github.com/wonli/aqi/ws"
)

// Telemetry creates one span per action request.
func Telemetry() ws.HandlerFunc {
	return func(a *ws.Context) {
		startedAt := time.Now()

		ctx, span := telemetry.Start(a.Context(), a.Action, telemetry.Fields{
			"action":     a.Action,
			"request_id": a.Id,
			"client_id":  clientID(a),
			"app_id":     appID(a),
			"platform":   platform(a),
		})
		a.WithContext(ctx)

		defer func() {
			fields := telemetry.Fields{
				"duration_ms": time.Since(startedAt).Milliseconds(),
			}

			if a.Response != nil {
				fields["response_code"] = a.Response.Code
				if a.Response.Action != "" {
					fields["response_action"] = a.Response.Action
				}
				if a.Response.Msg != "" {
					fields["response_message"] = a.Response.Msg
				}
			} else {
				fields["response_sent"] = false
			}

			span.SetFields(fields)

			switch {
			case a.Response == nil:
				span.SetStatus(telemetry.StatusUnset, "")
			case a.Response.Code == 0:
				span.SetStatus(telemetry.StatusOK, "")
			default:
				span.SetStatus(telemetry.StatusError, a.Response.Msg)
			}

			span.End()
		}()

		a.Next()
	}
}

func clientID(a *ws.Context) string {
	if a == nil || a.Client == nil {
		return ""
	}

	return a.Client.ClientId
}

func appID(a *ws.Context) string {
	if a == nil || a.Client == nil {
		return ""
	}

	return a.Client.AppId
}

func platform(a *ws.Context) string {
	if a == nil || a.Client == nil {
		return ""
	}

	return a.Client.Platform
}
