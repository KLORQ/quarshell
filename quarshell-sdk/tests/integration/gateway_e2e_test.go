package integration

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-sdk/go/klorq"
)

func TestPublicSDKAgainstLocalGatewayProcess(t *testing.T) {
	if os.Getenv("QUARSHELL_SDK_GATEWAY_E2E") != "1" {
		t.Skip("set QUARSHELL_SDK_GATEWAY_E2E=1 to run the local Gateway acceptance scenario")
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate SDK integration source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	gatewayDir := filepath.Join(root, "quarshell-gateway")
	typescriptDir := filepath.Join(root, "quarshell-proto", "typescript")
	pythonDir := filepath.Join(root, "quarshell-python")
	temp := t.TempDir()
	runCommand(t, typescriptDir, nil, "npm", "ci")
	runCommand(t, typescriptDir, nil, "npm", "run", "build")
	binary := filepath.Join(temp, "quarshell-gateway")
	build := exec.Command("go", "-C", gatewayDir, "build", "-o", binary, "./cmd/quarshell-gateway")
	build.Env = append(os.Environ(), "GOWORK="+filepath.Join(root, "go.work"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real Gateway: %v\n%s", err, output)
	}

	command := exec.Command(binary,
		"--listen", "127.0.0.1:0",
		"--grpc-listen", "127.0.0.1:0",
		"--data", filepath.Join(temp, "state.db"),
		"--runtime", "",
		"--name", "sdk-acceptance",
	)
	command.Env = append(os.Environ(), "QUARSHELL_TOKEN=quarshell-sdk-e2e-token-0123456789")
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		t.Fatalf("start Gateway: %v", err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	lines := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	httpEndpoint, grpcEndpoint := "", ""
	for httpEndpoint == "" || grpcEndpoint == "" {
		select {
		case <-ctx.Done():
			t.Fatalf("Gateway listeners did not start: http=%q grpc=%q", httpEndpoint, grpcEndpoint)
		case line, open := <-lines:
			if !open {
				t.Fatalf("Gateway exited before startup: http=%q grpc=%q", httpEndpoint, grpcEndpoint)
			}
			if strings.Contains(line, "quarshell-gateway listening on ") {
				httpEndpoint = strings.TrimSpace(strings.SplitN(line, "listening on ", 2)[1])
			}
			if strings.Contains(line, "quarshell-gateway gRPC listening on ") {
				grpcEndpoint = strings.TrimSpace(strings.SplitN(line, "gRPC listening on ", 2)[1])
			}
		}
	}

	client := klorq.NewWithToken("http://"+httpEndpoint, "quarshell-sdk-e2e-token-0123456789")
	client.SetGRPCEndpoint(grpcEndpoint)
	t.Cleanup(func() { _ = client.Close() })
	if health, err := client.Healthz(ctx); err != nil || health["status"] != "ok" {
		t.Fatalf("real Gateway health=%v err=%v", health, err)
	}
	info, err := client.Info(ctx)
	if err != nil || info["gateway_version"] != "0.0.0-dev" {
		t.Fatalf("real Gateway info=%v err=%v", info, err)
	}
	name := "sdk-acceptance-workspace"
	created, err := client.CreateWorkspaceWithLabels(ctx, name, map[string]string{"source": "sdk-e2e"})
	if err != nil || created.ID != name {
		t.Fatalf("SDK create workspace=%+v err=%v", created, err)
	}
	found, err := client.GetWorkspace(ctx, name)
	if err != nil || found.ID != name || found.Labels["source"] != "sdk-e2e" {
		t.Fatalf("SDK get workspace=%+v err=%v", found, err)
	}
	listed, err := client.ListWorkspaces(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != name {
		t.Fatalf("SDK list workspaces=%+v err=%v", listed, err)
	}
	if err := client.DeleteWorkspace(ctx, name); err != nil {
		t.Fatalf("SDK delete workspace: %v", err)
	}
	tsEnv := []string{
		"QUARSHELL_GATEWAY_URL=http://" + httpEndpoint,
		"QUARSHELL_GATEWAY_TOKEN=quarshell-sdk-e2e-token-0123456789",
	}
	runCommand(t, typescriptDir, tsEnv, "npm", "run", "test:gateway")
	pythonEnv := []string{
		"QUARSHELL_GATEWAY_URL=http://" + httpEndpoint,
		"QUARSHELL_GATEWAY_GRPC_ENDPOINT=" + grpcEndpoint,
		"QUARSHELL_GATEWAY_TOKEN=quarshell-sdk-e2e-token-0123456789",
	}
	runCommand(t, pythonDir, pythonEnv, "uv", "run", "--extra", "dev", "pytest", "-q", "tests/test_gateway_e2e.py")
}

func runCommand(t *testing.T, dir string, env []string, command string, args ...string) {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Env = commandEnvironment(env)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %s %s: %v\n%s", command, strings.Join(args, " "), err, output)
	}
}

func commandEnvironment(overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		if key, _, ok := strings.Cut(item, "="); ok {
			keys[key] = struct{}{}
		}
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if _, replaced := keys[key]; !ok || !replaced {
			env = append(env, item)
		}
	}
	return append(env, overrides...)
}
