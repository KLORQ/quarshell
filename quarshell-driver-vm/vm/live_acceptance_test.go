//go:build darwin || linux

package vm

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	core "github.com/KLORQ/quarshell-core"
	"github.com/KLORQ/quarshell-driver/driver"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"golang.org/x/crypto/ssh"
)

// TestLiveMicroVMLifecycle exercises the full host-to-guest path. It is opt-in
// because it needs a native libkrun runtime and an ext4/overlayfs-capable guest.
func TestLiveMicroVMLifecycle(t *testing.T) {
	if os.Getenv("CAUTEM_VM_ACCEPTANCE") != "1" {
		t.Skip("set CAUTEM_VM_ACCEPTANCE=1 and configure the MicroVM runtime to run live acceptance")
	}
	rootfsTar := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_ROOTFS_TAR"))
	if rootfsTar == "" {
		t.Fatal("CAUTEM_VM_TEST_ROOTFS_TAR is required for live MicroVM acceptance")
	}
	guestAgent := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_GUEST_AGENT"))
	if guestAgent == "" {
		t.Fatal("CAUTEM_VM_TEST_GUEST_AGENT is required for live MicroVM acceptance")
	}
	initBin := requireLiveAsset(t, "CAUTEM_VM_TEST_INIT_BIN")
	supervisorBin := requireLiveAsset(t, "CAUTEM_VM_TEST_SUPERVISOR_BIN")
	sshBin := requireLiveAsset(t, "CAUTEM_VM_TEST_SSH_BIN")
	securityProbe := requireLiveAsset(t, "CAUTEM_VM_TEST_SECURITY_PROBE")
	runtimeDir := strings.TrimSpace(os.Getenv("OPENSHELL_VM_RUNTIME_DIR"))
	if runtimeDir == "" {
		t.Fatal("OPENSHELL_VM_RUNTIME_DIR is required for live MicroVM acceptance")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("live MicroVM acceptance is unsupported on %s", runtime.GOOS)
	}

	formatter := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_E2FS_FORMATTER"))
	if formatter == "" {
		var err error
		formatter, err = resolveExt4Formatter()
		if err != nil {
			t.Fatal(err)
		}
	}
	stateDir := filepath.Join(t.TempDir(), "state")
	bootstrapRegistry := newTestOCIRegistry(t)
	bootstrapPlatform := v1.Platform{OS: "linux", Architecture: runtime.GOARCH}
	bootstrapRegistry.publishImageIndex(t, []testOCILayerBlob{testOCILayerFromArchive(t, rootfsTar)}, bootstrapPlatform)
	host := HostConfig{
		RuntimeDir: runtimeDir, StateDir: stateDir, RootfsTarDir: filepath.Dir(filepath.Dir(rootfsTar)),
		BootstrapImage: strings.TrimPrefix(bootstrapRegistry.server.URL, "http://") + "/agent:latest",
		KVMDevice:      strings.TrimSpace(os.Getenv("CAUTEM_KVM_DEVICE")),
		FormatterPath:  formatter, OverlaySizeBytes: 64 << 20,
		GuestAgentPath: guestAgent, WorkerPath: strings.TrimSpace(os.Getenv("CAUTEM_VM_WORKER")),
		VCPUs: 2, MemoryMiB: 1024,
		LifecycleExtensions: []LifecycleExtension{liveGuestInitExtension{}},
	}
	host = normalizeHostConfig(host)
	if _, err := ProbeHost(host); err != nil {
		t.Fatalf("probe live MicroVM host: %v", err)
	}
	driverConfig, err := json.Marshal(SandboxDriverConfig{RootfsTarPath: rootfsTar})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	forwardEchoHelper := buildLiveForwardEchoHelper(t, ctx)
	proxyHTTPProbe := buildLiveProxyHTTPProbe(t, ctx)
	proxyBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve live policy proxy fixture: %v", err)
	}
	policyDir := filepath.Join(t.TempDir(), "policy")
	if err := os.Mkdir(policyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(policyDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("# cautem-policy-revision: 1\nversion: 1\nlandlock:\n  compatibility: best_effort\nprocess:\n  run_as_user: '1000'\n  run_as_group: '1000'\nnetwork_policies: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	proxyCABundle := newTestProxyCABundle(t)
	proxyCAPath := filepath.Join(policyDir, "corporate-proxy-ca.pem")
	if err := os.WriteFile(proxyCAPath, proxyCABundle, 0o600); err != nil {
		t.Fatal(err)
	}
	controlEventsPath := filepath.Join(policyDir, "supervisor-events.log")
	brokerPIDPath := filepath.Join(policyDir, "broker.pid")
	policyAckPath := filepath.Join(policyDir, "policy-ack")
	brokerSpec := vmCreateSpec{
		ProxyBin: proxyBinary, PolicyPath: policyPath,
		ProxyEnv: []string{
			"HTTPS_PROXY=http://127.0.0.1:1",
			"CAUTEM_PROXY_CA_BUNDLE=" + proxyCAPath,
			"CAUTEM_VM_TEST_POLICY_PROXY_HELPER=1",
			"CAUTEM_VM_TEST_BROKER_PID_FILE=" + brokerPIDPath,
			"CAUTEM_VM_TEST_POLICY_ACK_FILE=" + policyAckPath,
			"CAUTEM_VM_TEST_CONTROL_EVENTS=" + controlEventsPath,
		},
		InitBin: initBin, SupervisorBin: supervisorBin, EnableSSH: true, SSHBin: sshBin,
	}
	if os.Getenv("CAUTEM_VM_TEST_GATEWAY_CREDENTIALS_ONLY") == "1" {
		proxyCLI := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_PROXY_CLI"))
		if proxyCLI == "" {
			t.Fatal("CAUTEM_VM_TEST_PROXY_CLI is required for paired Gateway credential acceptance")
		}
		assertLiveGatewayCredentialLaunch(t, ctx, host, rootfsTar, initBin, supervisorBin, sshBin, proxyCLI, policyPath)
		return
	}
	first := newLiveAcceptanceDriver(host)
	handle, err := first.createSandbox(ctx, vmCreateSpec{
		Name: "live-microvm-acceptance", Image: "alpine:3.22",
		Labels:                   liveGuestInitExtensionLabels(),
		DriverConfigJSON:         string(driverConfig),
		Command:                  []string{"/bin/sh", "-c", "if [ ! -e /sandbox/.restart-once ]; then touch /sandbox/.restart-once; exit 7; fi; if [ ! -e /sandbox/canonical-identity ]; then printf '%s:%s\\n' \"$(id -u)\" \"$(id -g)\" > /sandbox/canonical-identity; sync; fi; while :; do sleep 1; done"},
		MainProcessRestartPolicy: "on-failure",
		ProxyBin:                 brokerSpec.ProxyBin, PolicyPath: brokerSpec.PolicyPath, ProxyEnv: brokerSpec.ProxyEnv,
		InitBin: brokerSpec.InitBin, SupervisorBin: brokerSpec.SupervisorBin,
		EnableSSH: brokerSpec.EnableSSH, SSHBin: brokerSpec.SSHBin,
	})
	if err != nil {
		t.Fatalf("create live MicroVM: %v", err)
	}
	registerLiveSandboxCleanup(t, host, handle.ID)
	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	vmEvents, err := first.WatchPlatformEvents(watchCtx, driver.PlatformEventWatchRequest{SandboxID: handle.ID})
	if err != nil {
		t.Fatalf("watch live VM lifecycle: %v", err)
	}
	assertLiveVMPlatformState(t, vmEvents, "created")
	if err := first.Start(ctx, handle.ID); err != nil {
		var logs bytes.Buffer
		_ = first.Logs(context.Background(), handle.ID, false, &logs)
		store, _ := openStateStore(host.StateDir)
		_, _, consolePath := workerSocketPaths(store.root, string(handle.ID))
		workerLog, _ := os.ReadFile(consolePath + ".worker")
		t.Fatalf("start live MicroVM: %v; console: %s; worker: %s", err, logs.String(), workerLog)
	}
	assertLiveVMPlatformState(t, vmEvents, "running")
	initialRuntimeGeneration := assertLiveIsolationDescriptor(t, host.StateDir, handle.ID)
	assertLiveSupervisorCredentialConsumed(t, ctx, first, host.StateDir, handle.ID)
	assertLiveGuestProxyCAInstalled(t, ctx, first, handle.ID, proxyCABundle)
	assertLiveGuestInitExtension(t, ctx, first, handle.ID)
	assertLiveWorkloadIdentity(t, ctx, first, handle.ID)
	assertLiveGuestResources(t, ctx, first, handle.ID, host.VCPUs, host.MemoryMiB)
	assertLiveHostLoopbackIsolation(t, ctx, first, handle.ID, "tcp4", "127.0.0.1")
	assertLiveHostLoopbackIsolation(t, ctx, first, handle.ID, "tcp6", "::1")
	assertLiveGuestNetworkBoundaries(t, ctx, first, handle.ID, securityProbe)
	assertLiveNetworkBroker(t, ctx, first, handle.ID, "policy-probe.invalid", proxyHTTPProbe)
	assertLivePolicyRevisionUpdate(t, ctx, first, handle.ID, policyPath, policyAckPath, proxyHTTPProbe)
	assertLiveGuestExec(t, ctx, first, handle.ID)
	assertLiveGuestCopy(t, ctx, first, handle.ID)
	assertLiveSupervisorReady(t, ctx, controlEventsPath)
	assertLiveSupervisorRestarted(t, ctx, controlEventsPath)
	if err := first.EnsureSSHDaemon(ctx, handle.ID); err != nil {
		t.Fatalf("ensure live guest SSH daemon: %v", err)
	}
	assertLiveSSHRelay(t, ctx, first, handle.ID, forwardEchoHelper)
	if err := first.WaitProxyReady(ctx, handle.Name); err != nil {
		t.Fatalf("wait for live VM policy proxy readiness: %v", err)
	}
	if info, err := first.Inspect(ctx, handle.Name); err != nil || info.Status != "running" {
		t.Fatalf("inspect live MicroVM: info=%+v err=%v", info, err)
	}
	if infos, err := first.List(ctx); err != nil || len(infos) != 1 || infos[0].ID != handle.ID {
		t.Fatalf("list live MicroVM: infos=%+v err=%v", infos, err)
	}
	if err := first.Logs(ctx, handle.ID, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("read live MicroVM logs: %v", err)
	}

	// Recover the running worker through a fresh driver instance, as happens
	// after a Gateway/compute process restart.
	recovered := newLiveAcceptanceDriver(host)
	if err := recovered.restoreLaunchSpec(ctx, handle.ID, vmCreateSpec{
		Name: handle.Name, Labels: liveGuestInitExtensionLabels(), Command: []string{"/bin/sh", "-c", "while :; do sleep 1; done"},
		MainProcessRestartPolicy: "on-failure", ProxyBin: brokerSpec.ProxyBin, PolicyPath: brokerSpec.PolicyPath,
		ProxyEnv: brokerSpec.ProxyEnv, InitBin: brokerSpec.InitBin, SupervisorBin: brokerSpec.SupervisorBin,
		EnableSSH: brokerSpec.EnableSSH, SSHBin: brokerSpec.SSHBin,
	}); err != nil {
		t.Fatalf("restore workload spec for live worker adoption: %v", err)
	}
	if err := recovered.Start(ctx, handle.ID); err != nil {
		t.Fatalf("recover live MicroVM worker: %v", err)
	}
	if recoveredGeneration := assertLiveIsolationDescriptor(t, host.StateDir, handle.ID); recoveredGeneration != initialRuntimeGeneration {
		t.Fatalf("adopting the running VM changed runtime generation from %q to %q", initialRuntimeGeneration, recoveredGeneration)
	}
	t.Log("recovered existing live MicroVM worker")
	assertLiveGuestExec(t, ctx, recovered, handle.ID)
	assertLiveWorkloadIdentity(t, ctx, recovered, handle.ID)
	assertLiveGuestCopy(t, ctx, recovered, handle.ID)
	assertLiveSSHRelay(t, ctx, recovered, handle.ID, forwardEchoHelper)
	if err := recovered.Stop(ctx, handle.ID); err != nil {
		t.Fatalf("stop live MicroVM: %v", err)
	}
	assertLiveVMPlatformState(t, vmEvents, "stopped")
	t.Log("stopped recovered live MicroVM worker")
	if err := os.Remove(controlEventsPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("reset supervisor acceptance events before restart: %v", err)
	}
	if err := recovered.restoreLaunchSpec(ctx, handle.ID, vmCreateSpec{
		Name: handle.Name, Labels: liveGuestInitExtensionLabels(), Command: []string{"/bin/sh", "-c", "printf '%s:%s\\n' \"$(id -u)\" \"$(id -g)\" >> /sandbox/restart-identity; sync; while :; do sleep 1; done"}, MainProcessRestartPolicy: "on-failure",
		ProxyBin: brokerSpec.ProxyBin, PolicyPath: brokerSpec.PolicyPath, ProxyEnv: brokerSpec.ProxyEnv,
		InitBin: brokerSpec.InitBin, SupervisorBin: brokerSpec.SupervisorBin,
		EnableSSH: brokerSpec.EnableSSH, SSHBin: brokerSpec.SSHBin,
	}); err != nil {
		t.Fatalf("restore stopped workload spec: %v", err)
	}
	if err := recovered.Start(ctx, handle.ID); err != nil {
		var logs bytes.Buffer
		_ = recovered.Logs(context.Background(), handle.ID, false, &logs)
		store, _ := openStateStore(host.StateDir)
		_, _, consolePath := workerSocketPaths(store.root, string(handle.ID))
		workerLog, _ := os.ReadFile(consolePath + ".worker")
		t.Fatalf("restart live MicroVM: %v; console: %s; worker: %s", err, logs.String(), workerLog)
	}
	t.Log("restarted live MicroVM after explicit stop")
	if restartedGeneration := assertLiveIsolationDescriptor(t, host.StateDir, handle.ID); restartedGeneration == initialRuntimeGeneration {
		t.Fatalf("explicit VM restart reused runtime generation %q", restartedGeneration)
	}
	assertLiveWorkloadIdentity(t, ctx, recovered, handle.ID)
	assertLiveRestartWorkloadIdentity(t, ctx, recovered, handle.ID)
	assertLiveNetworkBroker(t, ctx, recovered, handle.ID, "blocked-probe.invalid", proxyHTTPProbe)
	assertLiveGuestExec(t, ctx, recovered, handle.ID)
	assertLiveGuestCopy(t, ctx, recovered, handle.ID)
	assertLiveSupervisorReady(t, ctx, controlEventsPath)
	assertLiveSSHRelay(t, ctx, recovered, handle.ID, forwardEchoHelper)

	// Kill only the broker while the guest is live. The worker watcher must
	// terminate the VM process group and persist stopped state before recovery.
	brokerPID := liveTestBrokerPID(t, brokerPIDPath)
	brokerProcess, err := os.FindProcess(brokerPID)
	if err != nil {
		t.Fatalf("find live policy broker process %d: %v", brokerPID, err)
	}
	if err := brokerProcess.Kill(); err != nil {
		t.Fatalf("kill live policy broker process %d: %v", brokerPID, err)
	}
	waitForLiveVMStoppedState(t, ctx, host.StateDir, string(handle.ID))
	t.Logf("live broker loss stopped worker group and persisted stopped state (broker pid %d)", brokerPID)

	active := newLiveAcceptanceDriver(host)
	if err := active.restoreLaunchSpec(ctx, handle.ID, vmCreateSpec{
		Name: handle.Name, Labels: liveGuestInitExtensionLabels(), Command: []string{"/bin/sh", "-c", "while :; do sleep 1; done"}, MainProcessRestartPolicy: "on-failure",
		ProxyBin: brokerSpec.ProxyBin, PolicyPath: brokerSpec.PolicyPath, ProxyEnv: brokerSpec.ProxyEnv,
		InitBin: initBin, SupervisorBin: supervisorBin, EnableSSH: true, SSHBin: sshBin,
	}); err != nil {
		t.Fatalf("restore workload after live broker loss: %v", err)
	}
	if err := os.Remove(brokerPIDPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stale broker PID before live broker recovery: %v", err)
	}
	if err := active.Start(ctx, handle.ID); err != nil {
		t.Fatalf("restart MicroVM after live broker loss: %v", err)
	}
	_ = liveTestBrokerPID(t, brokerPIDPath)
	assertLiveNetworkBroker(t, ctx, active, handle.ID, "blocked-probe.invalid", proxyHTTPProbe)
	assertLiveGuestExec(t, ctx, active, handle.ID)

	// Simulate a host worker crash, including its broker/relay child processes.
	// The next driver instance must discard stale sockets and relaunch from the
	// durable rootfs/overlay state after the control plane restores its spec.
	active.mu.Lock()
	worker := active.workers[string(handle.ID)]
	active.mu.Unlock()
	if worker == nil || worker.Process == nil {
		t.Fatal("running MicroVM has no worker process to crash for recovery acceptance")
	}
	if err := killVMWorkerProcessGroupForAcceptance(worker.Process.Pid); err != nil {
		t.Fatalf("kill live VM worker process group: %v", err)
	}
	waitForLiveVMStoppedState(t, ctx, host.StateDir, string(handle.ID))

	afterCrash := newLiveAcceptanceDriver(host)
	if err := afterCrash.restoreLaunchSpec(ctx, handle.ID, vmCreateSpec{
		Name: handle.Name, Labels: liveGuestInitExtensionLabels(), Command: []string{"/bin/sh", "-c", "while :; do sleep 1; done"}, MainProcessRestartPolicy: "on-failure",
		ProxyBin: brokerSpec.ProxyBin, PolicyPath: brokerSpec.PolicyPath, ProxyEnv: brokerSpec.ProxyEnv,
		InitBin: initBin, SupervisorBin: supervisorBin, EnableSSH: true, SSHBin: sshBin,
	}); err != nil {
		t.Fatalf("restore workload after worker crash: %v", err)
	}
	if err := afterCrash.Start(ctx, handle.ID); err != nil {
		t.Fatalf("restart MicroVM after worker crash: %v", err)
	}
	assertLiveGuestExec(t, ctx, afterCrash, handle.ID)
	assertLiveGuestCopy(t, ctx, afterCrash, handle.ID)
	assertLiveNetworkBroker(t, ctx, afterCrash, handle.ID, "blocked-probe.invalid", proxyHTTPProbe)
	assertLiveSSHRelay(t, ctx, afterCrash, handle.ID, forwardEchoHelper)
	if err := afterCrash.Delete(ctx, handle.ID); err != nil {
		t.Fatalf("delete live MicroVM: %v", err)
	}
	assertLiveOCIImageRuntime(t, ctx, host, rootfsTar)

	// Natural completion of the canonical workload must terminate the guest VM
	// instead of leaving an idle guest agent that Gateway could mistake as live.
	terminal := newLiveAcceptanceDriver(host)
	terminalHandle, err := terminal.createSandbox(ctx, vmCreateSpec{
		Name: "live-microvm-main-exit", Image: "alpine:3.22",
		DriverConfigJSON: string(driverConfig), Command: []string{"/bin/sh", "-c", "exit 0"},
	})
	if err != nil {
		t.Fatalf("create terminal-exit MicroVM: %v", err)
	}
	registerLiveSandboxCleanup(t, host, terminalHandle.ID)
	if err := terminal.Start(ctx, terminalHandle.ID); err != nil {
		t.Fatalf("start terminal-exit MicroVM: %v", err)
	}
	waitForLiveVMStoppedState(t, ctx, host.StateDir, string(terminalHandle.ID))
	terminalState, err := openStateStore(host.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	state, err := terminalState.load(string(terminalHandle.ID))
	if err != nil || state.Status != "stopped" {
		t.Fatalf("natural guest workload exit state=%+v err=%v; want stopped VM", state, err)
	}
	t.Log("live natural main-process exit powered off the guest and stopped the VM worker")

	restartHandle, err := terminal.createSandbox(ctx, vmCreateSpec{
		Name: "live-microvm-terminal-explicit-restart", Image: "alpine:3.22",
		DriverConfigJSON: string(driverConfig), Command: []string{"/bin/sh", "-c", "while :; do sleep 1; done"},
	})
	if err != nil {
		t.Fatalf("create terminal-restart MicroVM: %v", err)
	}
	registerLiveSandboxCleanup(t, host, restartHandle.ID)
	if err := terminal.Start(ctx, restartHandle.ID); err != nil {
		t.Fatalf("start terminal-restart MicroVM: %v", err)
	}
	if err := terminal.RecordMainProcessExit(ctx, restartHandle.ID, "live-supervisor-instance", 0); err != nil {
		t.Fatalf("persist live terminal result: %v", err)
	}
	if err := terminal.Start(ctx, restartHandle.ID); err != nil {
		t.Fatalf("explicitly restart terminal MicroVM: %v", err)
	}
	terminalState, err = openStateStore(host.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	state, err = terminalState.load(string(restartHandle.ID))
	if err != nil || state.Status != "running" || state.MainProcessExitCode != nil || state.MainProcessInstanceID != "" {
		t.Fatalf("explicit restart state=%+v err=%v; want running state with cleared terminal result", state, err)
	}
	if err := terminal.Stop(ctx, restartHandle.ID); err != nil {
		t.Fatalf("stop restarted terminal MicroVM: %v", err)
	}
	t.Log("live explicit Start closed the previous terminal VM worker, booted a fresh worker, and cleared its exit result")

	proxyCLI := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_PROXY_CLI"))
	if proxyCLI == "" {
		t.Fatal("CAUTEM_VM_TEST_PROXY_CLI is required for paired Gateway credential acceptance")
	}
	assertLiveGatewayCredentialLaunch(t, ctx, host, rootfsTar, initBin, supervisorBin, sshBin, proxyCLI, policyPath)
}

func assertLiveIsolationDescriptor(t *testing.T, stateDir string, id core.ID) string {
	t.Helper()
	store, err := openStateStore(stateDir)
	if err != nil {
		t.Fatalf("open live VM state store for isolation descriptor: %v", err)
	}
	state, err := store.load(string(id))
	if err != nil {
		t.Fatalf("load live VM state for isolation descriptor: %v", err)
	}
	sandboxDir := filepath.Join(store.root, "sandboxes", state.ID)
	if _, err := readVMIsolationDescriptor(sandboxDir, state); err != nil {
		t.Fatalf("read live VM isolation descriptor: %v", err)
	}
	return state.RuntimeGeneration
}

func assertLiveSupervisorCredentialConsumed(t *testing.T, ctx context.Context, d *Driver, stateDir string, id core.ID) {
	t.Helper()
	store, err := openStateStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	_, guestSocket, _ := workerSocketPaths(store.root, string(id))
	_, err = d.exchangeControl(ctx, string(id), guestSocket, controlRequest{Operation: "copy-from", Path: guestControlCredentialDir + "/client.key"})
	if err == nil {
		t.Fatal("guest supervisor client private-key file remains available after workload startup")
	}
	result, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "test \"$(id -u)\" != 0; test ! -r /run/cautem/supervisor-control-private/ca.pem; test ! -r /run/cautem/supervisor-control-private/client.crt; test ! -e /run/cautem/supervisor-control-private/client.key"}, NoLoginShell: true})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("workload can access supervisor credential directory: result=%+v err=%v", result, err)
	}
}

func assertLiveOCIImageRuntime(t *testing.T, ctx context.Context, host HostConfig, rootfsTar string) {
	t.Helper()
	registry := newTestOCIRegistry(t)
	const registryUsername = "live-acceptance"
	const registryPassword = "test-only-registry-secret"
	registry.requireBasicAuth(registryUsername, registryPassword)
	dockerConfigDir := filepath.Join(t.TempDir(), "docker-config")
	if err := os.Mkdir(dockerConfigDir, 0o700); err != nil {
		t.Fatalf("create isolated Docker credential config: %v", err)
	}
	registryHost := strings.TrimPrefix(registry.server.URL, "http://")
	helperMarker := configureTestDockerCredentialHelper(t, registryHost, registryUsername, registryPassword)
	dockerConfig, err := json.Marshal(map[string]any{
		"credHelpers": map[string]string{registryHost: "test"},
	})
	if err != nil {
		t.Fatalf("marshal isolated Docker credential config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dockerConfigDir, "config.json"), dockerConfig, 0o600); err != nil {
		t.Fatalf("write isolated Docker credential config: %v", err)
	}
	t.Setenv("DOCKER_CONFIG", dockerConfigDir)
	platform, err := vmOCIPlatform()
	if err != nil {
		t.Fatal(err)
	}
	baseLayer := testOCILayerFromArchive(t, rootfsTar)
	appLayer := testOCILayer(t, []rootfsTarEntry{
		{name: "opt", kind: tar.TypeDir, mode: 0o755},
		{name: "opt/app", kind: tar.TypeDir, mode: 0o755},
	})
	workload := "printf '%s|%s|%s' \"$(pwd)\" \"$(id -u)\" \"$OCI_LIVE_EXPECTED\" > /tmp/oci-live-config; printf '%s\\n' \"$OCI_LIVE_EXPECTED\" >> /sandbox/.oci-overlay-restart; sync; while :; do sleep 1; done"
	registry.publishImageIndexWithConfig(t, []testOCILayerBlob{baseLayer, appLayer}, platform, v1.Config{
		Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{workload},
		Env: []string{"OCI_LIVE_EXPECTED=from-pinned-image"}, WorkingDir: "/opt/app", User: "1000:1000",
	})
	imageRef := registryHost + "/agent:latest"
	sandbox := newLiveAcceptanceDriver(host)
	handle, err := sandbox.createSandbox(ctx, vmCreateSpec{Name: "live-oci-image-acceptance", Image: imageRef, Labels: liveGuestInitExtensionLabels()})
	if err != nil {
		t.Fatalf("create live OCI-backed MicroVM: %v", err)
	}
	helperCalls, err := os.ReadFile(helperMarker)
	if err != nil || !strings.Contains(string(helperCalls), registryHost) {
		t.Fatalf("live OCI credential helper was not called for %q: calls=%q err=%v", registryHost, helperCalls, err)
	}
	store, err := openStateStore(host.StateDir)
	if err != nil {
		t.Fatalf("open live OCI state: %v", err)
	}
	state, err := store.load(string(handle.ID))
	if err != nil || state.WorkloadDisk == "" || state.WorkloadSourceType != "oci" || state.WorkloadSourceDigest == "" {
		t.Fatalf("live OCI source identity = %+v err=%v", state, err)
	}
	durableState, err := json.Marshal(state)
	if err != nil || bytes.Contains(durableState, []byte(registryPassword)) || bytes.Contains(durableState, []byte("from-pinned-image")) {
		t.Fatalf("live OCI durable state contains registry credentials or image environment: state=%s err=%v", durableState, err)
	}
	t.Log("live OCI private registry authentication succeeded through a Docker credential helper; credentials and image env are absent from durable state")
	t.Logf("live OCI workload disk prepared in bootstrap VM for %s from platform manifest %s", state.WorkloadPlatform, state.WorkloadSourceDigest)
	registerLiveSandboxCleanup(t, host, handle.ID)
	if err := sandbox.Start(ctx, handle.ID); err != nil {
		t.Fatalf("start live OCI-backed MicroVM: %v", err)
	}
	assertLiveOCIWorkloadConfig(t, ctx, sandbox, handle.ID, "from-pinned-image")
	assertLiveGuestInitExtension(t, ctx, sandbox, handle.ID)
	assertLiveOCIOverlay(t, ctx, sandbox, handle.ID, []string{"from-pinned-image"})
	if err := sandbox.Stop(ctx, handle.ID); err != nil {
		t.Fatalf("stop live OCI-backed MicroVM: %v", err)
	}
	registry.publishImageIndexWithConfig(t, []testOCILayerBlob{baseLayer, appLayer}, platform, v1.Config{
		Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{workload},
		Env: []string{"OCI_LIVE_EXPECTED=from-moved-tag"}, WorkingDir: "/opt/app", User: "1000:1000",
	})
	if err := sandbox.restoreLaunchSpec(ctx, handle.ID, vmCreateSpec{Name: handle.Name, Labels: liveGuestInitExtensionLabels()}); err != nil {
		t.Fatalf("restore live OCI workload from pinned manifest digest: %v", err)
	}
	if err := sandbox.Start(ctx, handle.ID); err != nil {
		t.Fatalf("restart live OCI-backed MicroVM after tag move: %v", err)
	}
	assertLiveOCIWorkloadConfig(t, ctx, sandbox, handle.ID, "from-pinned-image")
	assertLiveOCIOverlay(t, ctx, sandbox, handle.ID, []string{"from-pinned-image", "from-pinned-image"})
	t.Logf("live OCI recovery used pinned workload manifest %s after the image tag moved", state.WorkloadSourceDigest)
	if err := sandbox.Delete(ctx, handle.ID); err != nil {
		t.Fatalf("delete live OCI-backed MicroVM: %v", err)
	}
}

type liveGuestInitExtension struct{}

func (liveGuestInitExtension) Name() string { return "live-guest-init" }
func (liveGuestInitExtension) Activation() ExtensionActivation {
	return ExtensionActivation{RequestKey: "live-guest-init"}
}
func (liveGuestInitExtension) Descriptor() ExtensionDescriptor {
	return ExtensionDescriptor{Name: "live-guest-init", RequiredBackendFeatures: []string{BackendFeatureGuestInitDropins}}
}
func (liveGuestInitExtension) ConfigureLaunch(_ context.Context, _ vmCreateSpec, _ string, plan *ExtensionLaunchPlan) error {
	plan.GuestInitDropins = append(plan.GuestInitDropins, GuestInitDropin{
		Name: "00-live-acceptance.sh", Contents: []byte("#!/bin/sh\nprintf 'guest-init-ok\\n' > /sandbox/.live-guest-init\n"),
	})
	return nil
}

func liveGuestInitExtensionLabels() map[string]string {
	return map[string]string{SandboxExtensionLabelPrefix + "live-guest-init": "enabled"}
}

func assertLiveGuestInitExtension(t *testing.T, ctx context.Context, sandbox *Driver, id core.ID) {
	t.Helper()
	result, err := sandbox.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/cat", "/sandbox/.live-guest-init"}, NoLoginShell: true})
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != "guest-init-ok\n" {
		t.Fatalf("live guest-init extension marker = exit %d stdout %q err=%v", result.ExitCode, result.Stdout, err)
	}
}

func assertLiveOCIOverlay(t *testing.T, ctx context.Context, sandbox *Driver, id core.ID, expected []string) {
	t.Helper()
	result, err := sandbox.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/cat", "/sandbox/.oci-overlay-restart"}, NoLoginShell: true})
	want := strings.Join(expected, "\n") + "\n"
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != want {
		t.Fatalf("live OCI overlay marker = exit %d stdout %q err=%v; want %q", result.ExitCode, result.Stdout, err, want)
	}
}

func assertLiveOCIWorkloadConfig(t *testing.T, ctx context.Context, sandbox *Driver, id core.ID, expectedEnv string) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := sandbox.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/cat", "/tmp/oci-live-config"}, NoLoginShell: true})
		if err == nil && result.ExitCode == 0 {
			want := "/opt/app|1000|" + expectedEnv
			if got := string(result.Stdout); got != want {
				t.Fatalf("live OCI workload config = %q, want %q", got, want)
			}
			terminal, err := sandbox.Exec(ctx, id, driver.ExecRequest{
				Argv:         []string{"/bin/sh", "-c", `printf '%s|%s' "$PWD" "$OCI_LIVE_EXPECTED"`},
				NoLoginShell: true, TTY: true,
			})
			if err != nil || terminal.ExitCode != 0 || string(terminal.Stdout) != "/opt/app|"+expectedEnv {
				t.Fatalf("live OCI TTY exec config = exit %d stdout %q err=%v", terminal.ExitCode, terminal.Stdout, err)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for live OCI workload config: %v", ctx.Err())
		case <-deadline.C:
			t.Fatalf("live OCI workload did not publish config marker: result=%+v err=%v", result, err)
		case <-ticker.C:
		}
	}
}

func newLiveAcceptanceDriver(host HostConfig) *Driver {
	driver := NewWithHostConfig(host)
	driver.ociNameOptions = []name.Option{name.Insecure}
	return driver
}

func registerLiveSandboxCleanup(t *testing.T, host HostConfig, id core.ID) {
	t.Helper()
	t.Cleanup(func() {
		if t.Failed() {
			captureLiveConsoleTail(t, host, id)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := newLiveAcceptanceDriver(host).Delete(ctx, id); err != nil {
			t.Errorf("delete live acceptance sandbox %s during cleanup: %v", id, err)
		}
	})
}

func captureLiveConsoleTail(t *testing.T, host HostConfig, id core.ID) {
	t.Helper()
	artifactDir := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_ARTIFACT_DIR"))
	if artifactDir == "" {
		return
	}
	_, _, consolePath := workerSocketPaths(host.StateDir, string(id))
	data, err := os.ReadFile(consolePath)
	if err != nil {
		workerLog, workerErr := os.ReadFile(consolePath + ".worker")
		if workerErr != nil {
			t.Logf("could not capture VM console or worker log artifacts: console=%v worker=%v", err, workerErr)
			return
		}
		data = workerLog
	}
	const maxTailBytes = 64 << 10
	if len(data) > maxTailBytes {
		data = data[len(data)-maxTailBytes:]
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Logf("could not create VM acceptance artifact directory: %v", err)
		return
	}
	path := filepath.Join(artifactDir, "console-tail-"+string(id)+".log")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Logf("could not write VM console tail artifact: %v", err)
	}
}

func assertLiveHostLoopbackIsolation(t *testing.T, ctx context.Context, d *Driver, id core.ID, network, host string) {
	t.Helper()
	listener, err := net.Listen(network, net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatalf("listen on host %s loopback for isolation check: %v", network, err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan struct{}, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		accepted <- struct{}{}
		const body = "host-loopback-reached\n"
		_, _ = fmt.Fprintf(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
	}()
	endpoint := "http://" + net.JoinHostPort(host, fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port))
	result, err := d.Exec(ctx, id, driver.ExecRequest{
		Argv: []string{"/bin/sh", "-c", `wget -T 3 -q -O - "$1"`, "vm-loopback-isolation", endpoint},
		Env:  []string{"NO_PROXY=*", "no_proxy=*"},
	})
	if err != nil {
		t.Fatalf("probe host %s loopback from guest: %v", network, err)
	}
	if result.ExitCode == 0 || strings.Contains(string(result.Stdout), "host-loopback-reached") {
		t.Fatalf("guest reached host %s loopback: exit=%d stdout=%q stderr=%q", network, result.ExitCode, result.Stdout, result.Stderr)
	}
	select {
	case <-accepted:
		t.Fatalf("guest connected to host %s loopback listener", network)
	default:
	}
}

func assertLiveGuestNetworkBoundaries(t *testing.T, ctx context.Context, d *Driver, id core.ID, probePath string) {
	t.Helper()
	const guestProbePath = "/sandbox/.quarshell-vm-security-probe"
	if err := d.CopyTo(ctx, id, probePath, guestProbePath); err != nil {
		t.Fatalf("copy guest network security probe: %v", err)
	}
	if _, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/chmod", "0700", guestProbePath}}); err != nil {
		t.Fatalf("make guest network security probe executable: %v", err)
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := d.Exec(probeCtx, id, driver.ExecRequest{Argv: []string{guestProbePath, "raw"}})
	if err != nil {
		t.Fatalf("run live raw-socket egress probe: %v", err)
	}
	if raw.ExitCode != 0 || !strings.Contains(string(raw.Stdout), "raw-egress-denied:") {
		t.Fatalf("live raw-socket egress probe = exit %d stdout %q stderr %q", raw.ExitCode, raw.Stdout, raw.Stderr)
	}
	t.Logf("live raw ICMP probe denied egress: %s", strings.TrimSpace(string(raw.Stdout)))

	vsock, err := d.Exec(probeCtx, id, driver.ExecRequest{Argv: []string{guestProbePath, "vsock", "5504"}})
	if err != nil {
		t.Fatalf("run live unmapped-vsock probe: %v", err)
	}
	if vsock.ExitCode != 0 || !strings.Contains(string(vsock.Stdout), "unmapped-vsock-denied: port=5504") {
		t.Fatalf("live unmapped-vsock probe = exit %d stdout %q stderr %q", vsock.ExitCode, vsock.Stdout, vsock.Stderr)
	}
	t.Logf("live guest-to-host vsock probe denied unmapped port: %s", strings.TrimSpace(string(vsock.Stdout)))
}

func assertLiveGuestResources(t *testing.T, ctx context.Context, d *Driver, id core.ID, wantCPUs uint8, wantMemoryMiB uint32) {
	t.Helper()
	result, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{
		"/bin/sh", "-c",
		`awk '/^processor[[:space:]]*:/ {cpus++} END {printf "%d ", cpus+0}' /proc/cpuinfo; awk '/^MemTotal:/ {print $2; exit}' /proc/meminfo`,
	}})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("read live guest CPU/memory limits: result=%+v err=%v", result, err)
	}
	values := strings.Fields(string(result.Stdout))
	if len(values) != 2 {
		t.Fatalf("live guest resource probe output = %q, want CPU count and memory KiB", result.Stdout)
	}
	cpus, cpuErr := strconv.ParseUint(values[0], 10, 8)
	memoryKiB, memoryErr := strconv.ParseUint(values[1], 10, 32)
	if cpuErr != nil || memoryErr != nil {
		t.Fatalf("parse live guest resource probe output %q: cpu error=%v memory error=%v", result.Stdout, cpuErr, memoryErr)
	}
	wantMinMemoryKiB := uint64(wantMemoryMiB) * 1024 * 3 / 4
	wantMaxMemoryKiB := uint64(wantMemoryMiB) * 1024
	if uint8(cpus) != wantCPUs || memoryKiB < wantMinMemoryKiB || memoryKiB > wantMaxMemoryKiB {
		t.Fatalf("live guest resources = %d vCPU, %d MiB visible memory; want %d vCPU and between %d and %d MiB", cpus, memoryKiB/1024, wantCPUs, wantMinMemoryKiB/1024, wantMaxMemoryKiB/1024)
	}
	t.Logf("guest sees %d vCPU and %d MiB memory (configured %d MiB)", cpus, memoryKiB/1024, wantMemoryMiB)
}

func waitForLiveVMStoppedState(t *testing.T, ctx context.Context, stateDir, id string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		store, err := openStateStore(stateDir)
		if err != nil {
			t.Fatalf("open state while waiting for crashed worker: %v", err)
		}
		state, err := store.load(id)
		if err != nil {
			t.Fatalf("read state while waiting for crashed worker: %v", err)
		}
		if state.Status == "stopped" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("worker crash did not update durable state: status=%q: %v", state.Status, ctx.Err())
		case <-ticker.C:
		}
	}
}

func liveTestBrokerPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live policy broker PID: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 || pid == os.Getpid() {
		t.Fatalf("invalid live policy broker PID %q: %v", data, err)
	}
	return pid
}

func assertLiveSupervisorRestarted(t *testing.T, ctx context.Context, eventPath string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	for {
		data, err := os.ReadFile(eventPath)
		if err == nil && strings.Count(string(data), "begin\n") >= 2 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("live supervisor did not restart on failure: events=%q: %v", data, ctx.Err())
		case <-timeout.C:
			t.Fatalf("live supervisor did not start a second process instance: events=%q", data)
		case <-ticker.C:
		}
	}
}

func requireLiveAsset(t *testing.T, variable string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(variable))
	if path == "" {
		t.Fatalf("%s is required for live MicroVM supervisor/SSH acceptance", variable)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s must name an executable regular file: %q", variable, path)
	}
	return path
}

func assertLiveSupervisorReady(t *testing.T, ctx context.Context, eventPath string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	for {
		data, err := os.ReadFile(eventPath)
		if err == nil && strings.Contains(string(data), "begin") && strings.Contains(string(data), "process_event") {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("live supervisor did not complete its begin handshake: events=%q: %v", data, ctx.Err())
		case <-timeout.C:
			t.Fatalf("live supervisor did not complete its begin handshake: events=%q", data)
		case <-ticker.C:
		}
	}
}

func assertLiveSSHRelay(t *testing.T, ctx context.Context, d *Driver, id core.ID, forwardEchoHelper string) {
	t.Helper()
	store, err := openStateStore(d.host.StateDir)
	if err != nil {
		t.Fatalf("open VM state for SSH acceptance: %v", err)
	}
	workerSocket, _, _ := workerSocketPaths(store.root, string(id))
	path := sshRelaySocketPath(workerSocket, string(id))
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", path)
	if err != nil {
		t.Fatalf("connect to live VM SSH relay socket: %v", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	clientConfig := &ssh.ClientConfig{
		User: "acceptance", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 10 * time.Second,
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, "vm-acceptance", clientConfig)
	if err != nil {
		t.Fatalf("complete SSH handshake through live vsock relay: %v", err)
	}
	client := ssh.NewClient(clientConnection, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("open SSH session through live vsock relay: %v", err)
	}
	defer session.Close()
	output, err := session.Output("printf 'ssh-live-ok\\n'")
	if err != nil || !strings.Contains(string(output), "ssh-live-ok") {
		t.Fatalf("run command through live SSH relay: output=%q err=%v", output, err)
	}
	assertLiveSSHDirectTCPForward(t, ctx, client, d, id, forwardEchoHelper)
}

func assertLiveSSHDirectTCPForward(t *testing.T, ctx context.Context, client *ssh.Client, d *Driver, id core.ID, helper string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve guest TCP forward port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	guestPath := "/sandbox/.cautem-forward-echo"
	if err := d.CopyTo(ctx, id, helper, guestPath); err != nil {
		t.Fatalf("copy direct-tcpip echo fixture into guest: %v", err)
	}
	start := fmt.Sprintf("%s %d >/sandbox/.cautem-forward-echo.log 2>&1 &", guestPath, port)
	if result, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", start}, NoLoginShell: true}); err != nil || result.ExitCode != 0 {
		t.Fatalf("start guest direct-tcpip echo fixture: result=%+v err=%v", result, err)
	}
	var forwarded net.Conn
	dialDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(dialDeadline) {
		connection, dialErr := client.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if dialErr == nil {
			forwarded = connection
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("connect through guest direct-tcpip forward: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	if forwarded == nil {
		t.Fatalf("SSH direct-tcpip could not reach guest loopback port %d", port)
	}
	defer forwarded.Close()
	_ = forwarded.SetDeadline(time.Now().Add(10 * time.Second))
	want := []byte("vm-port-forward-live")
	if _, err := forwarded.Write(want); err != nil {
		t.Fatalf("write through guest direct-tcpip forward: %v", err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(forwarded, got); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("guest direct-tcpip echo=%q err=%v, want %q", got, err, want)
	}
}

func buildLiveForwardEchoHelper(t *testing.T, ctx context.Context) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "forward-echo.go")
	output := filepath.Join(dir, "forward-echo")
	const program = `package main
import ("io"; "net"; "os")
func main() {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", os.Args[1]))
	if err != nil { os.Exit(2) }
	defer listener.Close()
	connection, err := listener.Accept()
	if err != nil { os.Exit(3) }
	defer connection.Close()
	_, _ = io.Copy(connection, connection)
}`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatalf("write direct-tcpip echo helper: %v", err)
	}
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, source)
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0", "GOWORK=off")
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build guest direct-tcpip echo helper: %v: %s", err, combined)
	}
	return output
}

func assertLiveNetworkBroker(t *testing.T, ctx context.Context, sandbox *Driver, id core.ID, host, probePath string) {
	t.Helper()
	if err := sandbox.CopyTo(ctx, id, probePath, "/sandbox/proxy-http-probe"); err != nil {
		t.Fatalf("copy live HTTP proxy probe into guest: %v", err)
	}
	result, err := sandbox.Exec(ctx, id, driver.ExecRequest{
		Argv: []string{"/sandbox/proxy-http-probe", host},
		Env:  []string{"HTTP_PROXY=http://127.0.0.1:1", "NO_PROXY=*"},
	})
	if err != nil {
		t.Fatalf("request through live VM policy broker: %v", err)
	}
	response := string(result.Stdout) + string(result.Stderr)
	if result.ExitCode != 0 || !strings.Contains(response, "HTTP 403") {
		t.Fatalf("live VM policy broker did not deny %s: exit=%d stdout=%q stderr=%q", host, result.ExitCode, result.Stdout, result.Stderr)
	}
}

func buildLiveProxyHTTPProbe(t *testing.T, ctx context.Context) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "proxy-http-probe.go")
	output := filepath.Join(dir, "proxy-http-probe")
	program := `package main
import (
  "context"
  "fmt"
  "io"
  "net/http"
  "os"
  "time"
)
func main() {
  if len(os.Args) != 2 { fmt.Fprintln(os.Stderr, "usage: proxy-http-probe HOST"); os.Exit(2) }
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel()
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+os.Args[1]+"/", nil)
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
  response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}}).Do(req)
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
  defer response.Body.Close()
  _, _ = io.Copy(io.Discard, response.Body)
	  fmt.Printf("HTTP %d\n", response.StatusCode)
}`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatalf("write guest HTTP proxy probe: %v", err)
	}
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, source)
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0", "GOWORK=off")
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build guest HTTP proxy probe: %v: %s", err, combined)
	}
	return output
}

func assertLivePolicyRevisionUpdate(t *testing.T, ctx context.Context, sandbox *Driver, id core.ID, policyPath, ackPath, probePath string) {
	t.Helper()
	policyDocument := []byte("# cautem-policy-revision: 2\nversion: 1\nlandlock:\n  compatibility: best_effort\nprocess:\n  run_as_user: '1000'\n  run_as_group: '1000'\nnetwork_policies:\n  live-acceptance:\n    endpoints:\n      - host: policy-probe.invalid\n        port: 80\n")
	updatedPath := policyPath + ".next"
	if err := os.WriteFile(updatedPath, policyDocument, 0o600); err != nil {
		t.Fatalf("write next live policy revision: %v", err)
	}
	if err := os.Rename(updatedPath, policyPath); err != nil {
		t.Fatalf("atomically publish live policy revision: %v", err)
	}
	waitForLivePolicyAck(t, ctx, ackPath, 2)
	t.Log("live policy broker acknowledged revision 2")

	allowed, err := sandbox.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/sandbox/proxy-http-probe", "policy-probe.invalid"}})
	if err != nil {
		t.Fatalf("request through newly allowed live policy endpoint: %v", err)
	}
	allowedOutput := string(allowed.Stdout) + string(allowed.Stderr)
	if allowed.ExitCode != 0 || strings.Contains(allowedOutput, "HTTP 403") {
		t.Fatalf("revision 2 did not allow policy-probe.invalid past policy evaluation: exit=%d stdout=%q stderr=%q", allowed.ExitCode, allowed.Stdout, allowed.Stderr)
	}
	t.Logf("revision 2 allow decision passed policy evaluation: %s", strings.TrimSpace(allowedOutput))
	policyDocument = []byte("# cautem-policy-revision: 3\nversion: 1\nlandlock:\n  compatibility: best_effort\nprocess:\n  run_as_user: '1000'\n  run_as_group: '1000'\nnetwork_policies: {}\n")
	if err := os.WriteFile(updatedPath, policyDocument, 0o600); err != nil {
		t.Fatalf("write policy revocation revision: %v", err)
	}
	if err := os.Rename(updatedPath, policyPath); err != nil {
		t.Fatalf("atomically publish policy revocation revision: %v", err)
	}
	waitForLivePolicyAck(t, ctx, ackPath, 3)
	revoked, err := sandbox.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/sandbox/proxy-http-probe", "policy-probe.invalid"}})
	if err != nil {
		t.Fatalf("request through revoked live policy endpoint: %v", err)
	}
	revokedOutput := string(revoked.Stdout) + string(revoked.Stderr)
	if revoked.ExitCode != 0 || !strings.Contains(revokedOutput, "HTTP 403") {
		t.Fatalf("revision 3 did not revoke policy-probe.invalid: exit=%d stdout=%q stderr=%q", revoked.ExitCode, revoked.Stdout, revoked.Stderr)
	}
	t.Log("live policy revision 3 revoked the previously allowed endpoint")
	assertLiveNetworkBroker(t, ctx, sandbox, id, "blocked-probe.invalid", probePath)
}

func waitForLivePolicyAck(t *testing.T, ctx context.Context, ackPath string, wantRevision int) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(ackPath)
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 2 {
				revision, parseErr := strconv.Atoi(fields[0])
				if parseErr == nil && revision == wantRevision {
					if fields[1] != "loaded" {
						t.Fatalf("live policy revision %d was not loaded: %q", wantRevision, data)
					}
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("live policy broker did not acknowledge revision %d: last ack=%q: %v", wantRevision, data, ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertLiveGuestCopy(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "upload.txt")
	if err := os.WriteFile(source, []byte("microvm-copy-roundtrip"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := d.CopyTo(ctx, id, source, "/workspace/upload.txt"); err != nil {
		t.Fatalf("copy file into live guest: %v", err)
	}
	destination := filepath.Join(root, "download.txt")
	if err := d.CopyFrom(ctx, id, "/workspace/upload.txt", destination); err != nil {
		t.Fatalf("copy file from live guest: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "microvm-copy-roundtrip" {
		t.Fatalf("live guest copy roundtrip = %q err=%v", data, err)
	}
}

func assertLiveGuestProxyCAInstalled(t *testing.T, ctx context.Context, d *Driver, id core.ID, expected []byte) {
	t.Helper()
	root := t.TempDir()
	installedPath := filepath.Join(root, "installed-ca.pem")
	if err := d.CopyFrom(ctx, id, guestProxyCADir+"/ca-0001.crt", installedPath); err != nil {
		t.Fatalf("copy installed corporate proxy CA from live guest: %v", err)
	}
	installed, err := os.ReadFile(installedPath)
	if err != nil || !bytes.Equal(installed, expected) {
		t.Fatalf("live guest installed CA differs from configured bundle: bytes=%d err=%v", len(installed), err)
	}
	mode, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{
		"/bin/sh", "-c", "stat -c '%u:%g:%a' '" + guestProxyCADir + "/ca-0001.crt'",
	}})
	if err != nil || mode.ExitCode != 0 || strings.TrimSpace(string(mode.Stdout)) != "0:0:644" {
		t.Fatalf("live guest proxy CA ownership/mode = %q exit=%d err=%v", mode.Stdout, mode.ExitCode, err)
	}
	bundlePath := filepath.Join(root, "system-ca-bundle.pem")
	if err := d.CopyFrom(ctx, id, "/etc/ssl/certs/ca-certificates.crt", bundlePath); err != nil {
		t.Fatalf("copy guest system trust bundle: %v", err)
	}
	trustBundle, err := os.ReadFile(bundlePath)
	if err != nil || !bytes.Contains(trustBundle, expected) {
		t.Fatalf("live guest system trust bundle does not contain configured CA: bytes=%d err=%v", len(trustBundle), err)
	}
	t.Log("live guest installed corporate proxy CA as root-owned 0644 and included it in system trust bundle")
}

func assertLiveGuestExec(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "profile")
	if err := os.WriteFile(profile, []byte("export QUARSHELL_VM_LOGIN_PROFILE=loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.CopyTo(ctx, id, profile, "/workspace/.profile"); err != nil {
		t.Fatalf("install live guest login profile: %v", err)
	}
	result, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", `printf '%s' "${QUARSHELL_VM_LOGIN_PROFILE-unset}"`}})
	if err != nil {
		t.Fatalf("execute in live guest: %v", err)
	}
	if result.ExitCode != 0 || string(result.Stdout) != "loaded" {
		t.Fatalf("live guest exec result = exit %d stdout %q stderr %q", result.ExitCode, result.Stdout, result.Stderr)
	}
	result, err = d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", `printf '%s' "${QUARSHELL_VM_LOGIN_PROFILE-unset}"`}, NoLoginShell: true})
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != "unset" {
		t.Fatalf("live guest no-login-shell result = exit %d stdout %q stderr %q err=%v", result.ExitCode, result.Stdout, result.Stderr, err)
	}
	result, err = d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "printf 'guest-live-ok\\n'"}})
	if err != nil || result.ExitCode != 0 || !bytes.Contains(result.Stdout, []byte("guest-live-ok")) {
		t.Fatalf("live guest command result = exit %d stdout %q stderr %q err=%v", result.ExitCode, result.Stdout, result.Stderr, err)
	}
	marker := "/sandbox/exec-timeout-must-stop"
	if _, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/rm", "-f", marker}}); err != nil {
		t.Fatalf("clear live guest exec timeout marker: %v", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	_, timeoutErr := d.Exec(timeoutCtx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "sleep 1; touch " + marker}})
	cancel()
	if timeoutErr == nil {
		t.Fatal("live guest exec ignored its context deadline")
	}
	time.Sleep(1100 * time.Millisecond)
	result, err = d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "test ! -e " + marker}})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("live guest exec process survived caller timeout: exit=%d err=%v", result.ExitCode, err)
	}
	assertLiveExecTerminal(t, ctx, d, id)
}

func assertLiveVMPlatformState(t *testing.T, events <-chan driver.PlatformEventResult, want string) {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case result, open := <-events:
			if !open {
				t.Fatalf("VM platform event stream closed before state %q", want)
			}
			if result.Err != nil {
				t.Fatalf("VM platform event stream error: %v", result.Err)
			}
			if result.Event.Metadata["status"] == want {
				t.Logf("live VM platform watch reported state %q", want)
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for live VM platform state %q", want)
		}
	}
}

func assertLiveWorkloadIdentity(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var result driver.ExecResult
	var err error
	for {
		result, err = d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "set -eu; actual=$(id -u):$(id -g); canonical=$(cat /sandbox/canonical-identity); owner=$(stat -c %u:%g /sandbox/canonical-identity); test \"$actual\" = 1000:1000; test \"$canonical\" = \"$actual\"; test \"$owner\" = \"$actual\"; printf 'identity=%s owner=%s\\n' \"$actual\" \"$owner\""}, NoLoginShell: true})
		if err == nil && result.ExitCode == 0 && bytes.Contains(result.Stdout, []byte("identity=1000:1000 owner=1000:1000")) {
			break
		}
		if time.Now().After(deadline) {
			diagnostic, diagnosticErr := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "id; ls -ld /sandbox /workspace 2>&1; ls -la /sandbox /workspace 2>&1"}, NoLoginShell: true})
			var logs bytes.Buffer
			_ = d.Logs(context.Background(), id, false, &logs)
			t.Fatalf("live workload identity does not match canonical process and persistent file owner: result=%+v err=%v diagnostics=%q diagnostic-error=%v logs=%q", result, err, append(diagnostic.Stdout, diagnostic.Stderr...), diagnosticErr, logs.String())
		}
		select {
		case <-ctx.Done():
			t.Fatalf("live workload identity readiness context ended: %v", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
	info, inspectErr := d.Inspect(ctx, string(id))
	if inspectErr != nil || info.WorkloadUID != 1000 || info.WorkloadGID != 1000 {
		t.Fatalf("live VM persisted workload identity=%d:%d err=%v, want observed 1000:1000", info.WorkloadUID, info.WorkloadGID, inspectErr)
	}
	t.Logf("live policy identity matched canonical process and /sandbox file ownership: %s", strings.TrimSpace(string(result.Stdout)))
}

func assertLiveRestartWorkloadIdentity(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	result, err := d.Exec(ctx, id, driver.ExecRequest{Argv: []string{"/bin/sh", "-c", "set -eu; actual=$(id -u):$(id -g); recorded=$(tail -n1 /sandbox/restart-identity); owner=$(stat -c %u:%g /sandbox/restart-identity); test \"$actual\" = 1000:1000; test \"$recorded\" = \"$actual\"; test \"$owner\" = \"$actual\"; printf 'restart-identity=%s owner=%s\\n' \"$recorded\" \"$owner\""}, NoLoginShell: true})
	if err != nil || result.ExitCode != 0 || !bytes.Contains(result.Stdout, []byte("restart-identity=1000:1000 owner=1000:1000")) {
		t.Fatalf("live stop/start workload identity did not persist on /sandbox: result=%+v err=%v", result, err)
	}
	t.Logf("live stop/start workload identity and persistent /sandbox owner: %s", strings.TrimSpace(string(result.Stdout)))
}

func assertLiveExecTerminal(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	session, err := d.OpenExecTerminal(ctx, id, driver.ExecTerminalRequest{
		Argv:         []string{"/bin/sh", "-c", "stty size; IFS= read -r line; stty size"},
		NoLoginShell: true, Columns: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("open live guest exec terminal: %v", err)
	}
	defer session.Close()
	terminalOutput := bufio.NewReader(session)
	var initialOutput strings.Builder
	for !strings.Contains(initialOutput.String(), "24 80") {
		line, err := terminalOutput.ReadString('\n')
		initialOutput.WriteString(line)
		if err != nil {
			t.Fatalf("read live guest initial terminal size from %q: %v", initialOutput.String(), err)
		}
		if initialOutput.Len() > 4096 {
			t.Fatalf("live guest did not report initial terminal dimensions: %q", initialOutput.String())
		}
	}
	if err := session.Resize(ctx, 101, 41); err != nil {
		t.Fatalf("resize live guest exec terminal: %v", err)
	}
	if _, err := io.WriteString(session, "resize-check\n"); err != nil {
		t.Fatalf("write live guest exec terminal input: %v", err)
	}
	output, err := io.ReadAll(terminalOutput)
	if err != nil {
		t.Fatalf("read live guest exec terminal output: %v", err)
	}
	if exitCode, err := session.Wait(ctx); err != nil || exitCode != 0 {
		t.Fatalf("wait for live guest exec terminal: exit %d err=%v", exitCode, err)
	}
	if !bytes.Contains(output, []byte("41 101")) {
		t.Fatalf("live guest terminal did not apply initial and resized dimensions: %q", output)
	}

	result, err := d.Exec(ctx, id, driver.ExecRequest{
		Argv:         []string{"/bin/sh", "-c", "if [ -t 0 ] && [ -t 1 ]; then printf 'tty-exec-ok'; else exit 33; fi"},
		NoLoginShell: true, TTY: true,
	})
	if err != nil || result.ExitCode != 0 || !bytes.Contains(result.Stdout, []byte("tty-exec-ok")) {
		t.Fatalf("live guest TTY exec result = exit %d stdout %q err=%v", result.ExitCode, result.Stdout, err)
	}
	assertLivePTYInterrupt(t, ctx, d, id)
}

func assertLivePTYInterrupt(t *testing.T, ctx context.Context, d *Driver, id core.ID) {
	t.Helper()
	session, err := d.OpenExecTerminal(ctx, id, driver.ExecTerminalRequest{
		Argv:         []string{"/bin/sh", "-c", "trap 'printf interrupted; exit 42' INT; printf 'ready\\n'; while :; do sleep 1; done"},
		NoLoginShell: true, Columns: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("open live guest interrupt terminal: %v", err)
	}
	defer session.Close()
	terminalOutput := bufio.NewReader(session)
	var readyOutput strings.Builder
	for !strings.Contains(readyOutput.String(), "ready") {
		line, err := terminalOutput.ReadString('\n')
		readyOutput.WriteString(line)
		if err != nil {
			t.Fatalf("read live guest interrupt readiness from %q: %v", readyOutput.String(), err)
		}
		if readyOutput.Len() > 4096 {
			t.Fatalf("live guest interrupt command did not become ready: %q", readyOutput.String())
		}
	}
	if _, err := session.Write([]byte{3}); err != nil {
		t.Fatalf("send live guest terminal interrupt: %v", err)
	}
	output, err := io.ReadAll(terminalOutput)
	if err != nil {
		t.Fatalf("read live guest output after terminal interrupt: %v", err)
	}
	exitCode, err := session.Wait(ctx)
	if err != nil || exitCode != 42 || !bytes.Contains(output, []byte("interrupted")) {
		t.Fatalf("live guest terminal interrupt = exit %d output %q err=%v; want trap exit 42", exitCode, output, err)
	}
}
