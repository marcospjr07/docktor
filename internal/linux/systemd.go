package linux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/marcospjr07/docktor/internal/check"
)

const (
	systemdQueryTimeout = 3 * time.Second
	maxFailedUnitNames  = 3
)

type systemdCheck struct {
	runSystemctl systemctlRunner
	timeout      time.Duration
}

func (systemdCheck) Name() string { return "Systemd" }

func (c systemdCheck) Run(ctx context.Context) check.Result {
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}

	timeout := c.timeout
	if timeout <= 0 {
		timeout = systemdQueryTimeout
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := c.runSystemctl(queryCtx, "--system", "list-units", "--type=service", "--state=failed", "--output=json", "--no-pager")
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}
	if errors.Is(queryCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return systemdWarning("systemd service query timed out")
	}
	if errors.Is(err, context.Canceled) {
		return systemdWarning("systemd service query interrupted")
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return systemdWarning("systemctl not found")
	}
	if err != nil {
		return systemdWarning("cannot query systemd services")
	}

	units, err := parseFailedServiceUnits(output)
	if err != nil {
		return systemdWarning("invalid systemd service data")
	}
	if len(units) == 0 {
		return check.Result{Status: check.StatusPass, Message: "no failed service units"}
	}

	visible := units
	if len(visible) > maxFailedUnitNames {
		visible = visible[:maxFailedUnitNames]
	}
	label := "failed service"
	if len(units) != 1 {
		label += "s"
	}
	message := fmt.Sprintf("%d %s: %s", len(units), label, strings.Join(visible, ", "))
	if remaining := len(units) - len(visible); remaining > 0 {
		message += fmt.Sprintf(" (+%d more)", remaining)
	}
	return check.Result{Status: check.StatusFail, Message: message}
}

func parseFailedServiceUnits(output []byte) ([]string, error) {
	output = bytes.TrimSpace(output)
	// Only an explicit JSON array can establish that no units failed. Empty,
	// null, or truncated output must not turn into a false PASS.
	if len(output) == 0 || output[0] != '[' {
		return nil, errors.New("expected systemd unit array")
	}
	var entries []struct {
		Unit   string `json:"unit"`
		Active string `json:"active"`
	}
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(entries))
	units := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Unit
		if entry.Active != "failed" || len(name) <= len(".service") || !strings.HasSuffix(name, ".service") ||
			strings.IndexFunc(name, func(r rune) bool { return r < '!' || r > '~' }) >= 0 {
			return nil, errors.New("invalid failed service unit")
		}
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			units = append(units, name)
		}
	}
	sort.Strings(units)
	return units, nil
}

func systemdWarning(message string) check.Result {
	return check.Result{Status: check.StatusWarn, Message: message}
}
