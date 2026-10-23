// Package core provides shared identity and error primitives for Quarshell.
package core

// ID is an opaque sandbox or resource identifier.
type ID string

var ErrNotImplemented = errNotImplemented{}

type errNotImplemented struct{}

func (errNotImplemented) Error() string { return "quarshell-core: not implemented" }
