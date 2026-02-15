package mqtrace

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type amqpHeaderCarrier struct {
	headers amqp.Table
}

func (c amqpHeaderCarrier) Get(key string) string {
	if c.headers == nil {
		return ""
	}
	value, ok := c.headers[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (c amqpHeaderCarrier) Set(key, value string) {
	if c.headers == nil {
		return
	}
	c.headers[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// Inject injects current trace context into RabbitMQ headers.
func Inject(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier{headers: headers})
	return headers
}

// Extract extracts trace context from RabbitMQ headers.
func Extract(ctx context.Context, headers amqp.Table) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier{headers: headers})
}

var _ propagation.TextMapCarrier = amqpHeaderCarrier{}
