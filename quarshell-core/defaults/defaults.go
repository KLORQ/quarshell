// Package defaults holds shared ports, guest paths, and image names used across quarshell modules.
package defaults

import (
	"fmt"
	"strings"
	"time"
)

// Well-known TCP ports.
const (
	ProxyPort   = 3128
	GatewayPort = 7443
	NoVNCPort   = 6080
	HTTPSPort   = 443
	HTTPPort    = 80
)

// GatewayListen is the default quarshell-gateway bind address.
const GatewayListen = "127.0.0.1:7443"

// HostInternal is the host gateway alias injected into sandbox DNS.
const HostInternal = "host.quarshell.internal"

// ProxyListenHost is used when binding the sidecar on all interfaces inside the netns.
const ProxyListenHost = "0.0.0.0"

// Guest filesystem layout inside sandboxes.
const (
	GuestRoot   = "/quarshell"
	GuestData   = GuestRoot + "/data"
	GuestHome   = GuestData + "/home"
	GuestBin    = GuestData + "/bin"
	GuestCADir  = GuestRoot + "/ca"
	GuestCAFile = GuestCADir + "/ca.pem"
	GuestPolicy = GuestRoot + "/policy.yaml"
	GuestInit   = GuestRoot + "/quarshell-init"
	GuestPath   = "/quarshell/data/home/.local/bin:/quarshell/data/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// GuestEtcOSG is the reserved control tree for supervisor-owned guidance.
	// Same role as OpenShell's /etc/openshell; quarshell naming, Apache-2.0 project content.
	GuestEtcOSG = "/etc/quarshell"
	GuestSkills = GuestEtcOSG + "/skills"
)

// NoProxyValue is the default NO_PROXY / no_proxy list for sandbox guests.
const NoProxyValue = "localhost,127.0.0.1,::1"

// Sandbox image tags (local dev) and GHCR catalog (OpenShell-style paths).
const (
	ImageDebian = "debian:bookworm"
	// ImageProxy is the slim base for egress sidecars (not the agent/sandbox image).
	ImageProxy  = "debian:bookworm-slim"
	ImageLocal  = "quarshell-sandbox:local"
	ImageGUI    = "quarshell-sandbox:gui"
	ImageGPU    = "quarshell-sandbox:gpu"
	ImageGUIGPU = "quarshell-sandbox:gui-gpu"
	ImageCursor = "quarshell-sandbox:cursor"
	ImageClaude = "quarshell-sandbox:claude"
	ImageCodex  = "quarshell-sandbox:codex"

	// SandboxPidsLimit is the OpenShell-aligned default PIDs cgroup limit when unset.
	SandboxPidsLimit int64 = 2048

	// GHCR: separate image per flavor (like openshell-community/sandboxes/<name>).
	GHCROrg        = "ghcr.io/klorq"
	GHCRGateway    = GHCROrg + "/quarshell/gateway"
	GHCRSandboxes  = GHCROrg + "/quarshell/sandboxes"
	ImageBaseRef   = GHCRSandboxes + "/base:latest"
	ImageGUIRef    = GHCRSandboxes + "/gui:latest"
	ImageGPURef    = GHCRSandboxes + "/gpu:latest"
	ImageGUIGPURef = GHCRSandboxes + "/gui-gpu:latest"
	ImageCursorRef = GHCRSandboxes + "/cursor:latest"
	ImageClaudeRef = GHCRSandboxes + "/claude:latest"
	ImageCodexRef  = GHCRSandboxes + "/codex:latest"
)

// EnvImagePull controls pulling published images for missing local
// quarshell-sandbox:* tags: "never" disables it (offline / local builds only).
const EnvImagePull = "QUARSHELL_IMAGE_PULL"

// PublishedImage maps a local quarshell-sandbox:<flavor> tag to the GHCR image
// CI publishes for it, so installs work without building images locally.
func PublishedImage(local string) (string, bool) {
	flavor, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(local)), "quarshell-sandbox:")
	if !ok {
		return "", false
	}
	switch flavor {
	case "local", "base":
		return ImageBaseRef, true
	case "gui":
		return ImageGUIRef, true
	case "gpu":
		return ImageGPURef, true
	case "gui-gpu":
		return ImageGUIGPURef, true
	case "cursor":
		return ImageCursorRef, true
	case "claude":
		return ImageClaudeRef, true
	case "codex":
		return ImageCodexRef, true
	}
	return "", false
}

// Guest SSH layout (quarshell-sshd). The socket directory is a volume shared
// with the proxy sidecar (supervisor relay); sshd never listens on TCP.
const (
	GuestSSHD           = GuestRoot + "/quarshell-sshd"
	GuestSSHDir         = GuestRoot + "/ssh"
	GuestSSHSocket      = GuestSSHDir + "/sshd.sock"
	GuestSSHLog         = GuestSSHDir + "/sshd.log"
	GuestAgentSocket    = "/run/quarshell/guest-agent.sock"
	GuestAgentTokenFile = "/run/quarshell/guest-agent.token"
	GuestWorkspace      = "/workspace"
)

// HeaderBinary is an optional HTTP header naming the egress client binary (tests / ops).
const HeaderBinary = "X-QUARSHELL-Binary"

// EnvTrustBinaryHeader enables trusting HeaderBinary when set to 1/true/yes.
const EnvTrustBinaryHeader = "QUARSHELL_TRUST_BINARY_HEADER"

// RelayClientTimeout is the CLI/SDK wait for a gateway relay exec round-trip.
const RelayClientTimeout = 70 * time.Second

// ProxyEnvKeys lists host proxy variables forwarded into the sidecar (not the guest agent).
var ProxyEnvKeys = []string{
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "all_proxy", "no_proxy",
}

// ProxyListenLocal returns 127.0.0.1:<ProxyPort>.
func ProxyListenLocal() string {
	return fmt.Sprintf("127.0.0.1:%d", ProxyPort)
}

// Inference API defaults. Provider configuration can override these endpoints.
const (
	InferenceOpenAI    = "https://api.openai.com"
	InferenceAnthropic = "https://api.anthropic.com"
	InferenceNVIDIA    = "https://integrate.api.nvidia.com"
	InferenceDeepInfra = "https://api.deepinfra.com"
	InferenceOllama    = "http://" + HostInternal + ":11434"
)
