# observability-log-go

Go二方观测日志库，目标是统一团队日志结构并自动关联 OpenTelemetry Trace。

## 主要模块

- `logx`: 结构化日志字段、JSON 序列化、响应体预览（限长+脱敏）
- `spanx`: 常用 span 操作封装（Start/EndWithError/RecordError/SetAttributes）
- `otelx`: OTel 初始化（OTLP exporter + propagator + 默认 HTTP transport）
- `gfmiddleware`: GoFrame 入站 trace + access log 中间件
- `gclientx`: GoFrame gclient 出站 trace 注入与耗时日志
- `mqtrace`: RabbitMQ headers trace 透传/提取

## 响应体预览（可选）

- 通过 `gfmiddleware` / `gclientx` 的 `EnableResponseBodyPreview` 开启
- 默认最大长度 `2048` 字节，可通过 `ResponseBodyPreviewMaxBytes` 调整
- 支持路径/URL白名单，避免全量采集
- 内置敏感字段脱敏（如 `authorization`、`token`、`password`、`cookie`）

## 统一字段

- `application_name`
- `method_name`
- `detail`
- `time`
- `level`
- `trace_id`（存在有效 span 时自动注入）
- `span_id`（存在有效 span 时自动注入）

## 用法

```go
package main

import (
    "context"
    "fmt"
    "gitlab.local/leowayne/observability-log-go.git/logx"
)

func main() {
    line, _ := logx.MarshalEventJSON(
        context.Background(),
        "anotherme-api",
        "Campaign.Create",
        "campaign created",
        "info",
        map[string]any{"campaign_id": 123},
    )
    fmt.Println(line)
}
```

## 设计目标

- 让日志字段在 Go 服务内保持一致。
- 减少业务代码手写日志格式。
- 可被 SigNoz/ClickHouse 直接按 `trace_id`、`method_name`、`time` 查询。

