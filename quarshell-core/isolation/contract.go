// Package isolation defines the host-supervisor contract for attaching a
// sandbox to a verified isolation backend.
package isolation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	DescriptorVersion  = 1
	MaxDescriptorBytes = 1 << 20
)

var (
	namePattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,127}$`)
	identityPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// Descriptor is the common host-supervisor envelope. Payload is backend
// specific and must contain no credentials; use private file references for
// secrets so they can be permission-checked and rotated independently.
type Descriptor struct {
	Version       uint16          `json:"version"`
	BackendName   string          `json:"backend_name"`
	SandboxID     string          `json:"sandbox_id"`
	SessionID     string          `json:"session_id"`
	Generation    string          `json:"generation"`
	RootfsDigest  string          `json:"rootfs_digest"`
	RuntimeDigest string          `json:"runtime_digest"`
	Payload       json.RawMessage `json:"payload"`
}

// EncodeDescriptor produces the bounded wire representation consumed by
// Registry.Attach. It validates descriptor claims before encoding so callers
// cannot publish incomplete or malformed isolation metadata.
func EncodeDescriptor(descriptor Descriptor) ([]byte, error) {
	if descriptor.Version != DescriptorVersion {
		return nil, fmt.Errorf("unsupported isolation descriptor version %d", descriptor.Version)
	}
	if !namePattern.MatchString(descriptor.BackendName) {
		return nil, fmt.Errorf("invalid isolation backend name")
	}
	if !identityPattern.MatchString(descriptor.SandboxID) || !identityPattern.MatchString(descriptor.SessionID) || !identityPattern.MatchString(descriptor.Generation) {
		return nil, fmt.Errorf("invalid isolation sandbox, session, or generation identity")
	}
	if !digestPattern.MatchString(descriptor.RootfsDigest) || !digestPattern.MatchString(descriptor.RuntimeDigest) {
		return nil, fmt.Errorf("invalid isolation rootfs or runtime digest")
	}
	if len(descriptor.Payload) == 0 || !json.Valid(descriptor.Payload) {
		return nil, fmt.Errorf("isolation descriptor payload must be valid JSON")
	}
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return nil, fmt.Errorf("encode isolation descriptor: %w", err)
	}
	if len(encoded) > MaxDescriptorBytes {
		return nil, fmt.Errorf("isolation descriptor exceeds %d bytes", MaxDescriptorBytes)
	}
	if _, err := decodeDescriptor(encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}

// DecodeDescriptor parses one strict, bounded descriptor envelope without
// attaching it to a backend. Callers still need Registry.Attach to bind it to
// trusted sandbox context and confirm enforcement.
func DecodeDescriptor(raw []byte) (Descriptor, error) { return decodeDescriptor(raw) }

// SandboxContext is derived from trusted driver state, not from the descriptor.
type SandboxContext struct {
	SandboxID         string
	SessionID         string
	Generation        string
	RootfsDigest      string
	RuntimeDigest     string
	WorkloadUID       uint32
	WorkloadGID       uint32
	SupplementaryGIDs []uint32
	IdentitySource    string
	IdentityDigest    string
}

// VerifiedDescriptor can only be constructed by Registry.Attach after the
// descriptor has been checked against the admitted backend and trusted context.
type VerifiedDescriptor struct{ descriptor Descriptor }

func (d VerifiedDescriptor) BackendName() string   { return d.descriptor.BackendName }
func (d VerifiedDescriptor) SandboxID() string     { return d.descriptor.SandboxID }
func (d VerifiedDescriptor) SessionID() string     { return d.descriptor.SessionID }
func (d VerifiedDescriptor) Generation() string    { return d.descriptor.Generation }
func (d VerifiedDescriptor) RootfsDigest() string  { return d.descriptor.RootfsDigest }
func (d VerifiedDescriptor) RuntimeDigest() string { return d.descriptor.RuntimeDigest }
func (d VerifiedDescriptor) Payload() []byte       { return bytes.Clone(d.descriptor.Payload) }

// EnforcementEvidence is a backend's measured confirmation that the boundary
// is active before workload code can run. EvidenceDigest commits to the
// backend-native evidence; it does not replace the explicit guarantees.
type EnforcementEvidence struct {
	Generation                string
	DefaultDenyEgress         bool
	NoUnmanagedEgressPath     bool
	RevocationVerified        bool
	ControllerLossFailsClosed bool
	FilesystemConfinement     bool
	RequestAttribution        bool
	PrivilegeFloor            bool
	AuthenticatedSupervisor   bool
	RuntimeExitStopsWorkload  bool
	WorkloadUID               uint32
	WorkloadGID               uint32
	SupplementaryGIDs         []uint32
	IdentitySource            string
	IdentityDigest            string
	EvidenceDigest            string
}

func (e EnforcementEvidence) validate(expected SandboxContext) error {
	if e.Generation != expected.Generation || !e.DefaultDenyEgress || !e.NoUnmanagedEgressPath || !e.RevocationVerified || !e.ControllerLossFailsClosed || !e.FilesystemConfinement || !e.RequestAttribution || !e.PrivilegeFloor || !e.AuthenticatedSupervisor || !e.RuntimeExitStopsWorkload || e.WorkloadUID != expected.WorkloadUID || e.WorkloadGID != expected.WorkloadGID || !slices.Equal(e.SupplementaryGIDs, expected.SupplementaryGIDs) || e.IdentitySource != expected.IdentitySource || e.IdentityDigest != expected.IdentityDigest || !digestPattern.MatchString(e.EvidenceDigest) {
		return fmt.Errorf("isolation backend confirmation is incomplete or bound to another generation")
	}
	return nil
}

// Backend implements the resource-specific half of attach/confirm/start.
// Attach must not make untrusted workload code runnable.
type Backend interface {
	Name() string
	// Attach must validate the opaque payload and atomically bind this session
	// to its boundary. It must leave untrusted workload code stopped and clean
	// all partial resources before returning an error.
	Attach(context.Context, VerifiedDescriptor, SandboxContext) (Boundary, error)
}

// Boundary is attached but not confirmed. Start is intentionally unavailable
// until Confirm returns a ConfirmedBoundary.
type Boundary interface {
	Confirm(context.Context) (EnforcementEvidence, error)
	Close(context.Context) error
	// StartWorkload rechecks launch-time controls immediately before making
	// untrusted instructions runnable; a past confirmation is not sufficient.
	StartWorkload(context.Context) error
}

// ConfirmedBoundary is the only public state that can start an admitted
// workload. It owns the attached backend until Close.
type ConfirmedBoundary struct {
	boundary       Boundary
	evidence       EnforcementEvidence
	sandbox        SandboxContext
	mu             sync.Mutex
	startAttempted bool
	started        bool
	closed         bool
}

func (b *ConfirmedBoundary) Evidence() EnforcementEvidence {
	evidence := b.evidence
	evidence.SupplementaryGIDs = slices.Clone(evidence.SupplementaryGIDs)
	return evidence
}

// Start makes the admitted workload runnable after verified boundary
// confirmation. Repeated starts are rejected rather than replayed.
func (b *ConfirmedBoundary) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("isolation start context is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return fmt.Errorf("isolation boundary is closed")
	}
	if b.startAttempted {
		return fmt.Errorf("isolation workload start was already attempted")
	}
	b.startAttempted = true
	if err := b.boundary.StartWorkload(ctx); err != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		closeErr := b.boundary.Close(closeCtx)
		cancel()
		if closeErr == nil {
			b.closed = true
		}
		return errors.Join(fmt.Errorf("start workload behind isolation boundary: %w", err), closeErr)
	}
	b.started = true
	return nil
}

// Close releases an attached or running boundary. It is idempotent.
func (b *ConfirmedBoundary) Close(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("isolation close context is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	if err := b.boundary.Close(ctx); err != nil {
		return err
	}
	b.closed = true
	return nil
}

// Registry admits only explicitly registered backend names and never falls
// back when a descriptor names a different backend.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

func NewRegistry() *Registry { return &Registry{backends: make(map[string]Backend)} }

func (r *Registry) Register(backend Backend) error {
	if r == nil || backend == nil {
		return fmt.Errorf("isolation registry and backend are required")
	}
	name := strings.TrimSpace(backend.Name())
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid isolation backend name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.backends[name]; exists {
		return fmt.Errorf("isolation backend %q is already registered", name)
	}
	r.backends[name] = backend
	return nil
}

// Attach validates the bounded descriptor against both admission and trusted
// driver context before calling backend code.
func (r *Registry) Attach(ctx context.Context, raw []byte, admittedBackend string, sandbox SandboxContext) (*ConfirmedBoundary, error) {
	if ctx == nil || r == nil {
		return nil, fmt.Errorf("isolation context and registry are required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sandbox.SupplementaryGIDs = slices.Clone(sandbox.SupplementaryGIDs)
	descriptor, err := decodeDescriptor(raw)
	if err != nil {
		return nil, err
	}
	if err := validateContext(descriptor, admittedBackend, sandbox); err != nil {
		return nil, err
	}
	r.mu.RLock()
	backend := r.backends[descriptor.BackendName]
	r.mu.RUnlock()
	if backend == nil {
		return nil, fmt.Errorf("isolation backend %q is not registered", descriptor.BackendName)
	}
	if backend.Name() != descriptor.BackendName {
		return nil, fmt.Errorf("registered isolation backend identity changed")
	}
	verified := VerifiedDescriptor{descriptor: descriptor}
	boundary, err := backend.Attach(ctx, verified, sandbox)
	if err != nil {
		var cleanupErr error
		if boundary != nil {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			cleanupErr = boundary.Close(closeCtx)
			cancel()
		}
		return nil, errors.Join(fmt.Errorf("attach isolation backend %q: %w", descriptor.BackendName, err), cleanupErr)
	}
	if boundary == nil {
		return nil, fmt.Errorf("isolation backend returned no attached boundary")
	}
	evidence, err := boundary.Confirm(ctx)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		closeErr := boundary.Close(closeCtx)
		cancel()
		return nil, errors.Join(fmt.Errorf("confirm isolation backend %q: %w", descriptor.BackendName, err), closeErr)
	}
	if err := evidence.validate(sandbox); err != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		closeErr := boundary.Close(closeCtx)
		cancel()
		return nil, errors.Join(err, closeErr)
	}
	evidence.SupplementaryGIDs = slices.Clone(evidence.SupplementaryGIDs)
	return &ConfirmedBoundary{boundary: boundary, evidence: evidence, sandbox: sandbox}, nil
}

func decodeDescriptor(raw []byte) (Descriptor, error) {
	var descriptor Descriptor
	if len(raw) == 0 || len(raw) > MaxDescriptorBytes {
		return descriptor, fmt.Errorf("isolation descriptor size must be between 1 and %d bytes", MaxDescriptorBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return descriptor, fmt.Errorf("isolation descriptor must be a JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return descriptor, fmt.Errorf("decode isolation descriptor field: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return descriptor, fmt.Errorf("isolation descriptor field name is invalid")
		}
		if _, exists := fields[key]; exists {
			return descriptor, fmt.Errorf("isolation descriptor repeats field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return descriptor, fmt.Errorf("decode isolation descriptor field %q: %w", key, err)
		}
		fields[key] = value
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return descriptor, fmt.Errorf("isolation descriptor object is not closed")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return descriptor, fmt.Errorf("isolation descriptor must contain exactly one JSON object")
	}
	allowed := map[string]struct{}{"version": {}, "backend_name": {}, "sandbox_id": {}, "session_id": {}, "generation": {}, "rootfs_digest": {}, "runtime_digest": {}, "payload": {}}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return descriptor, fmt.Errorf("isolation descriptor contains unknown field %q", key)
		}
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return descriptor, fmt.Errorf("normalize isolation descriptor: %w", err)
	}
	strictDecoder := json.NewDecoder(bytes.NewReader(canonical))
	strictDecoder.DisallowUnknownFields()
	if err := strictDecoder.Decode(&descriptor); err != nil {
		return descriptor, fmt.Errorf("decode isolation descriptor: %w", err)
	}
	if descriptor.Version != DescriptorVersion {
		return descriptor, fmt.Errorf("unsupported isolation descriptor version %d", descriptor.Version)
	}
	if len(descriptor.Payload) == 0 || !json.Valid(descriptor.Payload) {
		return descriptor, fmt.Errorf("isolation descriptor payload must be valid JSON")
	}
	return descriptor, nil
}

func validateContext(d Descriptor, admitted string, s SandboxContext) error {
	if admitted == "" || d.BackendName != admitted {
		return fmt.Errorf("isolation descriptor backend does not match admitted backend")
	}
	if !namePattern.MatchString(d.BackendName) {
		return fmt.Errorf("invalid isolation backend name")
	}
	if !identityPattern.MatchString(d.SandboxID) || !identityPattern.MatchString(d.Generation) {
		return fmt.Errorf("invalid isolation sandbox or generation identity")
	}
	if !digestPattern.MatchString(d.RootfsDigest) || !digestPattern.MatchString(d.RuntimeDigest) || s.WorkloadUID == 0 || s.WorkloadGID == 0 || !identityPattern.MatchString(s.IdentitySource) || !digestPattern.MatchString(s.IdentityDigest) {
		return fmt.Errorf("invalid isolation runtime or workload identity claims")
	}
	for i, gid := range s.SupplementaryGIDs {
		if gid == 0 || (i > 0 && s.SupplementaryGIDs[i-1] >= gid) {
			return fmt.Errorf("supplementary workload groups must be sorted, unique, and non-root")
		}
	}
	if !identityPattern.MatchString(d.SessionID) || d.SessionID != s.SessionID || d.SandboxID != s.SandboxID || d.Generation != s.Generation || d.RootfsDigest != s.RootfsDigest || d.RuntimeDigest != s.RuntimeDigest {
		return fmt.Errorf("isolation descriptor does not match trusted sandbox session")
	}
	return nil
}

// EvidenceDigest returns the canonical digest commitment for backend evidence.
func EvidenceDigest(generation string, evidence []byte) string {
	h := sha256.New()
	_, _ = io.WriteString(h, "quarshell-isolation-evidence-v1\x00")
	_, _ = io.WriteString(h, generation)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(evidence)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
