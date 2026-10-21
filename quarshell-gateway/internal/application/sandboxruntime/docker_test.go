package sandboxruntime

import (
	"testing"

	"github.com/KLORQ/quarshell-driver/driver"
)

func TestCreateRequestRestoresVMRuntimeAssetsAndProxy(t *testing.T) {
	proxyEnv := []string{"CAUTEM_VM_TEST=value"}
	runtime := &Docker{
		WorkspaceKind: driver.WorkspaceGuestOverlay,
		GatewayURL:    "https://gateway.example:7444",
		Runtime:       driver.RuntimeAssets{InitBinary: "/opt/cautem/init", SupervisorBinary: "/opt/cautem/supervisor", SSHBinary: "/opt/cautem/sshd", EnableSSH: true},
		Proxy:         &driver.ProxySpec{Binary: "/opt/cautem/cautem", PolicyPath: "/var/lib/cautem/policy.yaml", Port: 3128, Environment: proxyEnv},
	}
	request := runtime.createRequest(Sandbox{Workspace: "ws", Sandbox: "sb", Image: "image"})
	if request.Workspace.Kind != driver.WorkspaceGuestOverlay || request.Network.GatewayURL != runtime.GatewayURL {
		t.Fatalf("workspace/network = %+v / %+v", request.Workspace, request.Network)
	}
	if request.Runtime.InitBinary != runtime.Runtime.InitBinary || request.Runtime.SupervisorBinary != runtime.Runtime.SupervisorBinary || !request.Runtime.EnableSSH {
		t.Fatalf("runtime assets = %+v", request.Runtime)
	}
	if request.Network.Proxy == nil || request.Network.Proxy.Binary != runtime.Proxy.Binary || request.Network.Proxy.PolicyPath != runtime.Proxy.PolicyPath || len(request.Network.Proxy.Environment) != 1 {
		t.Fatalf("proxy config = %+v", request.Network.Proxy)
	}
	request.Network.Proxy.Environment[0] = "mutated"
	if proxyEnv[0] != "CAUTEM_VM_TEST=value" {
		t.Fatalf("proxy environment aliases gateway config: %q", proxyEnv)
	}
}
