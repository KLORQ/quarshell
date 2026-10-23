// Package credential defines scoped secret access. It never exposes secret
// material to the workload environment.
package credential

import (
	"context"
	"regexp"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// AuthorizedEndpoint is produced by the policy enforcement path, never from
// client JSON.
type AuthorizedEndpoint struct {
	Scheme   string
	Host     string
	Port     int
	Method   string
	Path     string
	Provider domain.ProviderID
}

// CredentialLease is a scoped, time-bounded handle. Handle is opaque and is not
// the secret value.
type CredentialLease struct {
	Handle           string
	Workspace        domain.WorkspaceID
	Sandbox          domain.SandboxID
	Instance         domain.InstanceID
	Provider         domain.ProviderID
	AttachmentEpoch  string
	Credential       domain.CredentialID
	PolicyGeneration domain.Generation
	Scope            AuthorizedEndpoint
	ExpiresAt        time.Time
}

// Valid reports whether the lease has all required security context.
func (l CredentialLease) Valid(now time.Time) bool {
	return l.Handle != "" &&
		l.Workspace.Valid() && l.Sandbox.Valid() && l.Instance.Valid() &&
		l.Provider.Valid() && l.Credential.Valid() && l.AttachmentEpoch != "" &&
		l.PolicyGeneration > 0 && now.Before(l.ExpiresAt)
}

// SecretValue holds secret bytes. The value is intentionally not marshalable
// through fmt or JSON and is redacted by String.
type SecretValue struct {
	value []byte
}

// NewSecretValue copies the secret bytes.
func NewSecretValue(value []byte) SecretValue {
	copied := make([]byte, len(value))
	copy(copied, value)
	return SecretValue{value: copied}
}

// Reveal returns a copy of the secret bytes for the transport layer only.
func (s SecretValue) Reveal() []byte {
	copied := make([]byte, len(s.value))
	copy(copied, s.value)
	return copied
}

// String implements fmt.Stringer with redaction.
func (s SecretValue) String() string { return "[REDACTED]" }

// GoString implements fmt.GoStringer with redaction.
func (s SecretValue) GoString() string { return "credential.SecretValue{[REDACTED]}" }

// MarshalText refuses to serialize secrets.
func (s SecretValue) MarshalText() ([]byte, error) {
	return nil, domain.New(domain.CodePermissionDenied, "secret.marshal", "secret values cannot be serialized")
}

// SecretBroker resolves a lease to a secret for one authorized request.
type SecretBroker interface {
	ResolveForRequest(context.Context, CredentialLease, AuthorizedEndpoint) (SecretValue, error)
	Revoke(context.Context, string) error
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|basic)\s+)\S+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password|credential)\s*[=:]\s*)[^\s,;]+`),
}

// Redact removes common secret-bearing fragments from a diagnostic string.
// It is a last line of defense, not a substitute for not logging secrets.
func Redact(message string) string {
	for _, pattern := range secretPatterns {
		message = pattern.ReplaceAllString(message, "${1}[REDACTED]")
	}
	return message
}
