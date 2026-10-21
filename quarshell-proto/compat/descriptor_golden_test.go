package compat_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/KLORQ/quarshell-proto/internal/wiregolden"

	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/compute/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/credentials/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/extension/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway_interceptor/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/middleware/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/options/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/pagination/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/sandbox/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/sandbox_protocol/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/storage/v1"
	_ "github.com/KLORQ/quarshell-proto/gen/go/openshell/test/v1"
)

const wireGoldenPath = "openshell-v0.1.4-wire-golden.json"

func TestOpenShellWireGoldenMatchesGeneratedDescriptors(t *testing.T) {
	actual, err := wiregolden.SnapshotRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_WIRE_GOLDEN") == "1" {
		if err := os.WriteFile(wireGoldenPath, actual, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	golden, err := wiregolden.Read(wireGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !wiregolden.Equal(golden, actual) {
		t.Fatalf("generated OpenShell descriptors drifted from %s; regenerate with UPDATE_WIRE_GOLDEN=1 go test ./compat", filepath.Clean(wireGoldenPath))
	}
}

type manifest struct {
	ServiceCount int `json:"service_count"`
	RPCCount     int `json:"rpc_count"`
	Services     []struct {
		Name string `json:"name"`
	} `json:"services"`
	RPCs []struct {
		Service        string `json:"service"`
		Method         string `json:"method"`
		RequestStream  bool   `json:"request_stream"`
		ResponseStream bool   `json:"response_stream"`
	} `json:"rpcs"`
}

type descriptorMethod struct {
	service         string
	method          string
	clientStreaming bool
	serverStreaming bool
}

func TestOpenShellRPCMANIFESTMatchesDescriptors(t *testing.T) {
	raw, err := os.ReadFile("openshell-v0.1.4-rpc-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	actual, err := wiregolden.SnapshotRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var snapshot wiregolden.Snapshot
	if err := json.Unmarshal(actual, &snapshot); err != nil {
		t.Fatal(err)
	}
	methods := make([]descriptorMethod, 0, manifest.RPCCount)
	services := make(map[string]struct{})
	for _, file := range snapshot.Files {
		for _, service := range file.Services {
			simple := service.Name[strings.LastIndex(service.Name, ".")+1:]
			services[simple] = struct{}{}
			for _, method := range service.Methods {
				methods = append(methods, descriptorMethod{
					service: simple, method: method.Name,
					clientStreaming: method.ClientStreaming, serverStreaming: method.ServerStreaming,
				})
			}
		}
	}
	if len(services) != manifest.ServiceCount {
		t.Fatalf("descriptor services=%d; manifest=%d", len(services), manifest.ServiceCount)
	}
	if len(methods) != manifest.RPCCount {
		t.Fatalf("descriptor RPCs=%d; manifest=%d", len(methods), manifest.RPCCount)
	}
	manifestMethods := make([]descriptorMethod, 0, len(manifest.RPCs))
	manifestServices := make(map[string]struct{})
	for _, rpc := range manifest.RPCs {
		manifestServices[rpc.Service] = struct{}{}
		manifestMethods = append(manifestMethods, descriptorMethod{
			service: rpc.Service, method: rpc.Method,
			clientStreaming: rpc.RequestStream, serverStreaming: rpc.ResponseStream,
		})
	}
	for service := range manifestServices {
		if _, ok := services[service]; !ok {
			t.Fatalf("manifest service %q is missing from generated descriptors", service)
		}
	}
	sortDescriptorMethods(methods)
	sortDescriptorMethods(manifestMethods)
	for i := range methods {
		if methods[i] != manifestMethods[i] {
			t.Fatalf("RPC manifest mismatch at %d: descriptor=%+v manifest=%+v", i, methods[i], manifestMethods[i])
		}
	}
}

func sortDescriptorMethods(methods []descriptorMethod) {
	sort.Slice(methods, func(i, j int) bool {
		if methods[i].service != methods[j].service {
			return methods[i].service < methods[j].service
		}
		return methods[i].method < methods[j].method
	})
}
