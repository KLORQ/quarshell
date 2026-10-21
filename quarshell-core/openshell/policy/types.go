// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package policy contains OpenShell policy domain types and wire conversions.
package policy

import (
	"fmt"
	"strings"

	sandboxv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/sandbox/v1"
	"google.golang.org/protobuf/proto"
)

// SandboxPolicy is the runtime policy assembled from the OpenShell wire model.
type SandboxPolicy struct {
	Version            uint32
	Filesystem         FilesystemPolicy
	Network            NetworkPolicy
	Landlock           LandlockPolicy
	Process            ProcessPolicy
	NetworkPolicies    map[string]*sandboxv1.NetworkPolicyRule
	NetworkMiddlewares map[string]*sandboxv1.NetworkMiddlewareConfig
}

// FilesystemPolicy contains read allowlists and workdir behavior.
type FilesystemPolicy struct {
	ReadOnly       []string
	ReadWrite      []string
	IncludeWorkdir bool
}

// NetworkPolicy describes the fixed cluster egress mode.
type NetworkPolicy struct {
	Mode  NetworkMode
	Proxy *ProxyPolicy
}

// NetworkMode is the runtime's egress decision mode.
type NetworkMode uint8

const (
	NetworkBlock NetworkMode = iota
	NetworkProxy
	NetworkAllow
)

// ProxyPolicy holds a loopback proxy endpoint when one is assigned.
type ProxyPolicy struct {
	HTTPAddress string
}

// LandlockPolicy controls whether unsupported Landlock is tolerated.
type LandlockPolicy struct {
	Compatibility LandlockCompatibility
}

// LandlockCompatibility describes Landlock support requirements.
type LandlockCompatibility uint8

const (
	LandlockBestEffort LandlockCompatibility = iota
	LandlockHardRequirement
)

// ProcessPolicy selects the sandbox process identity.
type ProcessPolicy struct {
	RunAsUser  *string
	RunAsGroup *string
}

// LandlockCompatibilityValues are accepted wire values.
var LandlockCompatibilityValues = [...]string{"best_effort", "hard_requirement"}

// IsValidLandlockCompatibility reports whether a wire value is accepted. Empty
// means best_effort for compatibility with omitted configuration.
func IsValidLandlockCompatibility(value string) bool {
	return value == "" || value == "best_effort" || value == "hard_requirement"
}

// FromProto converts OpenShell's sandbox policy to runtime domain types. Cluster
// mode always routes egress through the policy-aware proxy.
func FromProto(input *sandboxv1.SandboxPolicy) (SandboxPolicy, error) {
	result := SandboxPolicy{
		Filesystem:         FilesystemPolicy{IncludeWorkdir: true},
		Network:            NetworkPolicy{Mode: NetworkProxy, Proxy: &ProxyPolicy{}},
		Landlock:           LandlockPolicy{Compatibility: LandlockBestEffort},
		NetworkPolicies:    make(map[string]*sandboxv1.NetworkPolicyRule),
		NetworkMiddlewares: make(map[string]*sandboxv1.NetworkMiddlewareConfig),
	}
	if input == nil {
		return result, nil
	}
	result.Version = input.Version
	if fs := input.Filesystem; fs != nil {
		result.Filesystem.IncludeWorkdir = fs.IncludeWorkdir
		result.Filesystem.ReadOnly = normalizePaths(fs.ReadOnly)
		result.Filesystem.ReadWrite = normalizePaths(fs.ReadWrite)
	}
	if landlock := input.Landlock; landlock != nil {
		switch landlock.Compatibility {
		case "", "best_effort":
			result.Landlock.Compatibility = LandlockBestEffort
		case "hard_requirement":
			result.Landlock.Compatibility = LandlockHardRequirement
		default:
			return SandboxPolicy{}, fmt.Errorf("invalid landlock.compatibility %q; accepted: %s", landlock.Compatibility, strings.Join(LandlockCompatibilityValues[:], ", "))
		}
	}
	if process := input.Process; process != nil {
		if process.RunAsUser != "" {
			result.Process.RunAsUser = stringPointer(process.RunAsUser)
		}
		if process.RunAsGroup != "" {
			result.Process.RunAsGroup = stringPointer(process.RunAsGroup)
		}
	}
	for key, rule := range input.NetworkPolicies {
		if rule == nil {
			result.NetworkPolicies[key] = nil
			continue
		}
		result.NetworkPolicies[key] = proto.Clone(rule).(*sandboxv1.NetworkPolicyRule)
	}
	for key, middleware := range input.NetworkMiddlewares {
		if middleware == nil {
			result.NetworkMiddlewares[key] = nil
			continue
		}
		result.NetworkMiddlewares[key] = proto.Clone(middleware).(*sandboxv1.NetworkMiddlewareConfig)
	}
	return result, nil
}

func stringPointer(value string) *string { return &value }

func normalizePaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = normalizePath(path)
	}
	return out
}

// normalizePath mirrors OpenShell's lexical normalization without filesystem
// access or symlink resolution. Parent components are intentionally preserved.
func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	absolute := strings.HasPrefix(path, "/")
	parts := strings.Split(path, "/")
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		default:
			stack = append(stack, part)
		}
	}
	joined := strings.Join(stack, "/")
	if absolute {
		return "/" + joined
	}
	return joined
}
