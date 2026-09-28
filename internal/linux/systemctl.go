package linux

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type systemctlRunner func(context.Context, ...string) ([]byte, error)

// runSystemctl keeps the process handling shared by the two read-only callers.
// Each caller supplies its own context deadline and fixed query arguments.
func runSystemctl(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/systemctl", args...)
	// Inherited D-Bus addresses must not redirect a local state query.
	environ := os.Environ()
	cmd.Env = make([]string, 0, len(environ))
	for _, entry := range environ {
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
		return nil, err
	}
	if stderr.Len() != 0 {
		return nil, errors.New("systemctl wrote to stderr")
	}
	return stdout.Bytes(), nil
}
