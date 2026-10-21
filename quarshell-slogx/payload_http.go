package slogx

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const attrMethod = "method"

var defaultSensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"x-auth-token":        {},
	"proxy-authorization": {},
}

// HTTPRequest logs a sanitized HTTP request snapshot.
func HTTPRequest(name string, r *http.Request, opts ...DumpOption) slog.Attr {
	return HTTPRequestWith(nil, name, r, opts...)
}

// HTTPRequestWith logs a sanitized HTTP request using logger dump settings.
func HTTPRequestWith(l *Logger, name string, r *http.Request, opts ...DumpOption) slog.Attr {
	if r == nil {
		return slog.String(name, "nil")
	}
	cfg := loadDumpConfig(l)
	body, truncated := readBody(r.Body, cfg.DumpMaxFields*256)
	RestoreBody(r, body)
	payload := map[string]any{
		attrMethod: r.Method,
		"url":      r.URL.String(),
		"headers":  sanitizeHeaders(r.Header, cfg),
	}
	if body != "" {
		payload["body"] = body
	}
	if truncated {
		payload["body_truncated"] = true
	}
	return slog.Any(name, Serialize(cfg, payload, opts...))
}

// HTTPResponse logs a sanitized HTTP response snapshot.
func HTTPResponse(name string, r *http.Response, opts ...DumpOption) slog.Attr {
	return HTTPResponseWith(nil, name, r, opts...)
}

// HTTPResponseWith logs a sanitized HTTP response using logger dump settings.
func HTTPResponseWith(l *Logger, name string, r *http.Response, opts ...DumpOption) slog.Attr {
	if r == nil {
		return slog.String(name, "nil")
	}
	cfg := loadDumpConfig(l)
	body, truncated := readBody(r.Body, cfg.DumpMaxFields*256)
	payload := map[string]any{
		"status":  r.Status,
		"headers": sanitizeHeaders(r.Header, cfg),
	}
	if body != "" {
		payload["body"] = body
	}
	if truncated {
		payload["body_truncated"] = true
	}
	return slog.Any(name, Serialize(cfg, payload, opts...))
}

func sanitizeHeaders(h http.Header, cfg Config) map[string]any {
	out := make(map[string]any, len(h))
	masker := cfg.Masker
	if masker == nil {
		masker = &DefaultMasker{}
	}
	for key, vals := range h {
		lower := strings.ToLower(key)
		nk := normalizeKey(key)
		if _, sensitive := defaultSensitiveHeaders[lower]; sensitive {
			out[key] = masker.Mask(strings.Join(vals, ","), MaskSecret)
			continue
		}
		if mType, ok := lookupMask(cfg.MaskKeys, key); ok {
			masked := make([]string, len(vals))
			for i, v := range vals {
				masked[i] = fmt.Sprint(masker.Mask(v, mType))
			}
			out[key] = masked
			continue
		}
		if _, remove := cfg.RemoveKeys[nk]; remove {
			continue
		}
		if _, remove := cfg.RemoveKeys[lower]; remove {
			continue
		}
		joined := vals
		if _, isCorp := masker.(*CorporateMasker); isCorp {
			masked := make([]string, len(vals))
			changed := false
			for i, v := range vals {
				if detected, hit := detectSecret(v); hit {
					masked[i] = fmt.Sprint(masker.Mask(v, detected))
					changed = true
				} else {
					masked[i] = v
				}
			}
			if changed {
				out[key] = masked
				continue
			}
		}
		if len(joined) == 1 {
			out[key] = joined[0]
		} else {
			out[key] = joined
		}
	}
	return out
}

func readBody(body io.ReadCloser, limit int) (string, bool) {
	if body == nil {
		return "", false
	}
	if limit <= 0 {
		limit = 4096
	}
	data, err := io.ReadAll(io.LimitReader(body, int64(limit+1)))
	if err != nil {
		return "", false
	}
	truncated := len(data) > limit
	if truncated {
		data = data[:limit]
	}
	_ = body.Close()
	return string(data), truncated
}

// RestoreBody replaces r.Body after logging so downstream handlers can still read it.
func RestoreBody(r *http.Request, body string) {
	if r == nil {
		return
	}
	r.Body = io.NopCloser(bytes.NewBufferString(body))
}
