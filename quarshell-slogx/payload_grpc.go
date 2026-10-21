package slogx

import (
	"fmt"
	"log/slog"
	"strings"
)

// Metadata is a gRPC-style metadata map (key -> values).
type Metadata map[string][]string

// GRPCCall logs a sanitized gRPC request snapshot.
func GRPCCall(name, method string, req any, md Metadata, opts ...DumpOption) slog.Attr {
	return GRPCCallWith(nil, name, method, req, md, opts...)
}

// GRPCCallWith logs a sanitized gRPC request using logger dump settings.
func GRPCCallWith(l *Logger, name, method string, req any, md Metadata, opts ...DumpOption) slog.Attr {
	cfg := loadDumpConfig(l)
	payload := map[string]any{
		attrMethod: method,
		"request":  Serialize(cfg, req, opts...),
		"metadata": sanitizeMetadata(md, cfg),
	}
	return slog.Any(name, payload)
}

// GRPCResponse logs a sanitized gRPC response snapshot.
func GRPCResponse(name string, resp any, opts ...DumpOption) slog.Attr {
	return GRPCResponseWith(nil, name, resp, opts...)
}

// GRPCResponseWith logs a sanitized gRPC response using logger dump settings.
func GRPCResponseWith(l *Logger, name string, resp any, opts ...DumpOption) slog.Attr {
	cfg := loadDumpConfig(l)
	return slog.Any(name, map[string]any{
		"response": Serialize(cfg, resp, opts...),
	})
}

// GRPCStreamEvent logs a streaming gRPC event with optional metadata.
func GRPCStreamEvent(name, method, event string, msg any, md Metadata, opts ...DumpOption) slog.Attr {
	return GRPCStreamEventWith(nil, name, method, event, msg, md, opts...)
}

// GRPCStreamEventWith logs a streaming gRPC event using logger dump settings.
func GRPCStreamEventWith(l *Logger, name, method, event string, msg any, md Metadata, opts ...DumpOption) slog.Attr {
	cfg := loadDumpConfig(l)
	return slog.Any(name, map[string]any{
		attrMethod: method,
		"event":    event,
		"message":  Serialize(cfg, msg, opts...),
		"metadata": sanitizeMetadata(md, cfg),
	})
}

func sanitizeMetadata(md Metadata, cfg Config) map[string]any {
	if len(md) == 0 {
		return nil
	}
	masker := cfg.Masker
	if masker == nil {
		masker = &DefaultMasker{}
	}
	out := make(map[string]any, len(md))
	for key, vals := range md {
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
		if len(vals) == 1 {
			out[key] = vals[0]
		} else {
			out[key] = vals
		}
	}
	return out
}
