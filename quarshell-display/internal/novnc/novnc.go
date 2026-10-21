// Package novnc documents the in-guest noVNC stack (started by cautem-gui-boot).
// Host-side "start" is satisfied by Docker port publish + guest supervisor;
// this type remains for API symmetry / future CDP hybrids.
package novnc

import (
	"context"
	"fmt"

	"github.com/KLORQ/quarshell-display"
)

// Stack reports the URL for an already-published guest noVNC port.
type Stack struct {
	Host string
	Port int
	Pass string
}

// Start returns the operator URL (guest stack must already be running).
func (s Stack) Start(_ context.Context, cfg display.Config) (string, error) {
	pass := s.Pass
	if pass == "" {
		pass = cfg.Password
	}
	port := s.Port
	if port <= 0 {
		port = cfg.Port
	}
	if port <= 0 {
		port = display.DefaultPort
	}
	host := s.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if cfg.Mode != display.ModeNoVNC && cfg.Mode != "" {
		return "", fmt.Errorf("novnc: unsupported mode %q", cfg.Mode)
	}
	return display.NoVNCURL(host, port, pass), nil
}

// Stop is a no-op (container lifecycle owns the stack).
func (Stack) Stop(_ context.Context) error { return nil }
