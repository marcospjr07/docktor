package linux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/marcospjr07/docktor/internal/check"
)

const (
	defaultDockerSocket  = "/var/run/docker.sock"
	dockerProbeTimeout   = 5 * time.Second
	maxDockerVersionBody = 1 << 20
	maxDockerVersionText = 128
)

type dialDockerSocketFunc func(context.Context, string) (net.Conn, error)

type dockerCheck struct {
	socketPath string
	scope      dockerSystemdScope
	unitState  dockerUnitStateFunc
	dialSocket dialDockerSocketFunc
	lookPath   func(string) (string, error)
}

func (dockerCheck) Name() string { return "Docker" }

func (c dockerCheck) Run(ctx context.Context) check.Result {
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}

	result, skipProbe := c.socketActivationWarning(ctx)
	if interrupted, done := contextWarning(ctx); done {
		return interrupted
	}
	if !skipProbe {
		probeCtx, cancel := context.WithTimeout(ctx, dockerProbeTimeout)
		result = c.probe(probeCtx)
		timedOut := errors.Is(probeCtx.Err(), context.DeadlineExceeded)
		cancel()
		if interrupted, done := contextWarning(ctx); done {
			return interrupted
		}
		if timedOut {
			result = dockerWarning("local daemon check timed out")
		}
	}

	if _, err := c.lookPath("docker"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			result.Message += "; Docker CLI not found"
		} else {
			result.Message += "; Docker CLI unavailable"
		}
	}
	return result
}

func (c dockerCheck) probe(ctx context.Context) check.Result {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return c.dialSocket(ctx, c.socketPath)
		},
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/version", nil)
	if err != nil {
		return dockerWarning("local Docker request unavailable")
	}
	response, err := client.Do(request)
	if err != nil {
		return dockerConnectionWarning(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return dockerWarning(fmt.Sprintf("local Docker API returned HTTP %d", response.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxDockerVersionBody+1))
	if err != nil {
		if dockerTimeout(err) {
			return dockerWarning("local daemon check timed out")
		}
		return dockerWarning("invalid local Docker HTTP response")
	}
	if len(data) > maxDockerVersionBody {
		return dockerWarning("local Docker version response too large")
	}
	var payload struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return dockerWarning("invalid local Docker version JSON")
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		return dockerWarning("local Docker daemon version missing")
	}
	if len(version) > maxDockerVersionText || strings.IndexFunc(version, func(r rune) bool {
		return r < '!' || r > '~'
	}) >= 0 {
		return dockerWarning("invalid local Docker daemon version")
	}
	return check.Result{Status: check.StatusPass, Message: "local daemon reachable (version " + version + ")"}
}

func dockerConnectionWarning(err error) check.Result {
	switch {
	case dockerTimeout(err):
		return dockerWarning("local daemon check timed out")
	case errors.Is(err, os.ErrNotExist):
		return dockerWarning("local Docker socket not found")
	case errors.Is(err, os.ErrPermission), errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return dockerWarning("local Docker socket access denied")
	case errors.Is(err, syscall.ECONNREFUSED):
		return dockerWarning("local Docker daemon unavailable (connection refused)")
	case errors.Is(err, syscall.ENOTSOCK):
		return dockerWarning("local Docker socket invalid")
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return dockerWarning("local Docker daemon unavailable")
	}
	return dockerWarning("invalid local Docker HTTP response")
}

func dockerTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func dockerWarning(message string) check.Result {
	return check.Result{Status: check.StatusWarn, Message: message}
}

func dialLocalDockerSocket(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}

func resolveDockerSocket(dockerHost, runtimeDir string) string {
	if strings.HasPrefix(dockerHost, "unix://") {
		endpoint, err := url.Parse(dockerHost)
		if err == nil && endpoint.Scheme == "unix" && endpoint.Host == "" && endpoint.Opaque == "" &&
			endpoint.User == nil && endpoint.RawQuery == "" && !endpoint.ForceQuery && endpoint.Fragment == "" &&
			filepath.IsAbs(endpoint.Path) {
			return filepath.Clean(endpoint.Path)
		}
	}
	if filepath.IsAbs(runtimeDir) {
		path := filepath.Join(runtimeDir, "docker.sock")
		info, err := os.Stat(path)
		if (err == nil && info.Mode()&os.ModeSocket != 0) || errors.Is(err, os.ErrPermission) {
			return path
		}
	}
	return defaultDockerSocket
}
