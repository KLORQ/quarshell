package credentials

import (
	"context"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// Secret is one credential material and its binding metadata.
type Secret struct {
	ID        domain.CredentialID
	Provider  domain.ProviderID
	Material  []byte
	Endpoint  EndpointBinding
	ExpiresAt time.Time
	Revision  uint64
}

// EndpointBinding restricts where a credential may be used.
type EndpointBinding struct {
	Scheme     string `json:"scheme"`
	Host       string `json:"host"`
	Port       uint16 `json:"port"`
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty"`
}

// Store persists encrypted credential material. Implementations must not
// expose plaintext at rest.
type Store interface {
	Put(ctx context.Context, secret Secret) (uint64, error)
	Get(ctx context.Context, id domain.CredentialID) (Secret, error)
	Rotate(ctx context.Context, id domain.CredentialID, material []byte, expiresAt time.Time) (Secret, error)
	Delete(ctx context.Context, id domain.CredentialID) error
}
