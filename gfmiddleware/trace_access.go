package gfmiddleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gitlab.local/leowayne/observability-log-go/logx"
	"gitlab.local/leowayne/observability-log-go/spanx"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TraceOptions configures GoFrame trace-access middleware behavior.
type TraceOptions struct {
	ApplicationName string
	TracerName      string
	MethodName      string
	TraceHeaderName string
	// EnableResponseBodyPreview enables sanitized response body preview collection.
	EnableResponseBodyPreview bool
	// ResponseBodyPreviewMaxBytes controls preview max bytes (default 2048).
	ResponseBodyPreviewMaxBytes int
	// ResponseBodyPreviewPathAllowlist limits preview to matched path prefixes.
	ResponseBodyPreviewPathAllowlist []string
	// ResponseBodyPreviewRedactKeys extends/redifines redact key list.
	ResponseBodyPreviewRedactKeys []string
	// Logger accepts level + serialized json line.
	Logger func(ctx context.Context, level, line string)
}

// NewTraceAccessMiddleware creates middleware for server-span + structured access log.
func NewTraceAccessMiddleware(opts TraceOptions) func(r *ghttp.Request) {
	tracerName := opts.TracerName
	if tracerName == "" {
		tracerName = "observability-log-go/http-server"
	}
	methodName := opts.MethodName
	if methodName == "" {
		methodName = "http.request"
	}
	traceHeaderName := opts.TraceHeaderName
	if traceHeaderName == "" {
		traceHeaderName = "X-Trace-Id"
	}

	return func(r *ghttp.Request) {
		start := time.Now()
		spanName := r.Method + " " + r.URL.Path

		extractedCtx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Request.Header))
		ctx, span := spanx.StartServer(extractedCtx, tracerName, spanName, map[string]any{
			"http_method":    r.Method,
			"http_target":    r.URL.Path,
			"http_client_ip": r.GetClientIp(),
		})
		r.SetCtx(ctx)

		r.Middleware.Next()

		statusCode := r.Response.Status
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		spanAttrs := map[string]any{
			"http_status_code":        statusCode,
			"http_server_duration_ms": durationMs,
		}
		logFields := map[string]any{
			"http_method":  r.Method,
			"http_path":    r.URL.Path,
			"http_status":  statusCode,
			"duration_ms":  durationMs,
			"client_ip":    r.GetClientIp(),
			"user_agent":   r.UserAgent(),
			"content_type": r.Header.Get("Content-Type"),
		}

		if opts.EnableResponseBodyPreview && shouldCapturePath(r.URL.Path, opts.ResponseBodyPreviewPathAllowlist) {
			requestPreview, requestTruncated, requestSize := logx.BuildBodyPreview(
				r.GetBody(),
				logx.BodyPreviewOptions{
					MaxBytes:   opts.ResponseBodyPreviewMaxBytes,
					RedactKeys: opts.ResponseBodyPreviewRedactKeys,
				},
			)
			if requestSize > 0 {
				spanAttrs["http_request_body_size"] = requestSize
				logFields["http_request_body_size"] = requestSize
			}
			if requestPreview != "" {
				spanAttrs["http_request_body_preview"] = requestPreview
				logFields["http_request_body_preview"] = requestPreview
			}
			if requestTruncated {
				spanAttrs["http_request_body_preview_truncated"] = true
				logFields["http_request_body_preview_truncated"] = true
			}

			preview, truncated, size := logx.BuildBodyPreview(
				r.Response.Buffer(),
				logx.BodyPreviewOptions{
					MaxBytes:   opts.ResponseBodyPreviewMaxBytes,
					RedactKeys: opts.ResponseBodyPreviewRedactKeys,
				},
			)
			if size > 0 {
				spanAttrs["http_response_body_size"] = size
				logFields["http_response_body_size"] = size
			}
			if preview != "" {
				spanAttrs["http_response_body_preview"] = preview
				logFields["http_response_body_preview"] = preview
			}
			if truncated {
				spanAttrs["http_response_body_preview_truncated"] = true
				logFields["http_response_body_preview_truncated"] = true
			}
		}

		spanx.SetAttributes(span, spanAttrs)
		if err := r.GetError(); err != nil {
			spanx.RecordError(span, err)
		} else {
			spanx.SetOK(span)
		}

		if traceHeaderName != "" {
			sc := span.SpanContext()
			if sc.IsValid() {
				r.Response.Header().Set(traceHeaderName, sc.TraceID().String())
			}
		}

		emit(opts, ctx, "info", methodName, "incoming request handled", logFields)
		span.End()
	}
}

func shouldCapturePath(path string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, item := range allowlist {
		candidate := strings.TrimSpace(item)
		if candidate == "" {
			continue
		}
		if path == candidate || strings.HasPrefix(path, candidate) {
			return true
		}
	}
	return false
}

func emit(opts TraceOptions, ctx context.Context, level, methodName, detail string, fields map[string]any) {
	if opts.Logger == nil {
		return
	}
	line, err := logx.MarshalEventJSON(ctx, opts.ApplicationName, methodName, detail, level, fields)
	if err != nil {
		return
	}
	opts.Logger(ctx, level, line)
}
