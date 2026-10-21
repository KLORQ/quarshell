package relayproto

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestPipeBridgesBothWays(t *testing.T) {
	a1, a2 := net.Pipe()
	b1, b2 := net.Pipe()
	done := make(chan struct{})
	go func() {
		Pipe(a2, b1)
		close(done)
	}()
	go func() {
		buf := make([]byte, 4)
		_, _ = io.ReadFull(b2, buf)
		_, _ = b2.Write(bytes.ToUpper(buf))
		_ = b2.Close()
	}()
	if _, err := a1.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(a1, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "PING" {
		t.Fatalf("got %q", buf)
	}
	_ = a1.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipe did not finish")
	}
}

type readErrorConn struct{ err error }

func (c readErrorConn) Read([]byte) (int, error)  { return 0, c.err }
func (readErrorConn) Write(p []byte) (int, error) { return len(p), nil }
func (readErrorConn) Close() error                { return nil }

func TestPipeReturnsCopyErrors(t *testing.T) {
	want := errors.New("read failed")
	if err := Pipe(readErrorConn{err: want}, readErrorConn{err: want}); !errors.Is(err, want) {
		t.Fatalf("Pipe() error = %v, want wrapped %v", err, want)
	}
}
