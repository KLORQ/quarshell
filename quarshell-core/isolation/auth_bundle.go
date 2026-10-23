package isolation

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	MaxAuthBundleBytes = 64 << 10
)

// HostAuthBundle matches OpenShell v0.1.4's SupervisorAuthBundle JSON schema.
// Secrets intentionally have no String or logging representation.
type HostAuthBundle struct {
	SessionID         string  `json:"session_id"`
	RuntimeGeneration string  `json:"runtime_generation"`
	SessionRotation   uint64  `json:"session_rotation"`
	AuthEpoch         uint64  `json:"auth_epoch"`
	GatewayToken      string  `json:"gateway_token"`
	GatewayExpiresAt  int64   `json:"gateway_expires_at"`
	SandboxToken      string  `json:"sandbox_token"`
	SandboxExpiresAt  int64   `json:"sandbox_expires_at"`
	SSHHostPrivateKey *string `json:"ssh_host_private_key,omitempty"`
}

func EncodeHostAuthBundle(bundle HostAuthBundle, descriptor Descriptor) ([]byte, error) {
	if _, err := EncodeDescriptor(descriptor); err != nil {
		return nil, fmt.Errorf("invalid isolation descriptor for host auth bundle: %w", err)
	}
	if !validHostAuthBundle(bundle) {
		return nil, fmt.Errorf("invalid host supervisor auth bundle")
	}
	if bundle.SessionID != descriptor.SessionID {
		return nil, fmt.Errorf("host supervisor auth bundle does not match isolation descriptor")
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("encode host supervisor auth bundle: %w", err)
	}
	if len(encoded) > MaxAuthBundleBytes {
		return nil, fmt.Errorf("host supervisor auth bundle exceeds %d bytes", MaxAuthBundleBytes)
	}
	return encoded, nil
}

func DecodeHostAuthBundle(raw []byte, descriptor Descriptor) (HostAuthBundle, error) {
	var bundle HostAuthBundle
	if _, err := EncodeDescriptor(descriptor); err != nil {
		return bundle, fmt.Errorf("invalid isolation descriptor for host auth bundle: %w", err)
	}
	if len(raw) == 0 || len(raw) > MaxAuthBundleBytes {
		return bundle, fmt.Errorf("host supervisor auth bundle has invalid size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return HostAuthBundle{}, fmt.Errorf("decode host supervisor auth bundle: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return HostAuthBundle{}, fmt.Errorf("host supervisor auth bundle contains trailing data")
	}
	if !validHostAuthBundle(bundle) {
		return HostAuthBundle{}, fmt.Errorf("invalid host supervisor auth bundle")
	}
	if bundle.SessionID != descriptor.SessionID {
		return HostAuthBundle{}, fmt.Errorf("host supervisor auth bundle does not match isolation descriptor")
	}
	return bundle, nil
}

func validHostAuthBundle(bundle HostAuthBundle) bool {
	for _, token := range []string{bundle.GatewayToken, bundle.SandboxToken} {
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n\x00") {
			return false
		}
	}
	return canonicalOpenShellSessionID(bundle.SessionID) && canonicalOpenShellGeneration(bundle.RuntimeGeneration) && bundle.SessionRotation > 0 && bundle.AuthEpoch > 0 && bundle.GatewayExpiresAt >= 0 && bundle.SandboxExpiresAt >= 0 && (bundle.SSHHostPrivateKey == nil || !strings.ContainsRune(*bundle.SSHHostPrivateKey, '\x00'))
}

func canonicalOpenShellSessionID(value string) bool {
	if len(value) != 36 || strings.ToLower(value) != value || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return false
	}
	for _, b := range decoded {
		if b != 0 {
			return true
		}
	}
	return false
}

func canonicalOpenShellGeneration(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, b := range []byte(value) {
		isLower := b >= 'a' && b <= 'z'
		isDigit := b >= '0' && b <= '9'
		if !isLower && !isDigit && b != '-' {
			return false
		}
	}
	return true
}
