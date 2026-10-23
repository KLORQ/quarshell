// Package extension defines versioned, authenticated plugin boundaries.
package extension

import "context"

// ProtocolVersion is the required extension protocol version.
const ProtocolVersion = "v1"

// Identity is the authenticated identity of an extension process.
type Identity struct {
	Name    string
	Version string
	Issuer  string
	Subject string
}

// Manifest describes an extension before it is invoked.
type Manifest struct {
	Identity   Identity
	Protocol   string
	Operations []string
}

// Request is one bounded extension call.
type Request struct {
	Operation string
	Payload   []byte
	Metadata  map[string]string
}

// Response is one bounded extension result.
type Response struct {
	Payload []byte
	Headers map[string]string
}

// Extension is a remote process reached over authenticated gRPC/UDS or TCP.
// Implementations enforce deadlines, frame limits, and version handshake.
type Extension interface {
	Manifest(context.Context) (Manifest, error)
	Invoke(context.Context, Request) (Response, error)
	Close(context.Context) error
}
