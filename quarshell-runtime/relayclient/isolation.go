package relayclient

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/KLORQ/quarshell-core/isolation"
	"github.com/KLORQ/quarshell-runtime/supervisorcontrol"
)

// validateIsolationDescriptor makes the host supervisor consume the descriptor
// provisioned by its compute driver before it opens the Gateway control stream.
// The VM's per-boot SPIFFE identity must describe the same boundary.
func validateIsolationDescriptor(cfg Config) error {
	path := strings.TrimSpace(cfg.IsolationDescriptorPath)
	if path == "" {
		if strings.TrimSpace(cfg.SupervisorControlExpectedIdentity) != "" {
			return fmt.Errorf("relayclient: VM supervisor identity requires an isolation descriptor")
		}
		return nil
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("relayclient: isolation descriptor path must be a clean absolute path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || info.Size() <= 0 || info.Size() > isolation.MaxDescriptorBytes {
		return fmt.Errorf("relayclient: isolation descriptor must be a private regular mode-0600 file")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("relayclient: open isolation descriptor: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 {
		return fmt.Errorf("relayclient: isolation descriptor changed while being opened")
	}
	raw, err := io.ReadAll(io.LimitReader(file, isolation.MaxDescriptorBytes+1))
	if err != nil || len(raw) > isolation.MaxDescriptorBytes {
		return fmt.Errorf("relayclient: read isolation descriptor: invalid size or read failure")
	}
	descriptor, err := isolation.DecodeDescriptor(raw)
	if err != nil {
		return fmt.Errorf("relayclient: decode isolation descriptor: %w", err)
	}
	backend := strings.TrimSpace(cfg.AdmittedIsolationBackend)
	if backend == "" || descriptor.BackendName != backend || descriptor.SandboxID != strings.TrimSpace(cfg.Sandbox) {
		return fmt.Errorf("relayclient: isolation descriptor does not match admitted backend and sandbox")
	}
	identity, err := supervisorcontrol.ParseVMIdentity(strings.TrimSpace(cfg.SupervisorControlExpectedIdentity))
	if err != nil || identity.SandboxID != descriptor.SandboxID || identity.SessionID != descriptor.SessionID || identity.Generation != descriptor.Generation || identity.ImageDigest != descriptor.RootfsDigest || identity.RuntimeDigest != descriptor.RuntimeDigest {
		return fmt.Errorf("relayclient: VM supervisor identity does not match isolation descriptor")
	}
	return nil
}
