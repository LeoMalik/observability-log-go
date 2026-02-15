package logx

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Event is the standard structured log payload shared by services.
type Event struct {
	ApplicationName string         `json:"application_name"`
	MethodName      string         `json:"method_name"`
	Detail          string         `json:"detail"`
	Time            string         `json:"time"`
	Level           string         `json:"level"`
	TraceID         string         `json:"trace_id,omitempty"`
	SpanID          string         `json:"span_id,omitempty"`
	Fields          map[string]any `json:"-"`
}

// BuildEvent creates a normalized log event and auto-injects trace/span IDs from context.
func BuildEvent(ctx context.Context, applicationName, methodName, detail, level string, fields map[string]any) map[string]any {
	event := map[string]any{
		"application_name": applicationName,
		"method_name":      methodName,
		"detail":           detail,
		"time":             time.Now().UTC().Format(time.RFC3339Nano),
		"level":            level,
	}

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		event["trace_id"] = sc.TraceID().String()
		event["span_id"] = sc.SpanID().String()
	}

	for k, v := range fields {
		event[k] = v
	}

	return event
}

// MarshalEventJSON returns the canonical JSON line for logging sinks.
func MarshalEventJSON(ctx context.Context, applicationName, methodName, detail, level string, fields map[string]any) (string, error) {
	event := BuildEvent(ctx, applicationName, methodName, detail, level, fields)
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
