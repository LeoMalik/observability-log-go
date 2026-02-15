package otelx

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultOTLPEndpoint = "signoz-otel-collector:4317"

var httpClientInstrumentationOnce sync.Once

// Init initializes OpenTelemetry provider with OTLP exporter.
func Init(ctx context.Context, fallbackServiceName string) (func(context.Context) error, error) {
	initHTTPClientInstrumentation()

	endpoint, insecure := parseOTLPEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultOTLPEndpoint
		insecure = true
	}

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = fallbackServiceName
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func initHTTPClientInstrumentation() {
	httpClientInstrumentationOnce.Do(func() {
		if _, ok := http.DefaultTransport.(*otelhttp.Transport); ok {
			return
		}
		http.DefaultTransport = otelhttp.NewTransport(http.DefaultTransport)
		http.DefaultClient.Transport = http.DefaultTransport
	})
}

func parseOTLPEndpoint(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Host != "" {
			return parsed.Host, parsed.Scheme == "http"
		}
	}

	return raw, true
}
