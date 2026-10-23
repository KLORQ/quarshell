package slogx

import (
	"io"
	"log/slog"
	"maps"
	"os"
	"strings"
)

// Format defines the output format for the logger.
type Format int

// RemoveMap is a set of attribute keys excluded from logs.
type RemoveMap map[string]struct{}

const (
	// FormatText is human-readable key=value output.
	FormatText Format = iota
	// FormatJSON is structured JSON output.
	FormatJSON
)

// Config holds the atomic logger configuration state.
type Config struct {
	Level       slog.Level
	Format      Format
	Output      io.Writer
	MaskKeys    MaskMap
	RemoveKeys  RemoveMap
	LevelNames  LevelNames
	Masker      Masker
	ContextKeys []ContextKey

	AddSource      bool
	StackOnError   bool
	StackDepth     int
	TraceContext   bool
	TraceIDKey     string
	SpanIDKey      string
	TraceSampleKey string

	DumpMaxDepth  int
	DumpMaxFields int
}

// Clone creates a deep copy of Config for copy-on-write updates.
func (c *Config) Clone() *Config {
	newCfg := *c

	newCfg.MaskKeys = make(MaskMap, len(c.MaskKeys))
	maps.Copy(newCfg.MaskKeys, c.MaskKeys)

	newCfg.RemoveKeys = make(RemoveMap, len(c.RemoveKeys))
	maps.Copy(newCfg.RemoveKeys, c.RemoveKeys)

	newCfg.LevelNames = make(LevelNames, len(c.LevelNames))
	maps.Copy(newCfg.LevelNames, c.LevelNames)

	newCfg.ContextKeys = make([]ContextKey, len(c.ContextKeys))
	copy(newCfg.ContextKeys, c.ContextKeys)

	return &newCfg
}

type options struct {
	initialConfig *Config
	exitFunc      func(int)
}

// Option configures logger initialization.
type Option func(*options)

// WithOutput sets the output destination.
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.initialConfig.Output = w
		}
	}
}

// WithFormat sets the log output format.
func WithFormat(f Format) Option {
	return func(o *options) {
		o.initialConfig.Format = f
	}
}

// ParseFormat converts a string to Format. Defaults to FormatText.
func ParseFormat(s string) Format {
	if strings.ToLower(s) == "json" {
		return FormatJSON
	}
	return FormatText
}

// WithLevel sets the initial logging threshold.
func WithLevel(l slog.Level) Option {
	return func(o *options) {
		o.initialConfig.Level = l
	}
}

// WithMaskKey associates a single attribute key with a MaskType.
func WithMaskKey(key string, mType MaskType) Option {
	return func(o *options) {
		o.initialConfig.MaskKeys[normalizeKey(key)] = mType
	}
}

// WithMaskKeys applies a batch of masking rules.
func WithMaskKeys(keys MaskMap) Option {
	return func(o *options) {
		for k, v := range keys {
			o.initialConfig.MaskKeys[normalizeKey(k)] = v
		}
	}
}

// WithMaskRules applies masking rules from a MaskRules builder.
func WithMaskRules(r *MaskRules) Option {
	return func(o *options) {
		if r == nil {
			return
		}
		for k, v := range r.rules {
			o.initialConfig.MaskKeys[normalizeKey(k)] = v
		}
	}
}

// WithCorporateMasking installs CorporateMasker plus CorporateMaskRules.
func WithCorporateMasking() Option {
	return func(o *options) {
		o.initialConfig.Masker = &CorporateMasker{Fingerprint: false}
		maps.Copy(o.initialConfig.MaskKeys, CorporateMaskRules().Keys())
	}
}

// WithCorporateMasker sets CorporateMasker with optional fingerprinting.
func WithCorporateMasker(fingerprint bool) Option {
	return func(o *options) {
		o.initialConfig.Masker = &CorporateMasker{Fingerprint: fingerprint}
	}
}

// WithMasker replaces the default masking logic.
func WithMasker(m Masker) Option {
	return func(o *options) {
		if m != nil {
			o.initialConfig.Masker = m
		}
	}
}

// WithRemoval registers keys from a RemovalSet for removal.
func WithRemoval(set *RemovalSet) Option {
	return func(o *options) {
		if set == nil {
			return
		}
		for _, k := range set.Keys() {
			o.initialConfig.RemoveKeys[normalizeKey(k)] = struct{}{}
		}
	}
}

// WithLevelNames customizes level string labels.
func WithLevelNames(m LevelNames) Option {
	return func(o *options) {
		maps.Copy(o.initialConfig.LevelNames, m)
	}
}

// WithContextKeys registers typed keys extracted from context.Context.
func WithContextKeys(keys ...ContextKey) Option {
	return func(o *options) {
		o.initialConfig.ContextKeys = append(o.initialConfig.ContextKeys, keys...)
	}
}

// WithAddSource enables source file and line in log records.
func WithAddSource(enabled bool) Option {
	return func(o *options) {
		o.initialConfig.AddSource = enabled
	}
}

// WithStackOnError attaches a stack trace to Error and Fatal records.
func WithStackOnError(enabled bool) Option {
	return func(o *options) {
		o.initialConfig.StackOnError = enabled
	}
}

// WithStackDepth sets how many stack frames to capture on errors.
func WithStackDepth(depth int) Option {
	return func(o *options) {
		if depth > 0 {
			o.initialConfig.StackDepth = depth
		}
	}
}

// WithTraceContext enables OpenTelemetry trace_id and span_id extraction.
func WithTraceContext(enabled bool) Option {
	return func(o *options) {
		o.initialConfig.TraceContext = enabled
	}
}

// WithTraceKeys customizes OTel field names in log output.
func WithTraceKeys(traceID, spanID, traceSampled string) Option {
	return func(o *options) {
		if traceID != "" {
			o.initialConfig.TraceIDKey = traceID
		}
		if spanID != "" {
			o.initialConfig.SpanIDKey = spanID
		}
		if traceSampled != "" {
			o.initialConfig.TraceSampleKey = traceSampled
		}
	}
}

// WithDumpLimits sets default depth and field limits for Dump and payload helpers.
func WithDumpLimits(maxDepth, maxFields int) Option {
	return func(o *options) {
		if maxDepth > 0 {
			o.initialConfig.DumpMaxDepth = maxDepth
		}
		if maxFields > 0 {
			o.initialConfig.DumpMaxFields = maxFields
		}
	}
}

// WithExitFunc overrides the process exit function used by Fatal.
func WithExitFunc(fn func(int)) Option {
	return func(o *options) {
		if fn != nil {
			o.exitFunc = fn
		}
	}
}

func defaultOptions() *options {
	ln := make(LevelNames, len(defaultLevelNames))
	for k, v := range defaultLevelNames {
		ln[k.Level()] = v
	}

	return &options{
		exitFunc: os.Exit,
		initialConfig: &Config{
			Level:          slog.LevelInfo,
			Format:         FormatText,
			Output:         os.Stdout,
			MaskKeys:       make(MaskMap),
			RemoveKeys:     make(RemoveMap),
			LevelNames:     ln,
			Masker:         &DefaultMasker{},
			StackDepth:     32,
			TraceIDKey:     "trace_id",
			SpanIDKey:      "span_id",
			TraceSampleKey: "trace_sampled",
			DumpMaxDepth:   5,
			DumpMaxFields:  64,
		},
	}
}
