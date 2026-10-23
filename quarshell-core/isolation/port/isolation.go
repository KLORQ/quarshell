// Package isolation defines how an allocated sandbox is restricted. It is
// independent of resource allocation.
package isolation

import (
	"context"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// FilesystemPolicy is the normalized filesystem restriction.
type FilesystemPolicy struct {
	Read  []string
	Write []string
	Exec  []string
}

// NetworkPolicy is the normalized network restriction.
type NetworkPolicy struct {
	Mode      string
	Endpoints []string
}

// ProcessPolicy is the normalized process restriction.
type ProcessPolicy struct {
	AllowPTrace     bool
	NoNewPrivs      bool
	Capabilities    []string
	SeccompProfile  string
	PidsLimit       int64
	MemoryBytes     int64
	CPUQuotaMicros  int64
	CPUPeriodMicros int64
}

// SecurityRequirements is the set of restrictions that must be verified before
// a workload is launched.
type SecurityRequirements struct {
	Filesystem FilesystemPolicy
	Network    NetworkPolicy
	Process    ProcessPolicy
}

// EvidenceItem records one verified security property.
type EvidenceItem struct {
	Name    string
	Source  string
	Details map[string]string
}

// SecurityEvidence is bound to one instance and host capability set. It does not
// promise future readiness; Prepare must re-check the required controls.
type SecurityEvidence struct {
	Instance     domain.InstanceRef
	Kernel       string
	Capabilities domain.CapabilitySet
	Items        []EvidenceItem
	VerifiedAt   time.Time
}

// Descriptor is an explicitly allowed inherited file descriptor.
type Descriptor struct {
	FD       int
	Source   string
	Writable bool
}

// LaunchSpec is the trusted, already-authorized process launch request.
type LaunchSpec struct {
	Instance     domain.InstanceRef
	Command      []string
	Environment  map[string]string
	Workdir      string
	UID          uint32
	GID          uint32
	Groups       []uint32
	Descriptors  []Descriptor
	PolicyHash   string
	Requirements SecurityRequirements
}

// PreparedLaunch is an opaque, time-bounded handle produced by Prepare. It
// cannot be constructed from user JSON.
type PreparedLaunch interface {
	Instance() domain.InstanceRef
	ExpiresAt() time.Time
}

// ProcessRef identifies a launched process by instance and kernel start time to
// avoid PID reuse ambiguity.
type ProcessRef struct {
	Instance  domain.InstanceRef
	PID       int
	StartTime time.Time
}

// Isolation verifies and applies workload restrictions.
type Isolation interface {
	Probe(context.Context, domain.InstanceRef, SecurityRequirements) (SecurityEvidence, error)
	Prepare(context.Context, LaunchSpec) (PreparedLaunch, error)
	Launch(context.Context, PreparedLaunch) (ProcessRef, error)
	Stop(context.Context, ProcessRef) error
}
