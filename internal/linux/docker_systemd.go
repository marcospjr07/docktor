package linux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/marcospjr07/docktor/internal/check"
)

const dockerSystemdTimeout = 2 * time.Second

type dockerSystemdScope uint8

const (
	dockerNoSystemd dockerSystemdScope = iota
	dockerSystemSystemd
	dockerUserSystemd
	dockerUnknownSystemd
)

type dockerUnitStateFunc func(context.Context, dockerSystemdScope, string) (string, error)

func dockerSocketSystemdScope(path, runtimeDir string) dockerSystemdScope {
	rawPath := filepath.Clean(path)
	path = canonicalDockerSocket(path)
	if isSystemDockerSocket(rawPath) || isSystemDockerSocket(path) {
		return dockerSystemSystemd
	}
	if filepath.IsAbs(runtimeDir) && path == canonicalDockerSocket(filepath.Join(runtimeDir, "docker.sock")) {
		// A user manager is authoritative only for its own runtime directory.
		info, err := os.Stat(runtimeDir)
		if err != nil || !info.IsDir() || os.Getuid() != os.Geteuid() {
			return dockerUnknownSystemd
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || owner.Uid != uint32(os.Getuid()) {
			return dockerUnknownSystemd
		}
		return dockerUserSystemd
	}
	return dockerNoSystemd
}

func isSystemDockerSocket(path string) bool {
	return path == "/run/docker.sock" || path == "/var/run/docker.sock"
}

func canonicalDockerSocket(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func (c dockerCheck) socketActivationWarning(ctx context.Context) (check.Result, bool) {
	if c.scope == dockerNoSystemd {
		return check.Result{}, false
	}
	// An unknown unit state cannot rule out activation, so skip the connection.
	unknown := dockerWarning("local Docker service state unavailable; probe skipped to avoid socket activation")
	if c.scope == dockerUnknownSystemd || c.unitState == nil {
		return unknown, true
	}

	stateCtx, cancel := context.WithTimeout(ctx, dockerSystemdTimeout)
	defer cancel()
	socketState, err := c.unitState(stateCtx, c.scope, "docker.socket")
	if err != nil || stateCtx.Err() != nil {
		return unknown, true
	}
	switch socketState {
	case "inactive", "failed":
		return check.Result{}, false
	case "active":
		// An active socket can start docker.service on the first connection.
	default:
		return unknown, true
	}

	serviceState, err := c.unitState(stateCtx, c.scope, "docker.service")
	if err != nil || stateCtx.Err() != nil {
		return unknown, true
	}
	switch serviceState {
	case "active":
		return check.Result{}, false
	case "inactive", "failed":
		return dockerWarning("local Docker daemon is not running; probe skipped to avoid socket activation"), true
	case "activating", "deactivating", "reloading":
		return dockerWarning("local Docker service is changing state; probe skipped to avoid socket activation"), true
	default:
		return unknown, true
	}
}

func systemctlDockerUnitState(ctx context.Context, scope dockerSystemdScope, unit string) (string, error) {
	var flag string
	switch scope {
	case dockerSystemSystemd:
		flag = "--system"
	case dockerUserSystemd:
		flag = "--user"
	default:
		return "", fmt.Errorf("invalid Docker systemd scope %d", scope)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/systemctl", flag, "show", "--property=ActiveState", "--value", "--no-pager", "--", unit)
	// Inherited D-Bus addresses must not redirect this state check to another manager.
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "DBUS_SESSION_BUS_ADDRESS=") || strings.HasPrefix(entry, "DBUS_SYSTEM_BUS_ADDRESS=") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.WaitDelay = 200 * time.Millisecond
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("systemctl show %s: %w", unit, err)
	}
	if stderr.Len() != 0 {
		return "", fmt.Errorf("systemctl show %s wrote to stderr", unit)
	}
	state := strings.TrimSpace(stdout.String())
	if state == "" || strings.ContainsAny(state, "\r\n") {
		return "", fmt.Errorf("systemctl show %s returned an invalid state", unit)
	}
	return state, nil
}
