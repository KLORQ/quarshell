package isolation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHostAuthBundleRoundTripAndDescriptorBinding(t *testing.T) {
	descriptor := Descriptor{
		Version: DescriptorVersion, BackendName: "quarshell/vm", SandboxID: "sandbox-a",
		SessionID: "11111111-1111-4111-8111-111111111111", Generation: "g0000000000000001",
		RootfsDigest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RuntimeDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Payload:       json.RawMessage(`{"worker_id":"sandbox-a"}`),
	}
	bundle := HostAuthBundle{
		SessionID: descriptor.SessionID, RuntimeGeneration: "g0000000000000002",
		SessionRotation: 2, AuthEpoch: 3, GatewayToken: "signed.jwt.value",
		GatewayExpiresAt: time.Now().Add(time.Hour).Unix(), SandboxToken: "sandbox.jwt.value",
		SandboxExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	encoded, err := EncodeHostAuthBundle(bundle, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHostAuthBundle(encoded, descriptor)
	if err != nil || decoded != bundle {
		t.Fatalf("DecodeHostAuthBundle() = (%+v, %v), want %+v", decoded, err, bundle)
	}
	if decoded.RuntimeGeneration != "g0000000000000002" {
		t.Fatalf("runtime generation=%q, want gateway generation", decoded.RuntimeGeneration)
	}
}

func TestDecodeHostAuthBundleRejectsUnknownFieldsAndMalformedToken(t *testing.T) {
	descriptor := Descriptor{
		BackendName: "quarshell/vm", SandboxID: "sandbox-a", SessionID: "11111111-1111-4111-8111-111111111111", Generation: "g0000000000000001",
		RootfsDigest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RuntimeDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	base := `{"session_id":"11111111-1111-4111-8111-111111111111","runtime_generation":"g0000000000000001","session_rotation":1,"auth_epoch":2,"gateway_token":"signed.jwt.value","gateway_expires_at":` + fmt.Sprint(time.Now().Add(time.Hour).Unix()) + `,"sandbox_token":"sandbox.jwt.value","sandbox_expires_at":` + fmt.Sprint(time.Now().Add(time.Hour).Unix()) + `}`
	for _, raw := range []string{
		strings.TrimSuffix(base, "}") + `,"extra":"value"}`,
		strings.Replace(base, `"signed.jwt.value"`, `"bad\ntoken"`, 1),
	} {
		if _, err := DecodeHostAuthBundle([]byte(raw), descriptor); err == nil {
			t.Fatalf("DecodeHostAuthBundle accepted %s", raw)
		}
	}
}

func TestHostAuthBundleJSONMatchesOpenShellSupervisorAuthBundle(t *testing.T) {
	descriptor := Descriptor{
		Version: DescriptorVersion, BackendName: "quarshell/vm", SandboxID: "sandbox-a",
		SessionID: "11111111-1111-4111-8111-111111111111", Generation: "g0000000000000001",
		RootfsDigest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RuntimeDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Payload:       json.RawMessage(`{"worker_id":"sandbox-a"}`),
	}
	encoded, err := EncodeHostAuthBundle(HostAuthBundle{
		SessionID: descriptor.SessionID, RuntimeGeneration: descriptor.Generation,
		SessionRotation: 1, AuthEpoch: 1, GatewayToken: "gateway-token",
		GatewayExpiresAt: time.Now().Add(time.Hour).Unix(), SandboxToken: "sandbox-token",
		SandboxExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{"session_id", "runtime_generation", "session_rotation", "auth_epoch", "gateway_token", "gateway_expires_at", "sandbox_token", "sandbox_expires_at"}
	if len(fields) != len(want) {
		t.Fatalf("bundle fields=%v, want exactly %v", fields, want)
	}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Fatalf("bundle is missing OpenShell field %q", key)
		}
	}
}
