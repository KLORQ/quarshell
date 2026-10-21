package isolation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type testBackend struct {
	attachCalls int
	boundary    *testBoundary
	confirmErr  error
}

func (b *testBackend) Name() string { return "cautem/vm" }
func (b *testBackend) Attach(_ context.Context, descriptor VerifiedDescriptor, sandbox SandboxContext) (Boundary, error) {
	b.attachCalls++
	if descriptor.SandboxID() != sandbox.SandboxID || descriptor.Generation() != sandbox.Generation {
		return nil, errors.New("backend received mismatched verified claims")
	}
	b.boundary = &testBoundary{evidence: EnforcementEvidence{
		Generation: sandbox.Generation, DefaultDenyEgress: true, NoUnmanagedEgressPath: true,
		RevocationVerified: true, ControllerLossFailsClosed: true, FilesystemConfinement: true,
		RequestAttribution: true, PrivilegeFloor: true, AuthenticatedSupervisor: true,
		RuntimeExitStopsWorkload: true, WorkloadUID: sandbox.WorkloadUID, WorkloadGID: sandbox.WorkloadGID,
		SupplementaryGIDs: append([]uint32(nil), sandbox.SupplementaryGIDs...), IdentitySource: sandbox.IdentitySource,
		IdentityDigest: sandbox.IdentityDigest,
		EvidenceDigest: EvidenceDigest(sandbox.Generation, []byte("measured-test-evidence")),
	}, confirmErr: b.confirmErr}
	return b.boundary, nil
}

type testBoundary struct {
	evidence     EnforcementEvidence
	confirmCalls int
	startCalls   int
	closeCalls   int
	confirmErr   error
	startErr     error
	closeErr     error
}

func (b *testBoundary) Confirm(context.Context) (EnforcementEvidence, error) {
	b.confirmCalls++
	return b.evidence, b.confirmErr
}
func (b *testBoundary) StartWorkload(context.Context) error { b.startCalls++; return b.startErr }
func (b *testBoundary) Close(context.Context) error         { b.closeCalls++; return b.closeErr }

func testContext() SandboxContext {
	return SandboxContext{SandboxID: "sandbox-1", SessionID: "session-1", Generation: "generation-1", RootfsDigest: testDigest, RuntimeDigest: testDigest, WorkloadUID: 1000, WorkloadGID: 1000, IdentitySource: "image", IdentityDigest: testDigest}
}

func descriptorBytes(t *testing.T, mutate func(*Descriptor)) []byte {
	t.Helper()
	sandbox := testContext()
	descriptor := Descriptor{Version: DescriptorVersion, BackendName: "cautem/vm", SandboxID: sandbox.SandboxID, SessionID: sandbox.SessionID, Generation: sandbox.Generation, RootfsDigest: sandbox.RootfsDigest, RuntimeDigest: sandbox.RuntimeDigest, Payload: json.RawMessage(`{"worker_socket":"/private/vm/worker.sock"}`)}
	if mutate != nil {
		mutate(&descriptor)
	}
	data, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRegistryAttachesConfirmsBeforeStartAndCloses(t *testing.T) {
	backend := &testBackend{}
	registry := NewRegistry()
	if err := registry.Register(backend); err != nil {
		t.Fatal(err)
	}
	attached, err := registry.Attach(context.Background(), descriptorBytes(t, nil), "cautem/vm", testContext())
	if err != nil {
		t.Fatal(err)
	}
	if backend.boundary.confirmCalls != 1 || backend.boundary.startCalls != 0 {
		t.Fatalf("attach lifecycle calls: confirm=%d start=%d; workload must remain stopped until explicit Start", backend.boundary.confirmCalls, backend.boundary.startCalls)
	}
	if err := attached.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if backend.boundary.startCalls != 1 {
		t.Fatalf("StartWorkload calls=%d, want 1", backend.boundary.startCalls)
	}
	if err := attached.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "already attempted") {
		t.Fatalf("second Start() error=%v, want already attempted", err)
	}
	if err := attached.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := attached.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if backend.boundary.closeCalls != 1 {
		t.Fatalf("Close calls=%d, want exactly 1", backend.boundary.closeCalls)
	}
	if err := attached.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Start after Close error=%v, want closed", err)
	}
}

func TestRegistryRejectsMismatchedDescriptorBeforeBackendAttach(t *testing.T) {
	backend := &testBackend{}
	registry := NewRegistry()
	if err := registry.Register(backend); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		mutate   func(*Descriptor)
		admitted string
	}{
		{name: "wrong sandbox", mutate: func(d *Descriptor) { d.SandboxID = "sandbox-other" }, admitted: "cautem/vm"},
		{name: "wrong session", mutate: func(d *Descriptor) { d.SessionID = "session-other" }, admitted: "cautem/vm"},
		{name: "stale generation", mutate: func(d *Descriptor) { d.Generation = "generation-old" }, admitted: "cautem/vm"},
		{name: "wrong rootfs", mutate: func(d *Descriptor) {
			d.RootfsDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}, admitted: "cautem/vm"},
		{name: "wrong runtime", mutate: func(d *Descriptor) {
			d.RuntimeDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}, admitted: "cautem/vm"},
		{name: "admission mismatch", mutate: nil, admitted: "other/backend"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := registry.Attach(context.Background(), descriptorBytes(t, test.mutate), test.admitted, testContext()); err == nil {
				t.Fatal("Attach() unexpectedly accepted mismatched descriptor")
			}
		})
	}
	if backend.attachCalls != 0 {
		t.Fatalf("backend Attach called %d times for invalid descriptors", backend.attachCalls)
	}
}

func TestRegistryRejectsIncompleteConfirmationAndCleansBoundary(t *testing.T) {
	backend := &testBackend{}
	// Force backend confirmation to omit the no-unmanaged-egress guarantee.
	badBackend := &badEvidenceBackend{testBackend: backend}
	registry := NewRegistry()
	if err := registry.Register(badBackend); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Attach(context.Background(), descriptorBytes(t, nil), "cautem/vm", testContext()); err == nil {
		t.Fatal("Attach() accepted incomplete enforcement evidence")
	}
	if backend.boundary == nil || backend.boundary.closeCalls != 1 || backend.boundary.startCalls != 0 {
		t.Fatalf("failed confirm cleanup: boundary=%+v", backend.boundary)
	}
}

func TestConfirmErrorClosesAttachedBoundary(t *testing.T) {
	backend := &testBackend{}
	badBackend := &badEvidenceBackend{testBackend: backend}
	registry := NewRegistry()
	if err := registry.Register(badBackend); err != nil {
		t.Fatal(err)
	}
	backend.confirmErr = errors.New("confirm probe failed")
	if _, err := registry.Attach(context.Background(), descriptorBytes(t, nil), "cautem/vm", testContext()); err == nil || !strings.Contains(err.Error(), "confirm probe failed") {
		t.Fatalf("Attach() error=%v, want confirmation failure", err)
	}
	if backend.boundary.closeCalls != 1 || backend.boundary.startCalls != 0 {
		t.Fatalf("confirmation failure cleanup: boundary=%+v", backend.boundary)
	}
}

func TestStartFailureClosesBoundary(t *testing.T) {
	backend := &testBackend{}
	registry := NewRegistry()
	if err := registry.Register(backend); err != nil {
		t.Fatal(err)
	}
	attached, err := registry.Attach(context.Background(), descriptorBytes(t, nil), "cautem/vm", testContext())
	if err != nil {
		t.Fatal(err)
	}
	backend.boundary.startErr = errors.New("guest launch failed")
	if err := attached.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "guest launch failed") {
		t.Fatalf("Start() error=%v, want guest launch failure", err)
	}
	if backend.boundary.closeCalls != 1 {
		t.Fatalf("failed Start left attached boundary open: close calls=%d", backend.boundary.closeCalls)
	}
	if err := attached.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Start after failed launch error=%v, want closed", err)
	}
}

func TestCloseFailureCanBeRetriedWithoutRestartingWorkload(t *testing.T) {
	backend := &testBackend{}
	registry := NewRegistry()
	if err := registry.Register(backend); err != nil {
		t.Fatal(err)
	}
	attached, err := registry.Attach(context.Background(), descriptorBytes(t, nil), "cautem/vm", testContext())
	if err != nil {
		t.Fatal(err)
	}
	backend.boundary.closeErr = errors.New("shutdown timed out")
	if err := attached.Close(context.Background()); err == nil {
		t.Fatal("Close() unexpectedly succeeded")
	}
	backend.boundary.closeErr = nil
	if err := attached.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if backend.boundary.closeCalls != 2 {
		t.Fatalf("Close attempts=%d, want retry after failure", backend.boundary.closeCalls)
	}
	if err := attached.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Start after successful Close error=%v", err)
	}
}

type badEvidenceBackend struct{ *testBackend }

func (b *badEvidenceBackend) Attach(ctx context.Context, descriptor VerifiedDescriptor, sandbox SandboxContext) (Boundary, error) {
	boundary, err := b.testBackend.Attach(ctx, descriptor, sandbox)
	if err == nil && b.testBackend.confirmErr == nil {
		b.testBackend.boundary.evidence.NoUnmanagedEgressPath = false
	}
	return boundary, err
}

func TestDescriptorDecoderRejectsUnknownFieldsTrailingDataAndOversize(t *testing.T) {
	valid := descriptorBytes(t, nil)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "trailing object", data: append(append([]byte(nil), valid...), []byte(` {}`)...)},
		{name: "unknown field", data: []byte(strings.TrimSuffix(string(valid), "}") + `,"unknown":true}`)},
		{name: "duplicate field", data: []byte(strings.TrimSuffix(string(valid), "}") + `,"version":1}`)},
		{name: "oversize", data: bytesOf(MaxDescriptorBytes + 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeDescriptor(test.data); err == nil {
				t.Fatal("decodeDescriptor() unexpectedly succeeded")
			}
		})
	}
}

func TestEncodeDescriptorValidatesAndRoundTripsBoundedClaims(t *testing.T) {
	sandbox := testContext()
	descriptor := Descriptor{
		Version: DescriptorVersion, BackendName: "cautem/vm", SandboxID: sandbox.SandboxID,
		SessionID: sandbox.SessionID, Generation: sandbox.Generation,
		RootfsDigest: sandbox.RootfsDigest, RuntimeDigest: sandbox.RuntimeDigest,
		Payload: json.RawMessage(`{"worker_id":"sandbox-1"}`),
	}
	encoded, err := EncodeDescriptor(descriptor)
	if err != nil {
		t.Fatalf("EncodeDescriptor() error = %v", err)
	}
	decoded, err := decodeDescriptor(encoded)
	if err != nil || decoded.BackendName != descriptor.BackendName || decoded.SessionID != descriptor.SessionID || decoded.Generation != descriptor.Generation {
		t.Fatalf("descriptor round trip = (%+v, %v), want %+v", decoded, err, descriptor)
	}
	descriptor.SessionID = ""
	if _, err := EncodeDescriptor(descriptor); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("missing session error = %v", err)
	}
	descriptor.SessionID = sandbox.SessionID
	descriptor.Payload = json.RawMessage(`{"large":"` + strings.Repeat("x", MaxDescriptorBytes) + `"}`)
	if _, err := EncodeDescriptor(descriptor); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized descriptor error = %v", err)
	}
}

func bytesOf(size int) []byte { return make([]byte, size) }
