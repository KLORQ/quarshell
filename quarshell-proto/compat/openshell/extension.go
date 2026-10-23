// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openshell

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	extensionv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/extension/v1"
)

const (
	ProtocolMajor                  uint32 = 1
	ProtocolMinor                  uint32 = 0
	ComputeLaunchAuthentication           = "openshell.compute.launch-authentication"
	maxImplementationNameBytes            = 128
	maxImplementationVersionBytes         = 128
	maxCapabilityBytes                    = 128
	maxCapabilities                       = 128
	RegexMiddlewareName                   = "openshell/regex"
	RegexMiddlewareMaxPayloadBytes uint64 = 256 << 10
)

// ExtensionFamily identifies an OpenShell extension contract.
type ExtensionFamily string

const (
	ExtensionCompute              ExtensionFamily = "compute"
	ExtensionCredentials          ExtensionFamily = "credentials"
	ExtensionGatewayInterceptor   ExtensionFamily = "gateway-interceptor"
	ExtensionSupervisorMiddleware ExtensionFamily = "supervisor-middleware"
)

func (f ExtensionFamily) contractCapability() string {
	return "openshell." + string(f) + ".contract"
}

func (f ExtensionFamily) valid() bool {
	switch f {
	case ExtensionCompute, ExtensionCredentials, ExtensionGatewayInterceptor, ExtensionSupervisorMiddleware:
		return true
	default:
		return false
	}
}

// NegotiatedExtension is the validated peer contract returned to a caller.
type NegotiatedExtension struct {
	Family                ExtensionFamily
	ConfiguredName        string
	ImplementationName    string
	ImplementationVersion string
	ProtocolMajor         uint32
	ProtocolMinor         uint32
	SupportedCapabilities []string
	RequiredCapabilities  []string
}

// NegotiationError identifies why an extension contract could not be accepted.
type NegotiationError struct {
	Kind        string
	Family      ExtensionFamily
	Name        string
	Field       string
	Reason      string
	Expected    string
	RemoteMajor uint32
	RemoteMinor uint32
	LocalMajor  uint32
	LocalMinor  uint32
	Missing     []string
}

func (e *NegotiationError) Error() string {
	if e == nil {
		return "extension negotiation failed"
	}
	family, name := string(e.Family), e.Name
	switch e.Kind {
	case "missing-metadata":
		return fmt.Sprintf("%s extension %q did not provide protocol metadata; upgrade the extension to a version that supports OpenShell extension negotiation (expected %s)", family, name, e.Expected)
	case "missing-gateway-metadata":
		return fmt.Sprintf("gateway did not provide protocol metadata to %s extension %q; upgrade the gateway and extension together", family, name)
	case "missing-protocol-version":
		return fmt.Sprintf("%s extension %q did not provide a protocol version (expected %s)", family, name, e.Expected)
	case "incompatible-protocol":
		return fmt.Sprintf("%s extension %q uses unsupported protocol %d.%d; gateway supports %d.%d", family, name, e.RemoteMajor, e.RemoteMinor, e.LocalMajor, e.LocalMinor)
	case "invalid-metadata":
		return fmt.Sprintf("%s extension %q has invalid %s: %s", family, name, e.Field, e.Reason)
	case "missing-extension-capabilities":
		return fmt.Sprintf("%s extension %q is missing required capabilities: %s", family, name, strings.Join(e.Missing, ", "))
	case "missing-gateway-capabilities":
		return fmt.Sprintf("gateway is missing capabilities required by %s extension %q: %s", family, name, strings.Join(e.Missing, ", "))
	default:
		return "extension negotiation failed"
	}
}

// GatewayMetadata declares the gateway side of a family-specific contract.
func GatewayMetadata(family ExtensionFamily, implementationVersion string) *extensionv1.PeerMetadata {
	contract := family.contractCapability()
	supported := []string{contract}
	if family == ExtensionCompute {
		supported = append(supported, ComputeLaunchAuthentication)
	}
	return &extensionv1.PeerMetadata{
		ProtocolVersion:    &extensionv1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
		ImplementationName: "openshell/gateway", ImplementationVersion: implementationVersion,
		SupportedCapabilities: supported, RequiredCapabilities: []string{contract},
	}
}

// ExtensionMetadata builds metadata for a peer that supports its family's base contract.
func ExtensionMetadata(family ExtensionFamily, name, version string, additional ...string) *extensionv1.PeerMetadata {
	contract := family.contractCapability()
	supported := []string{contract}
	supported = append(supported, additional...)
	return &extensionv1.PeerMetadata{
		ProtocolVersion:    &extensionv1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
		ImplementationName: name, ImplementationVersion: version,
		SupportedCapabilities: supported, RequiredCapabilities: []string{contract},
	}
}

// Negotiate validates protocol versions and both peers' required capabilities.
func Negotiate(family ExtensionFamily, configuredName string, gateway, peer *extensionv1.PeerMetadata) (*NegotiatedExtension, error) {
	if !family.valid() {
		return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: configuredName, Field: "family", Reason: "is not a supported extension family"}
	}
	expected := fmt.Sprintf("protocol %d.%d and capability %s", ProtocolMajor, ProtocolMinor, family.contractCapability())
	if peer == nil {
		return nil, &NegotiationError{Kind: "missing-metadata", Family: family, Name: configuredName, Expected: expected}
	}
	if peer.ProtocolVersion == nil {
		return nil, &NegotiationError{Kind: "missing-protocol-version", Family: family, Name: configuredName, Expected: expected}
	}
	if gateway == nil {
		return nil, &NegotiationError{Kind: "missing-gateway-metadata", Family: family, Name: "gateway"}
	}
	if gateway.ProtocolVersion == nil {
		return nil, &NegotiationError{Kind: "missing-protocol-version", Family: family, Name: "gateway", Expected: expected}
	}
	version := peer.ProtocolVersion
	gatewayVersion := gateway.ProtocolVersion
	if version.Major != gatewayVersion.Major {
		return nil, &NegotiationError{Kind: "incompatible-protocol", Family: family, Name: configuredName,
			RemoteMajor: version.Major, RemoteMinor: version.Minor, LocalMajor: gatewayVersion.Major, LocalMinor: gatewayVersion.Minor}
	}
	if err := validateText(family, configuredName, "implementation_name", peer.ImplementationName, maxImplementationNameBytes); err != nil {
		return nil, err
	}
	if err := validateText(family, configuredName, "implementation_version", peer.ImplementationVersion, maxImplementationVersionBytes); err != nil {
		return nil, err
	}
	peerSupported, err := normalizeCapabilities(family, configuredName, "supported_capabilities", peer.SupportedCapabilities)
	if err != nil {
		return nil, err
	}
	peerRequired, err := normalizeCapabilities(family, configuredName, "required_capabilities", peer.RequiredCapabilities)
	if err != nil {
		return nil, err
	}
	gatewaySupported, err := normalizeCapabilities(family, "gateway", "supported_capabilities", gateway.SupportedCapabilities)
	if err != nil {
		return nil, err
	}
	gatewayRequired, err := normalizeCapabilities(family, "gateway", "required_capabilities", gateway.RequiredCapabilities)
	if err != nil {
		return nil, err
	}
	contract := family.contractCapability()
	if !slices.Contains(peerSupported, contract) {
		return nil, &NegotiationError{Kind: "missing-extension-capabilities", Family: family, Name: configuredName, Missing: []string{contract}}
	}
	if !slices.Contains(peerRequired, contract) {
		return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: configuredName, Field: "required_capabilities", Reason: "must include " + contract}
	}
	if !slices.Contains(gatewaySupported, contract) {
		return nil, &NegotiationError{Kind: "missing-gateway-capabilities", Family: family, Name: "gateway", Missing: []string{contract}}
	}
	if !slices.Contains(gatewayRequired, contract) {
		return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: "gateway", Field: "required_capabilities", Reason: "must include " + contract}
	}
	if missing := difference(gatewayRequired, peerSupported); len(missing) > 0 {
		return nil, &NegotiationError{Kind: "missing-extension-capabilities", Family: family, Name: configuredName, Missing: missing}
	}
	if missing := difference(peerRequired, gatewaySupported); len(missing) > 0 {
		return nil, &NegotiationError{Kind: "missing-gateway-capabilities", Family: family, Name: configuredName, Missing: missing}
	}
	return &NegotiatedExtension{Family: family, ConfiguredName: configuredName,
		ImplementationName: peer.ImplementationName, ImplementationVersion: peer.ImplementationVersion,
		ProtocolMajor: version.Major, ProtocolMinor: version.Minor,
		SupportedCapabilities: peerSupported, RequiredCapabilities: peerRequired}, nil
}

// ValidateGatewayMetadata validates the gateway handshake metadata returned by a peer.
func ValidateGatewayMetadata(family ExtensionFamily, name string, peer, gateway *extensionv1.PeerMetadata) error {
	if gateway == nil {
		return &NegotiationError{Kind: "missing-gateway-metadata", Family: family, Name: name}
	}
	_, err := Negotiate(family, name, gateway, peer)
	return err
}

func validateText(family ExtensionFamily, name, field, value string, maxBytes int) error {
	reason := ""
	switch {
	case strings.TrimSpace(value) == "":
		reason = "must not be empty"
	case len([]byte(value)) > maxBytes:
		reason = fmt.Sprintf("must be at most %d bytes", maxBytes)
	case strings.IndexFunc(value, unicode.IsControl) >= 0:
		reason = "must not contain control characters"
	}
	if reason != "" {
		return &NegotiationError{Kind: "invalid-metadata", Family: family, Name: name, Field: field, Reason: reason}
	}
	return nil
}

func normalizeCapabilities(family ExtensionFamily, name, field string, values []string) ([]string, error) {
	if len(values) > maxCapabilities {
		return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: name, Field: field, Reason: fmt.Sprintf("must contain at most %d entries", maxCapabilities)}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !validCapability(value) {
			return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: name, Field: field, Reason: fmt.Sprintf("%q must be a lowercase namespaced identifier", value)}
		}
		if _, exists := seen[value]; exists {
			return nil, &NegotiationError{Kind: "invalid-metadata", Family: family, Name: name, Field: field, Reason: fmt.Sprintf("contains duplicate %q", value)}
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func validCapability(value string) bool {
	if value == "" || len([]byte(value)) > maxCapabilityBytes || !strings.HasPrefix(value, "openshell.") {
		return false
	}
	segments := strings.Split(value, ".")
	if len(segments) < 3 {
		return false
	}
	for _, segment := range segments {
		if segment == "" {
			return false
		}
		for _, b := range []byte(segment) {
			if (b < 'a' || b > 'z') && (b < '0' || b > '9') && b != '-' {
				return false
			}
		}
	}
	return true
}

func difference(required, supported []string) []string {
	set := make(map[string]struct{}, len(supported))
	for _, value := range supported {
		set[value] = struct{}{}
	}
	missing := make([]string, 0)
	for _, value := range required {
		if _, ok := set[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}
