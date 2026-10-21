package domain

// Capability names an optional backend or extension behavior. A method existing
// on an interface does not prove host capability.
type Capability string

const (
	CapExec                Capability = "exec"
	CapExecInteractive     Capability = "exec-interactive"
	CapPTY                 Capability = "pty"
	CapFileTransfer        Capability = "file-transfer"
	CapServiceForward      Capability = "service-forward"
	CapPolicyUpdate        Capability = "policy-update"
	CapRootfsStaging       Capability = "rootfs-staging"
	CapNativeEvents        Capability = "native-events"
	CapGPU                 Capability = "gpu"
	CapSnapshot            Capability = "snapshot"
	CapNetworkMediation    Capability = "network-mediation"
	CapResourceLimits      Capability = "resource-limits"
	CapWorkspaceStorage    Capability = "workspace-storage"
	CapSeccompNotification Capability = "seccomp-notification"
	CapUserNamespaces      Capability = "user-namespaces"
	CapLandlock            Capability = "landlock"
)

// CapabilitySet is an immutable set of capabilities.
type CapabilitySet struct {
	values map[Capability]struct{}
}

// NewCapabilitySet returns a set containing the given capabilities.
func NewCapabilitySet(values ...Capability) CapabilitySet {
	set := CapabilitySet{values: make(map[Capability]struct{}, len(values))}
	for _, value := range values {
		if value != "" {
			set.values[value] = struct{}{}
		}
	}
	return set
}

// Has reports whether the capability is present.
func (s CapabilitySet) Has(value Capability) bool {
	if s.values == nil {
		return false
	}
	_, ok := s.values[value]
	return ok
}

// Require returns an unsupported error when the capability is absent.
func (s CapabilitySet) Require(value Capability, op string) error {
	if !s.Has(value) {
		return &Error{Code: CodeUnsupported, Op: op, Message: "capability " + string(value) + " is unavailable", Err: ErrUnsupported}
	}
	return nil
}

// Values returns a copy of the capabilities.
func (s CapabilitySet) Values() []Capability {
	result := make([]Capability, 0, len(s.values))
	for value := range s.values {
		result = append(result, value)
	}
	return result
}
