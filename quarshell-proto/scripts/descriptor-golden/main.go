// Command descriptor-golden regenerates the canonical OpenShell wire snapshot.
package main

import (
	"fmt"
	"os"
	"path/filepath"

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

func main() {
	snapshot, err := wiregolden.SnapshotRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "descriptor-golden:", err)
		os.Exit(1)
	}
	path := filepath.Join("compat", "openshell-v0.1.4-wire-golden.json")
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := os.WriteFile(path, snapshot, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "descriptor-golden:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", path)
}
