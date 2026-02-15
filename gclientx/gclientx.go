package gclientx

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.local/leowayne/observability-log-go/logx"
	"gitlab.local/leowayne/observability-log-go/spanx"

	"github.com/gogf/gf/v2/net/gclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Options configures gclient tracing middleware behavior.
type Options struct {
	ApplicationName string
	TracerName      string
	MethodName      string
	// EnableResponseBodyPreview enables sanitized response body preview collection.
	EnableResponseBodyPreview bool
	// ResponseBodyPreviewMaxBytes controls preview max bytes (default 2048).
	ResponseBodyPreviewMaxBytes int
	// ResponseBodyPreviewURLAllowlist limits preview to matched URL prefixes.
	ResponseBodyPreviewURLAllowlist []string
	// ResponseBodyPreviewRedactKeys extends/redefines redact key list.
	ResponseBodyPreviewRedactKeys []string
	// Logger accepts level + serialized json line.
	Logger func(ctx context.Context, level, line string)
}

// Install installs outbound tracing and structured logging for a gclient.Client.
func Install(client *gclient.Client, opts Options) {
	if client == nil {
		return
	}
	tracerName := opts.TracerName
	if tracerName == "" {
		tracerName = "observability-log-go/http-client"
	}
	methodName := opts.MethodName
	if methodName == "" {
		methodName = "http.client"
	}

	client.Use(func(c *gclient.Client, req *http.Request) (*gclient.Response, error) {
		ctx := req.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, span := spanx.StartClient(ctx, tracerName, req.Method+" "+req.URL.Host, map[string]any{
			"http_method": req.Method,
			"http_url":    req.URL.String(),
		})
		req = req.WithContext(ctx)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		var requestBody []byte
		if opts.EnableResponseBodyPreview && shouldCaptureURL(req.URL.String(), opts.ResponseBodyPreviewURLAllowlist) {
			requestBody = snapshotRequestBody(req)
		}

		start := time.Now()
		resp, err := c.Next(req)
		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		fields := map[string]any{
			"http_method": req.Method,
			"http_url":    req.URL.String(),
			"duration_ms": durationMs,
		}

		if opts.EnableResponseBodyPreview && shouldCaptureURL(req.URL.String(), opts.ResponseBodyPreviewURLAllowlist) {
			requestPreview, requestTruncated, requestSize := logx.BuildBodyPreview(
				requestBody,
				logx.BodyPreviewOptions{
					MaxBytes:   opts.ResponseBodyPreviewMaxBytes,
					RedactKeys: opts.ResponseBodyPreviewRedactKeys,
				},
			)
			if requestSize > 0 {
				fields["http_request_body_size"] = requestSize
			}
			if requestPreview != "" {
				fields["http_request_body_preview"] = requestPreview
			}
			if requestTruncated {
				fields["http_request_body_preview_truncated"] = true
			}
		}

		if resp != nil {
			fields["http_status"] = resp.StatusCode
		}

		if opts.EnableResponseBodyPreview && resp != nil && shouldCaptureURL(req.URL.String(), opts.ResponseBodyPreviewURLAllowlist) {
			body := resp.ReadAll()
			resp.SetBodyContent(body)

			preview, truncated, size := logx.BuildBodyPreview(
				body,
				logx.BodyPreviewOptions{
					MaxBytes:   opts.ResponseBodyPreviewMaxBytes,
					RedactKeys: opts.ResponseBodyPreviewRedactKeys,
				},
			)
			if size > 0 {
				fields["http_response_body_size"] = size
			}
			if preview != "" {
				fields["http_response_body_preview"] = preview
			}
			if truncated {
				fields["http_response_body_preview_truncated"] = true
			}
		}
		spanx.SetAttributes(span, fields)

		if err != nil {
			spanx.RecordError(span, err)
			emit(opts, ctx, "error", methodName, "downstream request failed", fields)
			span.End()
			return resp, err
		}

		if resp != nil && resp.StatusCode >= http.StatusInternalServerError {
			emit(opts, ctx, "warn", methodName, "downstream request returned 5xx", fields)
		} else {
			spanx.SetOK(span)
			emit(opts, ctx, "info", methodName, "downstream request completed", fields)
		}
		span.End()
		return resp, nil
	})
}

func shouldCaptureURL(url string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, item := range allowlist {
		candidate := strings.TrimSpace(item)
		if candidate == "" {
			continue
		}
		if strings.Contains(url, candidate) || strings.HasPrefix(url, candidate) {
			return true
		}
	}
	return false
}

func snapshotRequestBody(req *http.Request) []byte {
	if req == nil || req.Body == nil {
		return nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return body
}

func emit(opts Options, ctx context.Context, level, methodName, detail string, fields map[string]any) {
	if opts.Logger == nil {
		return
	}
	line, err := logx.MarshalEventJSON(ctx, opts.ApplicationName, methodName, detail, level, fields)
	if err != nil {
		return
	}
	opts.Logger(ctx, level, line)
}
