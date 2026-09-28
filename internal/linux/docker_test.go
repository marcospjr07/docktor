package linux

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/marcospjr07/docktor/internal/check"
)

func TestDockerCheck(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		err        error
		wantStatus check.Status
		wantText   string
	}{
		{"reachable", "27.3.1\n", nil, check.StatusPass, "daemon reachable (version 27.3.1)"},
		{"CLI missing", "", &exec.Error{Name: "docker", Err: exec.ErrNotFound}, check.StatusWarn, "CLI unavailable"},
		{"daemon down", "Cannot connect to the Docker daemon at unix:///var/run/docker.sock", errors.New("exit status 1"), check.StatusWarn, "daemon or socket unavailable"},
		{"socket missing", "dial unix /var/run/docker.sock: no such file or directory", errors.New("exit status 1"), check.StatusWarn, "daemon or socket unavailable"},
		{"permission denied", "permission denied while trying to connect to the Docker daemon socket", errors.New("exit status 1"), check.StatusWarn, "daemon access denied"},
		{"other failure", "unexpected response", errors.New("exit status 1"), check.StatusWarn, "daemon inaccessible"},
		{"empty version", " \n", nil, check.StatusWarn, "daemon response unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := dockerCheck{runCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if ctx.Err() != nil {
					t.Errorf("command context already canceled: %v", ctx.Err())
				}
				if name != "docker" || len(args) != 3 || args[0] != "version" || args[1] != "--format" || args[2] != "{{.Server.Version}}" {
					t.Errorf("unexpected command: %q %q", name, args)
				}
				return []byte(test.output), test.err
			}}
			result := c.Run(context.Background())
			if result.Status != test.wantStatus || !strings.Contains(result.Message, test.wantText) {
				t.Fatalf("Run() = %+v; want %s containing %q", result, test.wantStatus, test.wantText)
			}
		})
	}
}

func TestDockerCheckCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := dockerCheck{runCommand: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("command ran after cancellation")
		return nil, nil
	}}
	if result := c.Run(ctx); result.Status != check.StatusWarn || !strings.Contains(result.Message, "scan interrupted") {
		t.Fatalf("Run() = %+v; want interruption warning", result)
	}

	ctx, cancel = context.WithCancel(context.Background())
	c.runCommand = func(context.Context, string, ...string) ([]byte, error) {
		cancel()
		return nil, context.Canceled
	}
	if result := c.Run(ctx); result.Status != check.StatusWarn || !strings.Contains(result.Message, "scan interrupted") {
		t.Fatalf("Run() = %+v; want interruption warning", result)
	}
}

func TestDockerCheckRegistered(t *testing.T) {
	for _, diagnostic := range Checks() {
		if diagnostic.Name() == "Docker" {
			return
		}
	}
	t.Fatal("Docker check is not registered")
}
