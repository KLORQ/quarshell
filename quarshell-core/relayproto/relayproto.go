// Package relayproto contains small shared constants and byte-stream plumbing
// used by the OpenShell supervisor and TCP forwarding implementations. Wire
// control and authorization are defined by the pinned OpenShell protobuf API.
package relayproto

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	// TargetSSH is the supervisor's local SSH socket target.
	TargetSSH = "ssh"

	KeepaliveInterval = 15 * time.Second
	KeepaliveTimeout  = 45 * time.Second
)

// Pipe copies bytes both ways until both directions finish, half-closing
// writers on EOF, then closes both streams.
func Pipe(a, b io.ReadWriteCloser) error {
	pipeErrors := make(chan error, 6)
	report := func(op string, err error) {
		if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
			return
		}
		pipeErrors <- fmt.Errorf("%s: %w", op, err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	cp := func(direction string, dst, src io.ReadWriteCloser) {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		report("copy "+direction, err)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			report("half-close "+direction, cw.CloseWrite())
		} else {
			report("close "+direction, dst.Close())
		}
	}
	go cp("a-to-b", b, a)
	go cp("b-to-a", a, b)
	wg.Wait()
	report("close stream a", a.Close())
	report("close stream b", b.Close())
	close(pipeErrors)
	var err error
	for pipeErr := range pipeErrors {
		err = errors.Join(err, pipeErr)
	}
	return err
}
