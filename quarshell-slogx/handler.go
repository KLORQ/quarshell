package slogx

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
)

// DynamicHandler is a slog.Handler with runtime reconfiguration, OTel context
// injection, and stack traces on errors.
type DynamicHandler struct {
	cfg *atomic.Pointer[Config]

	attrs  []slog.Attr
	groups []string

	cachedHandler       atomic.Pointer[slog.Handler]
	cachedConfigVersion atomic.Pointer[Config]
}

// Enabled reports whether the record should be logged.
func (h *DynamicHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.cfg.Load().Level
}

// Handle processes a log record using a cached static handler chain.
func (h *DynamicHandler) Handle(ctx context.Context, r slog.Record) error {
	cfg := h.cfg.Load()

	appendStackAttr(&r, cfg)

	var ctxAttrs []slog.Attr
	for _, key := range cfg.ContextKeys {
		if val := ctx.Value(key); val != nil {
			ctxAttrs = append(ctxAttrs, slog.Any(key.String(), val))
		}
	}
	for _, ta := range traceAttrs(ctx, cfg) {
		ctxAttrs = append(ctxAttrs, slog.Any(ta.key, ta.val))
	}

	base := h.getOrBuildCachedHandler(cfg)
	if len(ctxAttrs) > 0 {
		base = base.WithAttrs(ctxAttrs)
	}
	return base.Handle(ctx, r)
}

func (h *DynamicHandler) getOrBuildCachedHandler(cfg *Config) slog.Handler {
	if h.cachedConfigVersion.Load() == cfg {
		if cached := h.cachedHandler.Load(); cached != nil {
			return *cached
		}
	}

	hOpts := &slog.HandlerOptions{
		Level:       cfg.Level,
		AddSource:   cfg.AddSource,
		ReplaceAttr: h.getReplaceAttr(cfg),
	}

	var base slog.Handler
	if cfg.Format == FormatJSON {
		base = slog.NewJSONHandler(cfg.Output, hOpts)
	} else {
		base = slog.NewTextHandler(cfg.Output, hOpts)
	}

	if len(h.attrs) > 0 {
		base = base.WithAttrs(h.attrs)
	}
	for _, g := range h.groups {
		base = base.WithGroup(g)
	}

	h.cachedHandler.Store(&base)
	h.cachedConfigVersion.Store(cfg)
	return base
}

// WithAttrs returns a new DynamicHandler with additional attributes.
func (h *DynamicHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &DynamicHandler{
		cfg:    h.cfg,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new DynamicHandler with an additional group.
func (h *DynamicHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &DynamicHandler{
		cfg:    h.cfg,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

func (h *DynamicHandler) getReplaceAttr(cfg *Config) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		fullKey := a.Key
		if len(groups) > 0 {
			fullKey = strings.Join(append(append([]string{}, groups...), a.Key), ".")
		}
		nk := normalizeKey(fullKey)
		if _, shouldRemove := cfg.RemoveKeys[nk]; shouldRemove {
			return slog.Attr{}
		}
		if _, shouldRemove := cfg.RemoveKeys[normalizeKey(a.Key)]; shouldRemove {
			return slog.Attr{}
		}
		if mType, ok := lookupMask(cfg.MaskKeys, fullKey); ok {
			a.Value = slog.AnyValue(cfg.Masker.Mask(a.Value.Any(), mType))
			return a
		}
		// Corporate auto-detect on string values even without an explicit key rule.
		if _, isCorp := cfg.Masker.(*CorporateMasker); isCorp {
			if s, ok := a.Value.Any().(string); ok {
				if detected, hit := detectSecret(s); hit {
					a.Value = slog.AnyValue(cfg.Masker.Mask(s, detected))
					return a
				}
			}
		}
		if a.Key == slog.LevelKey {
			if lvl, ok := a.Value.Any().(slog.Level); ok {
				a.Value = slog.StringValue(getLevelName(lvl, cfg.LevelNames))
			}
		}
		return a
	}
}
