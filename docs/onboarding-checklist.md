# Service Onboarding Checklist (Go)

1. 启用 OpenTelemetry TracerProvider（含 OTLP exporter）。
2. 入站 HTTP 中间件统一创建/提取 trace，并在响应头返回 `X-Trace-Id`。
3. 出站 HTTP 统一走公共 client，自动注入 `traceparent`。
4. MQ 发布/消费统一注入与提取 trace headers。
5. 业务日志统一使用 `logx.MarshalEventJSON(...)` 输出结构化字段。
6. 在关键路径仅补 3-5 个业务语义 span。
7. 在 SigNoz 使用 `trace_id + method_name` 验证链路可检索。

