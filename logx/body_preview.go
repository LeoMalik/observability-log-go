package logx

import (
	"bytes"
	"encoding/json"
	"strings"
)

// BodyPreviewOptions controls body preview generation and redaction.
type BodyPreviewOptions struct {
	MaxBytes   int
	RedactKeys []string
}

var defaultRedactKeys = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"password",
	"passwd",
	"secret",
	"token",
	"access_token",
	"refresh_token",
	"api_token",
	"api_key",
}

// BuildBodyPreview returns sanitized response body preview with truncation metadata.
func BuildBodyPreview(body []byte, opts BodyPreviewOptions) (preview string, truncated bool, bodySize int) {
	bodySize = len(body)
	if bodySize == 0 {
		return "", false, 0
	}

	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 2048
	}
	redactKeys := opts.RedactKeys
	if len(redactKeys) == 0 {
		redactKeys = defaultRedactKeys
	}

	sanitized := sanitizeBody(body, redactKeys)
	truncated = len(sanitized) > maxBytes
	if truncated {
		sanitized = sanitized[:maxBytes]
	}
	return string(bytes.ToValidUTF8(sanitized, []byte("?"))), truncated, bodySize
}

func sanitizeBody(body []byte, redactKeys []string) []byte {
	if !json.Valid(body) {
		return body
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return body
	}
	redactSet := buildRedactSet(redactKeys)
	sanitizeJSONValue(value, redactSet)

	redacted, err := json.Marshal(value)
	if err != nil {
		return body
	}
	return redacted
}

func sanitizeJSONValue(value any, redactSet map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if shouldRedactKey(key, redactSet) {
				typed[key] = "***"
				continue
			}
			sanitizeJSONValue(nested, redactSet)
		}
	case []any:
		for _, item := range typed {
			sanitizeJSONValue(item, redactSet)
		}
	}
}

func buildRedactSet(keys []string) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}

func shouldRedactKey(key string, redactSet map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	if _, ok := redactSet[normalized]; ok {
		return true
	}
	for candidate := range redactSet {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}
