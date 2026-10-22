package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostConfigFromMapPreservesGRPCEndpoint(t *testing.T) {
	t.Setenv("OPENSHELL_GRPC_ENDPOINT", "https://env.gateway:7443")
	// Use a real CA file because HTTPS endpoints require guest trust material.
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("test ca"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENSHELL_VM_TLS_CA", ca)

	config, err := hostConfigFromMap(map[string]any{"grpc_endpoint": "http://gateway.internal:7443"})
	if err != nil {
		t.Fatal(err)
	}
	if config.GRPCEndpoint != "http://gateway.internal:7443" {
		t.Fatalf("grpc endpoint = %q", config.GRPCEndpoint)
	}
}

func TestHostConfigFromMapReadsGRPCEndpointEnvironment(t *testing.T) {
	t.Setenv("OPENSHELL_GRPC_ENDPOINT", "http://gateway.internal:7443")
	config, err := hostConfigFromMap(nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.GRPCEndpoint != "http://gateway.internal:7443" {
		t.Fatalf("grpc endpoint = %q", config.GRPCEndpoint)
	}
}

func TestHostConfigFromMapRejectsGRPCEndpointPath(t *testing.T) {
	_, err := hostConfigFromMap(map[string]any{"grpc_endpoint": "http://gateway.internal:7443/api"})
	if err == nil {
		t.Fatal("grpc_endpoint path was accepted")
	}
}
