package slogx

import "log/slog"

// Dump serializes v into a slog attribute using saferefl with cycle detection.
func Dump(key string, v any, opts ...DumpOption) slog.Attr {
	return DumpWith(nil, key, v, opts...)
}

// DumpWith serializes v using configuration from l when available.
func DumpWith(l *Logger, key string, v any, opts ...DumpOption) slog.Attr {
	cfg := loadDumpConfig(l)
	return slog.Any(key, Serialize(cfg, v, opts...))
}

// DumpGroup wraps a serialized value as a slog group attribute.
func DumpGroup(key string, v any, opts ...DumpOption) slog.Attr {
	return DumpGroupWith(nil, key, v, opts...)
}

// DumpGroupWith serializes v as a group using configuration from l when available.
func DumpGroupWith(l *Logger, key string, v any, opts ...DumpOption) slog.Attr {
	cfg := loadDumpConfig(l)
	val := Serialize(cfg, v, opts...)
	if m, ok := val.(map[string]any); ok {
		attrs := make([]any, 0, len(m)*2)
		for k, v := range m {
			attrs = append(attrs, k, v)
		}
		return slog.Group(key, attrs...)
	}
	return slog.Any(key, val)
}
