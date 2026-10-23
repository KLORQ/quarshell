//go:build linux

package vm

import (
	"bytes"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkerUserNamespaceCanConnectPrivateHostUnixSocket(t *testing.T) {
	if os.Getenv("CAUTEM_VM_SOCKET_NAMESPACE_CHILD") == "1" {
		if err := prepareWorkerIsolation(workerIsolationConfig{
			HostMountNamespace: os.Getenv("CAUTEM_VM_PARENT_MOUNT_NS"),
			HostUserNamespace:  os.Getenv("CAUTEM_VM_PARENT_USER_NS"),
		}); err != nil {
			t.Fatalf("prepare worker namespace: %v", err)
		}
		connection, err := net.DialTimeout("unix", os.Getenv("CAUTEM_VM_NAMESPACE_SOCKET"), 2*time.Second)
		if err != nil {
			t.Fatalf("connect host Unix socket from worker namespace (uid/euid %d/%d): %v", os.Getuid(), os.Geteuid(), err)
		}
		_ = connection.Close()
		return
	}
	if os.Getuid() == 0 || os.Getgid() == 0 {
		t.Skip("worker namespace socket probe requires a non-root host identity")
	}

	socketPath := filepath.Join(t.TempDir(), "host.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatal(err)
	}
	parent, err := workerIsolationForChild()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestWorkerUserNamespaceCanConnectPrivateHostUnixSocket$")
	command.Env = append(os.Environ(),
		"CAUTEM_VM_SOCKET_NAMESPACE_CHILD=1",
		"CAUTEM_VM_PARENT_MOUNT_NS="+parent.HostMountNamespace,
		"CAUTEM_VM_PARENT_USER_NS="+parent.HostUserNamespace,
		"CAUTEM_VM_NAMESPACE_SOCKET="+socketPath,
	)
	command.SysProcAttr = workerProcessAttributes()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		t.Fatalf("worker namespace socket probe failed: %v\n%s", err, output.String())
	}
	_ = listener.(*net.UnixListener).SetDeadline(time.Now().Add(time.Second))
	connection, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept worker namespace socket probe: %v", err)
	}
	_ = connection.Close()
}

func TestWorkerProcessAttributesRequirePrivateUserAndMountNamespaces(t *testing.T) {
	attributes := workerProcessAttributes()
	if attributes == nil {
		t.Fatal("worker process attributes are nil")
	}
	if attributes.Cloneflags&(unix.CLONE_NEWUSER|unix.CLONE_NEWNS) != unix.CLONE_NEWUSER|unix.CLONE_NEWNS {
		t.Fatalf("worker clone flags = %#x; want user and mount namespaces", attributes.Cloneflags)
	}
	if len(attributes.UidMappings) != 1 || attributes.UidMappings[0] != (syscall.SysProcIDMap{ContainerID: 0, HostID: os.Getuid(), Size: 1}) {
		t.Fatalf("worker UID mappings = %+v", attributes.UidMappings)
	}
	if len(attributes.GidMappings) != 1 || attributes.GidMappings[0] != (syscall.SysProcIDMap{ContainerID: 0, HostID: os.Getgid(), Size: 1}) {
		t.Fatalf("worker GID mappings = %+v", attributes.GidMappings)
	}
	if attributes.GidMappingsEnableSetgroups {
		t.Fatal("worker user namespace must deny setgroups")
	}
}

func TestPrepareWorkerIsolationFailsClosedWithoutNamespaceIdentity(t *testing.T) {
	if err := prepareWorkerIsolation(workerIsolationConfig{}); err == nil {
		t.Fatal("missing namespace identities unexpectedly accepted")
	}
}

func TestMountInfoIsPrivateDetectsPropagationPeers(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "private", line: "29 23 0:26 / / rw,relatime - overlay overlay rw", want: true},
		{name: "shared", line: "29 23 0:26 / / rw,relatime shared:1 - overlay overlay rw", want: false},
		{name: "slave", line: "29 23 0:26 / / rw,relatime master:1 - overlay overlay rw", want: false},
		{name: "propagate-from", line: "29 23 0:26 / / rw,relatime propagate_from:1 - overlay overlay rw", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "mountinfo")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString(test.line + "\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			private, err := mountInfoIsPrivate(file)
			if err != nil {
				t.Fatal(err)
			}
			if private != test.want {
				t.Fatalf("mountInfoIsPrivate() = %t, want %t", private, test.want)
			}
		})
	}
}

func TestWorkerIsolationNamespaceSubprocess(t *testing.T) {
	if os.Getuid() == 0 || os.Getgid() == 0 {
		t.Skip("worker isolation probe requires a non-root host identity")
	}
	if os.Getenv("CAUTEM_VM_NAMESPACE_CHILD") == "1" {
		config := workerIsolationConfig{
			HostMountNamespace: os.Getenv("CAUTEM_VM_PARENT_MOUNT_NS"),
			HostUserNamespace:  os.Getenv("CAUTEM_VM_PARENT_USER_NS"),
		}
		if err := prepareWorkerIsolation(config); err != nil {
			t.Fatalf("prepare worker namespace: %v", err)
		}
		stopWatch, err := startParentLivenessWatch(3)
		if err != nil {
			t.Fatalf("start parent liveness watcher after isolation setup: %v", err)
		}
		defer stopWatch()
		if os.Geteuid() != 0 || os.Getegid() != 0 {
			t.Fatalf("worker namespace identity = %d:%d; want mapped namespace root", os.Geteuid(), os.Getegid())
		}
		return
	}

	parent, err := workerIsolationForChild()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestWorkerIsolationNamespaceSubprocess$")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	command.ExtraFiles = []*os.File{reader}
	command.Env = append(os.Environ(),
		"CAUTEM_VM_NAMESPACE_CHILD=1",
		"CAUTEM_VM_PARENT_MOUNT_NS="+parent.HostMountNamespace,
		"CAUTEM_VM_PARENT_USER_NS="+parent.HostUserNamespace,
	)
	command.SysProcAttr = workerProcessAttributes()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		_ = reader.Close()
		t.Fatalf("start isolated worker subprocess: %v", err)
	}
	_ = reader.Close()
	err = command.Wait()
	if err != nil {
		if errors.Is(err, syscall.EPERM) || strings.Contains(strings.ToLower(err.Error()), "operation not permitted") || strings.Contains(strings.ToLower(output.String()), "operation not permitted") {
			t.Skipf("host blocks unprivileged user namespaces; live VM create must fail closed here: %v", err)
		}
		t.Fatalf("isolated worker subprocess failed: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "PASS") {
		t.Fatalf("isolated worker subprocess did not pass its probe: %s", output.String())
	}
}

func TestNamespaceIdentityRejectsUnexpectedValue(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "namespace")
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if _, err := namespaceIdentity(file.Name()); err == nil {
		t.Fatal("regular file unexpectedly accepted as a namespace identity")
	}
}
