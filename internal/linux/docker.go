package linux

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/marcospjr07/docktor/internal/check"
)

type runCommandFunc func(context.Context, string, ...string) ([]byte, error)

type dockerCheck struct {
	runCommand runCommandFunc
}

func (dockerCheck) Name() string { return "Docker" }

func (c dockerCheck) Run(ctx context.Context) check.Result {
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := c.runCommand(probeCtx, "docker", "version", "--format", "{{.Server.Version}}")
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}
	if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
		return check.Result{Status: check.StatusWarn, Message: "daemon check timed out"}
	}
	if errors.Is(err, exec.ErrNotFound) {
		return check.Result{Status: check.StatusWarn, Message: "CLI unavailable (docker not found)"}
	}
	if err != nil {
		message := strings.ToLower(string(output) + " " + err.Error())
		switch {
		case strings.Contains(message, "permission denied"), strings.Contains(message, "access denied"),
			strings.Contains(message, "unauthorized"), strings.Contains(message, "forbidden"):
			return check.Result{Status: check.StatusWarn, Message: "daemon access denied"}
		case strings.Contains(message, "cannot connect to the docker daemon"),
			strings.Contains(message, "connection refused"), strings.Contains(message, "no such file or directory"):
			return check.Result{Status: check.StatusWarn, Message: "daemon or socket unavailable"}
		default:
			return check.Result{Status: check.StatusWarn, Message: "daemon inaccessible"}
		}
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return check.Result{Status: check.StatusWarn, Message: "daemon response unavailable"}
	}
	return check.Result{Status: check.StatusPass, Message: "daemon reachable (version " + version + ")"}
}

func runDockerCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
