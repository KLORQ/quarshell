package domain

// Field distinguishes an absent field, an explicit null, and a present value.
// The zero value is absent. It is deliberately not equivalent to *T, which
// cannot distinguish absent from null.
type Field[T any] struct {
	Present bool
	Null    bool
	Value   T
}

// Absent returns an absent field.
func Absent[T any]() Field[T] { return Field[T]{} }

// Null creates an explicit null field.
func Null[T any]() Field[T] { return Field[T]{Present: true, Null: true} }

// Some creates a present non-null field.
func Some[T any](value T) Field[T] { return Field[T]{Present: true, Value: value} }

// IsAbsent reports whether the field was omitted.
func (f Field[T]) IsAbsent() bool { return !f.Present && !f.Null }

// IsNull reports whether the field was explicitly null.
func (f Field[T]) IsNull() bool { return f.Present && f.Null }

// HasValue reports whether the field is present and non-null.
func (f Field[T]) HasValue() bool { return f.Present && !f.Null }

// OrElse returns the value when present and non-null, otherwise fallback.
func (f Field[T]) OrElse(fallback T) T {
	if f.HasValue() {
		return f.Value
	}
	return fallback
}

// RequireValue returns the value or a validation error when absent or null.
func (f Field[T]) RequireValue(op, name string) (T, error) {
	if !f.HasValue() {
		var zero T
		return zero, Invalidf(op, "%s must be present and non-null", name)
	}
	return f.Value, nil
}

// RejectNull returns a validation error when the field is explicitly null.
// Callers use it for object fields where upstream forbids null.
func (f Field[T]) RejectNull(op, name string) error {
	if f.IsNull() {
		return Invalidf(op, "%s must not be null", name)
	}
	return nil
}
