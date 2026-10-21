//go:build darwin || linux

package vm

import (
	"context"
	"testing"

	"github.com/KLORQ/quarshell-driver/driver"
)

func TestBackendCapabilitiesAdvertiseRuntimeExtensions(t *testing.T) {
	info, err := (*Driver)(nil).Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []driver.Capability{
		driver.CapabilityInteractiveExec,
		driver.CapabilityPTY,
		driver.CapabilityExecTerminal,
		driver.CapabilityLogAccess,
		driver.CapabilityFileTransfer,
		driver.CapabilityLaunchSpecRestore,
		driver.CapabilityTokenAwareStart,
		driver.CapabilityGatewayRuntimeValidation,
		driver.CapabilityProxyReadiness,
		driver.CapabilityRootfsTarStaging,
	} {
		if !info.Has(capability) {
			t.Errorf("VM backend omitted capability %q", capability)
		}
	}
}
