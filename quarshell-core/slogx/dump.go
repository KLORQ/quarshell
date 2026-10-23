package slogx

import (
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/lkmavi/saferefl"
)

const truncatedMaxFields = "[truncated: max fields]"

type dumpState struct {
	maxDepth  int
	maxFields int
	depth     int
	fields    int
	visited   map[uintptr]struct{}
	maskKeys  MaskMap
	remove    RemoveMap
	masker    Masker
}

func newDumpState(cfg *Config, opts []DumpOption) dumpState {
	o := dumpOptions{
		maxDepth:  cfg.DumpMaxDepth,
		maxFields: cfg.DumpMaxFields,
		maskKeys:  cfg.MaskKeys,
		remove:    cfg.RemoveKeys,
		masker:    cfg.Masker,
	}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	if o.masker == nil {
		o.masker = &DefaultMasker{}
	}
	return dumpState{
		maxDepth:  o.maxDepth,
		maxFields: o.maxFields,
		visited:   make(map[uintptr]struct{}),
		maskKeys:  o.maskKeys,
		remove:    o.remove,
		masker:    o.masker,
	}
}

type dumpOptions struct {
	maxDepth  int
	maxFields int
	maskKeys  MaskMap
	remove    RemoveMap
	masker    Masker
}

// DumpOption configures Dump and payload helpers.
type DumpOption func(*dumpOptions)

// WithDumpDepth overrides the maximum recursion depth.
func WithDumpDepth(depth int) DumpOption {
	return func(o *dumpOptions) {
		if depth > 0 {
			o.maxDepth = depth
		}
	}
}

// WithDumpMaxFields overrides the maximum number of fields emitted.
func WithDumpMaxFields(n int) DumpOption {
	return func(o *dumpOptions) {
		if n > 0 {
			o.maxFields = n
		}
	}
}

// WithDumpMaskKeys applies masking rules during dump serialization.
func WithDumpMaskKeys(keys MaskMap) DumpOption {
	return func(o *dumpOptions) {
		if o.maskKeys == nil {
			o.maskKeys = make(MaskMap)
		}
		for k, v := range keys {
			o.maskKeys[normalizeKey(k)] = v
		}
	}
}

// WithDumpRemoval excludes keys during dump serialization.
func WithDumpRemoval(keys ...string) DumpOption {
	return func(o *dumpOptions) {
		if o.remove == nil {
			o.remove = make(RemoveMap)
		}
		for _, k := range keys {
			o.remove[normalizeKey(k)] = struct{}{}
		}
	}
}

func (s *dumpState) nextField() bool {
	if s.maxFields <= 0 {
		return true
	}
	if s.fields >= s.maxFields {
		return false
	}
	s.fields++
	return true
}

func (s *dumpState) sanitizeKey(key string, val any) any {
	nk := normalizeKey(key)
	if _, ok := s.remove[nk]; ok {
		return nil
	}
	if _, ok := s.remove[key]; ok {
		return nil
	}
	if mType, ok := lookupMask(s.maskKeys, key); ok {
		return s.masker.Mask(val, mType)
	}
	if _, isCorp := s.masker.(*CorporateMasker); isCorp {
		if str, ok := val.(string); ok {
			if detected, hit := detectSecret(str); hit {
				return s.masker.Mask(str, detected)
			}
		}
	}
	return val
}

func (s *dumpState) value(v any) any {
	if !s.nextField() {
		return truncatedMaxFields
	}
	if v == nil {
		return nil
	}
	if s.depth >= s.maxDepth {
		return reflect.TypeOf(v).String()
	}

	kind := saferefl.KindOf(v)
	switch kind {
	case reflect.Pointer:
		if saferefl.IsNil(v) {
			return nil
		}
		ptr := pointerKey(v)
		if _, seen := s.visited[ptr]; seen {
			return "[cycle]"
		}
		s.visited[ptr] = struct{}{}
		s.depth++
		defer func() { s.depth-- }()
		return s.value(indirect(v))
	case reflect.Struct:
		return s.structValue(v)
	case reflect.Map:
		return s.mapValue(v)
	case reflect.Slice, reflect.Array:
		return s.sliceValue(v)
	default:
		return v
	}
}

func (s *dumpState) structValue(v any) map[string]any {
	out := make(map[string]any)
	target := v
	if saferefl.KindOf(v) == reflect.Struct {
		rv := reflect.ValueOf(v)
		if rv.CanAddr() {
			target = rv.Addr().Interface()
		} else {
			copy := reflect.New(rv.Type()).Elem()
			copy.Set(rv)
			target = copy.Addr().Interface()
		}
	}
	_ = saferefl.EachField(target, func(name string, val any) bool {
		if _, remove := s.remove[normalizeKey(name)]; remove {
			return true
		}
		if _, remove := s.remove[name]; remove {
			return true
		}
		if !s.nextField() {
			out["..."] = truncatedMaxFields
			return false
		}
		sanitized := s.sanitizeKey(name, val)
		if sanitized == nil && val != nil {
			return true
		}
		s.depth++
		out[name] = s.fieldValue(sanitized)
		s.depth--
		return true
	})
	return out
}

// fieldValue serializes a single field without consuming a top-level field slot.
func (s *dumpState) fieldValue(v any) any {
	if v == nil {
		return nil
	}
	if s.depth >= s.maxDepth {
		return reflect.TypeOf(v).String()
	}

	kind := saferefl.KindOf(v)
	switch kind {
	case reflect.Pointer:
		if saferefl.IsNil(v) {
			return nil
		}
		ptr := pointerKey(v)
		if _, seen := s.visited[ptr]; seen {
			return "[cycle]"
		}
		s.visited[ptr] = struct{}{}
		s.depth++
		defer func() { s.depth-- }()
		return s.fieldValue(indirect(v))
	case reflect.Struct:
		return s.structValue(v)
	case reflect.Map:
		return s.mapValue(v)
	case reflect.Slice, reflect.Array:
		return s.sliceValue(v)
	default:
		return v
	}
}

func (s *dumpState) mapValue(v any) map[string]any {
	out := make(map[string]any)
	rv := reflect.ValueOf(v)
	for iter := rv.MapRange(); iter.Next(); {
		if !s.nextField() {
			out["..."] = truncatedMaxFields
			break
		}
		key := iter.Key().Interface()
		keyStr, ok := key.(string)
		if !ok {
			keyStr = reflect.TypeOf(key).String()
		}
		if _, remove := s.remove[normalizeKey(keyStr)]; remove {
			continue
		}
		if _, remove := s.remove[keyStr]; remove {
			continue
		}
		val := iter.Value().Interface()
		sanitized := s.sanitizeKey(keyStr, val)
		if sanitized == nil && val != nil {
			continue
		}
		s.depth++
		out[keyStr] = s.fieldValue(sanitized)
		s.depth--
	}
	return out
}

func (s *dumpState) sliceValue(v any) []any {
	rv := reflect.ValueOf(v)
	n := rv.Len()
	out := make([]any, 0, n)
	for i := range n {
		if !s.nextField() {
			out = append(out, truncatedMaxFields)
			break
		}
		s.depth++
		out = append(out, s.fieldValue(rv.Index(i).Interface()))
		s.depth--
	}
	return out
}

func indirect(v any) any {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}
	return rv.Interface()
}

func pointerKey(v any) uintptr {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0
		}
		p := rv.Pointer()
		if p != 0 {
			return p
		}
		rv = rv.Elem()
	}
	e := (*eface)(unsafe.Pointer(&v))
	return uintptr(e.data)
}

type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// Serialize converts v to a log-safe representation using saferefl.
func Serialize(cfg Config, v any, opts ...DumpOption) any {
	st := newDumpState(&cfg, opts)
	return st.value(v)
}

// SerializeDefault uses Default dump limits and masking rules.
func SerializeDefault(v any, opts ...DumpOption) any {
	return Serialize(Config{
		DumpMaxDepth:  5,
		DumpMaxFields: 64,
		MaskKeys:      make(MaskMap),
		RemoveKeys:    make(RemoveMap),
		Masker:        &DefaultMasker{},
	}, v, opts...)
}

var dumpCfg atomic.Pointer[Config]

func init() {
	cfg := defaultOptions().initialConfig
	dumpCfg.Store(cfg)
}

// SetGlobalDumpConfig sets defaults used by Dump when no Logger is available.
func SetGlobalDumpConfig(cfg *Config) {
	if cfg != nil {
		dumpCfg.Store(cfg)
	}
}

func loadDumpConfig(l *Logger) Config {
	if l != nil {
		return l.Config()
	}
	if cfg := dumpCfg.Load(); cfg != nil {
		return *cfg
	}
	return Config{
		DumpMaxDepth:  5,
		DumpMaxFields: 64,
		MaskKeys:      make(MaskMap),
		RemoveKeys:    make(RemoveMap),
		Masker:        &DefaultMasker{},
	}
}
