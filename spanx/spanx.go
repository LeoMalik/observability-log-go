package spanx

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Start creates a span with normalized map-based attributes.
func Start(ctx context.Context, tracerName, spanName string, kind trace.SpanKind, attrs map[string]any) (context.Context, trace.Span) {
	options := []trace.SpanStartOption{
		trace.WithSpanKind(kind),
	}
	if len(attrs) > 0 {
		options = append(options, trace.WithAttributes(toAttributes(attrs)...))
	}
	return otel.Tracer(tracerName).Start(ctx, spanName, options...)
}

// StartInternal creates an internal span.
func StartInternal(ctx context.Context, tracerName, spanName string, attrs map[string]any) (context.Context, trace.Span) {
	return Start(ctx, tracerName, spanName, trace.SpanKindInternal, attrs)
}

// StartServer creates a server span.
func StartServer(ctx context.Context, tracerName, spanName string, attrs map[string]any) (context.Context, trace.Span) {
	return Start(ctx, tracerName, spanName, trace.SpanKindServer, attrs)
}

// StartClient creates a client span.
func StartClient(ctx context.Context, tracerName, spanName string, attrs map[string]any) (context.Context, trace.Span) {
	return Start(ctx, tracerName, spanName, trace.SpanKindClient, attrs)
}

// StartProducer creates a producer span.
func StartProducer(ctx context.Context, tracerName, spanName string, attrs map[string]any) (context.Context, trace.Span) {
	return Start(ctx, tracerName, spanName, trace.SpanKindProducer, attrs)
}

// StartConsumer creates a consumer span.
func StartConsumer(ctx context.Context, tracerName, spanName string, attrs map[string]any) (context.Context, trace.Span) {
	return Start(ctx, tracerName, spanName, trace.SpanKindConsumer, attrs)
}

// SetAttributes sets attributes from map onto a span.
func SetAttributes(span trace.Span, attrs map[string]any) {
	if span == nil || len(attrs) == 0 {
		return
	}
	span.SetAttributes(toAttributes(attrs)...)
}

// RecordError records error and marks span error status.
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetOK marks span as successful.
func SetOK(span trace.Span) {
	if span == nil {
		return
	}
	span.SetStatus(codes.Ok, "")
}

// EndWithError finalizes span status from error pointer.
func EndWithError(span trace.Span, errp *error) {
	if span == nil {
		return
	}
	if errp != nil && *errp != nil {
		RecordError(span, *errp)
	} else {
		SetOK(span)
	}
	span.End()
}

// Run executes a callback within a span and returns callback error.
func Run(ctx context.Context, tracerName, spanName string, kind trace.SpanKind, attrs map[string]any, fn func(context.Context) error) (err error) {
	ctx, span := Start(ctx, tracerName, spanName, kind, attrs)
	defer EndWithError(span, &err)
	err = fn(ctx)
	return err
}

func toAttributes(attrs map[string]any) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		if key == "" || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			result = append(result, attribute.String(key, v))
		case bool:
			result = append(result, attribute.Bool(key, v))
		case int:
			result = append(result, attribute.Int(key, v))
		case int8:
			result = append(result, attribute.Int(key, int(v)))
		case int16:
			result = append(result, attribute.Int(key, int(v)))
		case int32:
			result = append(result, attribute.Int(key, int(v)))
		case int64:
			result = append(result, attribute.Int64(key, v))
		case uint:
			result = append(result, attribute.Int64(key, int64(v)))
		case uint8:
			result = append(result, attribute.Int64(key, int64(v)))
		case uint16:
			result = append(result, attribute.Int64(key, int64(v)))
		case uint32:
			result = append(result, attribute.Int64(key, int64(v)))
		case uint64:
			result = append(result, attribute.Int64(key, int64(v)))
		case float32:
			result = append(result, attribute.Float64(key, float64(v)))
		case float64:
			result = append(result, attribute.Float64(key, v))
		default:
			result = append(result, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
	return result
}
