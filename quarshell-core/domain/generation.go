package domain

import "math"

// Generation fences runtime state, configuration acknowledgements, credentials,
// and durable operations. It is never reused within one sandbox.
type Generation uint64

// Next returns the next generation.
func (g Generation) Next() (Generation, error) {
	if g == math.MaxUint64 {
		return 0, Wrap(ErrConflict, "generation.next", "generation exhausted")
	}
	return g + 1, nil
}

// After reports whether g is newer than other.
func (g Generation) After(other Generation) bool { return g > other }

// Sequence orders events within one instance and generation.
type Sequence uint64

// Next returns the next sequence number.
func (s Sequence) Next() (Sequence, error) {
	if s == math.MaxUint64 {
		return 0, Wrap(ErrConflict, "sequence.next", "sequence exhausted")
	}
	return s + 1, nil
}

// After reports whether s is newer than other.
func (s Sequence) After(other Sequence) bool { return s > other }
