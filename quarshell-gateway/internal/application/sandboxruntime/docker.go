// Package sandboxruntime adapts compute backends to sandbox lifecycle calls.
package sandboxruntime

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/KLORQ/quarshell-core"
	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-driver/driver"
)

// Sandbox is the backend-facing lifecycle input.
type Sandbox struct {
	Workspace domain.WorkspaceID
	Sandbox   domain.SandboxID
	RuntimeID string
	Image     string
	Command   []string
	TTY       bool
	Resources Resources
	Network   Network
	Labels    map[string]string
}

// Network is the egress and host-network configuration.
type Network struct {
	GatewayURL  string
	ExtraHosts  []string
	PortPublish []PortPublish
}

// PortPublish maps a host loopback port to a sandbox port.
type PortPublish struct {
	Host  int
	Guest int
}

// Resources are portable sandbox limits.
type Resources struct {
	CPUCores    float64
	MemoryBytes int64
	PIDs        int64
	GPU         bool
	GPUCount    int
}

// Runtime is the sandbox lifecycle port implemented by compute backends.
type Runtime interface {
	Create(context.Context, Sandbox) (string, error)
	Start(context.Context, Sandbox) error
	Stop(context.Context, Sandbox) error
	Delete(context.Context, Sandbox) error
	Inspect(context.Context, Sandbox) (driver.Info, error)
}

// Docker adapts driver.Backend to Runtime.
type Docker struct {
	Backend       driver.Backend
	WorkspaceRoot string
	WorkspaceKind driver.WorkspaceKind
	Runtime       driver.RuntimeAssets
	Proxy         *driver.ProxySpec
	GatewayURL    string
}

// NewDocker returns a Docker runtime adapter.
func NewDocker(backend driver.Backend) (*Docker, error) {
	if backend == nil {
		return nil, domain.Invalidf("sandboxruntime.docker", "backend is required")
	}
	return NewForBackend(backend, driver.WorkspaceHostPath), nil
}

// PTY opens the canonical process terminal when supported.
func (d *Docker) PTY(ctx context.Context, sandbox Sandbox, columns, rows uint32) (io.ReadWriteCloser, error) {
	terminal, ok := d.Backend.(driver.MainProcessTTY)
	if !ok {
		return nil, domain.New(domain.CodeUnavailable, "sandboxruntime.pty", "backend does not expose a PTY")
	}
	return terminal.OpenMainProcessTTY(ctx, id(sandbox), columns, rows)
}

// Logs streams the canonical sandbox process logs when supported.
func (d *Docker) Logs(ctx context.Context, sandbox Sandbox) (io.ReadCloser, error) {
	attacher, ok := d.Backend.(driver.MainProcessLogAttacher)
	if !ok {
		return nil, domain.New(domain.CodeUnavailable, "sandboxruntime.logs", "backend does not expose logs")
	}
	return attacher.OpenMainProcessLogs(ctx, id(sandbox))
}

// Exec runs one command through the optional backend exec contract.
func (d *Docker) Exec(ctx context.Context, sandbox Sandbox, request driver.ExecRequest) (driver.ExecResult, error) {
	executor, ok := d.Backend.(driver.InteractiveExec)
	if !ok {
		return driver.ExecResult{}, domain.New(domain.CodeUnavailable, "sandboxruntime.exec", "backend does not support command execution")
	}
	return executor.Exec(ctx, id(sandbox), request)
}

// OpenExecTerminal opens a bidirectional PTY for an interactive command.
func (d *Docker) OpenExecTerminal(ctx context.Context, sandbox Sandbox, request driver.ExecTerminalRequest) (driver.ExecTerminalSession, error) {
	executor, ok := d.Backend.(driver.ExecTerminal)
	if !ok {
		return nil, domain.New(domain.CodeUnavailable, "sandboxruntime.exec_terminal", "backend does not support interactive terminals")
	}
	return executor.OpenExecTerminal(ctx, id(sandbox), request)
}

// Inspect returns backend state for reconciliation.
func (d *Docker) Inspect(ctx context.Context, sandbox Sandbox) (driver.Info, error) {
	return d.Backend.Inspect(ctx, string(id(sandbox)))
}

// NewForBackend returns an adapter with the backend's workspace kind.
func NewForBackend(backend driver.Backend, kind driver.WorkspaceKind) *Docker {
	return &Docker{Backend: backend, WorkspaceRoot: filepath.Join(os.TempDir(), "quarshell-workspaces"), WorkspaceKind: kind}
}

// Create provisions one sandbox.
func (d *Docker) Create(ctx context.Context, sandbox Sandbox) (string, error) {
	if err := validateRuntimeContract(ctx, d.Backend); err != nil {
		return "", err
	}
	request := d.createRequest(sandbox)
	if request.Workspace.Kind == driver.WorkspaceHostPath {
		if err := os.MkdirAll(request.Workspace.HostPath, 0o700); err != nil {
			return "", err
		}
	}
	handle, err := d.Backend.Create(ctx, request)
	if err != nil {
		return "", err
	}
	return string(handle.ID), nil
}

// Start starts a sandbox.
func (d *Docker) Start(ctx context.Context, sandbox Sandbox) error {
	if err := validateRuntimeContract(ctx, d.Backend); err != nil {
		return err
	}
	return d.Backend.Start(ctx, id(sandbox))
}

// StartWithToken starts a runtime that consumes short-lived supervisor
// credentials. The token is passed only through this call and is never copied
// into the durable sandbox record.
func (d *Docker) StartWithToken(ctx context.Context, sandbox Sandbox, token string) error {
	if err := validateRuntimeContract(ctx, d.Backend); err != nil {
		return err
	}
	if restorer, ok := d.Backend.(driver.RuntimeLaunchSpecRestorer); ok {
		if err := restorer.RestoreLaunchSpec(ctx, id(sandbox), d.createRequest(sandbox)); err != nil {
			return err
		}
	}
	starter, ok := d.Backend.(driver.RuntimeTokenStarter)
	if !ok {
		return domain.New(domain.CodeUnavailable, "sandboxruntime.start", "backend does not support token-aware start")
	}
	return starter.StartWithToken(ctx, id(sandbox), token)
}

// StartWithCredentials starts a backend with the paired Gateway and Sandbox
// Protocol credentials issued for one sandbox authorization epoch.
func (d *Docker) StartWithCredentials(ctx context.Context, sandbox Sandbox, credentials driver.RuntimeLaunchCredentials) error {
	if err := validateRuntimeContract(ctx, d.Backend); err != nil {
		return err
	}
	if restorer, ok := d.Backend.(driver.RuntimeLaunchSpecRestorer); ok {
		if err := restorer.RestoreLaunchSpec(ctx, id(sandbox), d.createRequest(sandbox)); err != nil {
			return err
		}
	}
	starter, ok := d.Backend.(driver.RuntimeCredentialStarter)
	if !ok {
		return domain.New(domain.CodeUnavailable, "sandboxruntime.start", "backend does not support paired runtime credentials")
	}
	return starter.StartWithCredentials(ctx, id(sandbox), credentials)
}

// SupportsRuntimeCredentials reports whether the wrapped compute backend can
// consume the complete Gateway/Sandbox Protocol launch credential pair.
func (d *Docker) SupportsRuntimeCredentials() bool {
	_, ok := d.Backend.(driver.RuntimeCredentialStarter)
	return ok
}

func (d *Docker) createRequest(sandbox Sandbox) driver.BackendCreateRequest {
	kind := d.WorkspaceKind
	if kind == "" {
		kind = driver.WorkspaceHostPath
	}
	workspacePath := ""
	if kind == driver.WorkspaceHostPath {
		workspacePath = filepath.Join(d.WorkspaceRoot, string(sandbox.Workspace), string(sandbox.Sandbox))
	}
	return driver.BackendCreateRequest{
		SandboxID: core.ID(string(sandbox.Sandbox)), Name: string(sandbox.Sandbox),
		Workspace: driver.WorkspaceTarget{Kind: kind, ID: string(sandbox.Workspace), HostPath: workspacePath, Persist: true},
		Workload:  driver.WorkloadSpec{Image: sandbox.Image, Command: append([]string(nil), sandbox.Command...), TTY: sandbox.TTY},
		Resources: driver.ResourceLimits{CPUCores: sandbox.Resources.CPUCores, MemoryBytes: sandbox.Resources.MemoryBytes, PIDs: sandbox.Resources.PIDs, GPU: sandbox.Resources.GPU, GPUCount: sandbox.Resources.GPUCount},
		Network:   d.networkSpec(sandbox.Network), Runtime: d.Runtime, Labels: sandbox.Labels,
	}
}

func (d *Docker) networkSpec(network Network) driver.NetworkSpec {
	out := networkSpec(network)
	if strings.TrimSpace(out.GatewayURL) == "" {
		out.GatewayURL = strings.TrimSpace(d.GatewayURL)
	}
	if d.Proxy != nil {
		proxy := *d.Proxy
		proxy.Environment = append([]string(nil), proxy.Environment...)
		out.Proxy = &proxy
	}
	return out
}

func validateRuntimeContract(ctx context.Context, backend driver.Backend) error {
	validator, ok := backend.(driver.RuntimeContractValidator)
	if !ok {
		return nil
	}
	return validator.ValidateRuntimeContract(ctx)
}

// Stop stops a sandbox.
func (d *Docker) Stop(ctx context.Context, sandbox Sandbox) error {
	return d.Backend.Stop(ctx, id(sandbox))
}

// Delete removes a sandbox.
func (d *Docker) Delete(ctx context.Context, sandbox Sandbox) error {
	return d.Backend.Delete(ctx, id(sandbox))
}

func networkSpec(network Network) driver.NetworkSpec {
	out := driver.NetworkSpec{GatewayURL: network.GatewayURL, ExtraHosts: append([]string(nil), network.ExtraHosts...)}
	for _, publish := range network.PortPublish {
		out.PortPublish = append(out.PortPublish, driver.PortPublish{Host: publish.Host, Guest: publish.Guest})
	}
	return out
}

func id(sandbox Sandbox) core.ID {
	if strings.TrimSpace(sandbox.RuntimeID) != "" {
		return core.ID(sandbox.RuntimeID)
	}
	return core.ID(strings.TrimSpace(string(sandbox.Sandbox)))
}
