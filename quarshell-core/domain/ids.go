package domain

import "strings"

// WorkspaceID identifies a workspace. The zero value is invalid.
type WorkspaceID string

// SandboxID identifies a sandbox within a workspace. The zero value is invalid.
type SandboxID string

// InstanceID identifies one runtime instance of a sandbox generation.
type InstanceID string

// OperationID identifies a durable long-running operation.
type OperationID string

// ProviderID identifies an installed provider profile.
type ProviderID string

// CredentialID identifies credential metadata, never a secret value.
type CredentialID string

// PolicyRevision is a monotonically increasing policy document revision.
type PolicyRevision uint64

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id WorkspaceID) Valid() bool { return validID(string(id)) }

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id SandboxID) Valid() bool { return validID(string(id)) }

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id InstanceID) Valid() bool { return validID(string(id)) }

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id OperationID) Valid() bool { return validID(string(id)) }

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id ProviderID) Valid() bool { return validID(string(id)) }

// Valid reports whether the identifier is non-empty and contains no whitespace.
func (id CredentialID) Valid() bool { return validID(string(id)) }

// String returns the identifier as a plain string.
func (id WorkspaceID) String() string { return string(id) }

// String returns the identifier as a plain string.
func (id SandboxID) String() string { return string(id) }

// String returns the identifier as a plain string.
func (id InstanceID) String() string { return string(id) }

// String returns the identifier as a plain string.
func (id OperationID) String() string { return string(id) }

// String returns the identifier as a plain string.
func (id ProviderID) String() string { return string(id) }

// String returns the identifier as a plain string.
func (id CredentialID) String() string { return string(id) }

func validID(value string) bool {
	if value == "" {
		return false
	}
	return !strings.ContainsAny(value, " \t\r\n")
}

// SandboxRef is a stable reference to a sandbox.
type SandboxRef struct {
	Workspace WorkspaceID
	Sandbox   SandboxID
}

// Valid reports whether both identifiers are valid.
func (ref SandboxRef) Valid() bool { return ref.Workspace.Valid() && ref.Sandbox.Valid() }

// InstanceRef binds a sandbox reference to a concrete instance and generation.
type InstanceRef struct {
	SandboxRef
	Instance   InstanceID
	Generation Generation
}

// Valid reports whether the whole reference is complete.
func (ref InstanceRef) Valid() bool {
	return ref.SandboxRef.Valid() && ref.Instance.Valid() && ref.Generation > 0
}
