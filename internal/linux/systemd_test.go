package linux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marcospjr07/docktor/internal/check"
)

var failedServiceQuery = []string{
	"--system", "list-units", "--type=service", "--state=failed", "--output=json", "--no-pager",
}

func TestSystemdCheckFailedServices(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		status  check.Status
		message string
	}{
		{"none", "[]", check.StatusPass, "no failed service units"},
		{"empty with whitespace", " \n[]\n", check.StatusPass, "no failed service units"},
		{"one", `[{"unit":"nginx.service","active":"failed","description":"ignored"}]`, check.StatusFail, "1 failed service: nginx.service"},
		{"several sorted", `[{"unit":"redis.service","active":"failed"},{"unit":"nginx.service","active":"failed"},{"unit":"postgresql.service","active":"failed"}]`, check.StatusFail, "3 failed services: nginx.service, postgresql.service, redis.service"},
		{"many with duplicates", `[{"unit":"z.service","active":"failed"},{"unit":"b.service","active":"failed"},{"unit":"a.service","active":"failed"},{"unit":"f.service","active":"failed"},{"unit":"e.service","active":"failed"},{"unit":"d.service","active":"failed"},{"unit":"a.service","active":"failed"}]`, check.StatusFail, "6 failed services: a.service, b.service, d.service (+3 more)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			c := systemdCheck{runSystemctl: func(_ context.Context, args ...string) ([]byte, error) {
				calls++
				if !slices.Equal(args, failedServiceQuery) {
					t.Fatalf("unexpected systemctl arguments: %v", args)
				}
				return []byte(tt.output), nil
			}}
			got := c.Run(context.Background())
			if got.Status != tt.status || got.Message != tt.message {
				t.Fatalf("got %#v; want status %s and message %q", got, tt.status, tt.message)
			}
			if calls != 1 {
				t.Fatalf("systemctl calls = %d, want 1", calls)
			}
		})
	}
}

func TestSystemdCheckMalformedOutputWarns(t *testing.T) {
	outputs := []string{
		"", "null", "{}", "[", `[{"active":"failed"}]`,
		`[{"unit":"nginx.service","active":"active"}]`,
		`[{"unit":"not-a-service.socket","active":"failed"}]`,
		`[{"unit":"bad\n.service","active":"failed"}]`,
	}
	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			c := systemdCheck{runSystemctl: func(context.Context, ...string) ([]byte, error) {
				return []byte(output), nil
			}}
			got := c.Run(context.Background())
			if got.Status != check.StatusWarn || got.Message != "invalid systemd service data" {
				t.Fatalf("got %#v; want invalid data warning", got)
			}
		})
	}
}

func TestSystemdCheckQueryErrorsWarn(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{"systemctl missing", &exec.Error{Name: "systemctl", Err: exec.ErrNotFound}, "systemctl not found"},
		{"binary missing", os.ErrNotExist, "systemctl not found"},
		{"query failed", errors.New("system manager unavailable"), "cannot query systemd services"},
		{"deadline error", context.DeadlineExceeded, "systemd service query timed out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := systemdCheck{runSystemctl: func(context.Context, ...string) ([]byte, error) {
				return nil, tt.err
			}}
			got := c.Run(context.Background())
			if got.Status != check.StatusWarn || got.Message != tt.message {
				t.Fatalf("got %#v; want warning %q", got, tt.message)
			}
		})
	}
}

func TestSystemdCheckTimeout(t *testing.T) {
	c := systemdCheck{
		timeout: 10 * time.Millisecond,
		runSystemctl: func(ctx context.Context, args ...string) ([]byte, error) {
			if !slices.Equal(args, failedServiceQuery) {
				t.Fatalf("unexpected systemctl arguments: %v", args)
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	got := c.Run(context.Background())
	if got.Status != check.StatusWarn || got.Message != "systemd service query timed out" {
		t.Fatalf("got %#v; want timeout warning", got)
	}
}

func TestSystemdCheckCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	c := systemdCheck{runSystemctl: func(ctx context.Context, _ ...string) ([]byte, error) {
		calls++
		cancel()
		<-ctx.Done()
		return []byte("[]"), ctx.Err()
	}}
	got := c.Run(ctx)
	if got.Status != check.StatusWarn || !strings.Contains(got.Message, "scan interrupted") || calls != 1 {
		t.Fatalf("got %#v after %d calls; want interrupted warning", got, calls)
	}

	calls = 0
	got = c.Run(ctx)
	if got.Status != check.StatusWarn || calls != 0 {
		t.Fatalf("got %#v after %d calls; canceled context must skip query", got, calls)
	}
}

func TestSystemdCheckRegistered(t *testing.T) {
	want := []string{"OS", "Uptime", "Memory", "Root disk", "Systemd", "SSH", "Docker"}
	checks := Checks()
	names := make([]string, len(checks))
	for i, diagnostic := range checks {
		names[i] = diagnostic.Name()
	}
	if !slices.Equal(names, want) {
		t.Fatalf("check order = %v, want %v", names, want)
	}
}

func TestDockerSystemdUnitQueryUsesSharedReadOnlyRunner(t *testing.T) {
	var args []string
	run := func(_ context.Context, actual ...string) ([]byte, error) {
		args = actual
		return []byte("active\n"), nil
	}
	state, err := queryDockerUnitState(context.Background(), dockerSystemSystemd, "docker.service", run)
	if err != nil || state != "active" {
		t.Fatalf("Docker unit state = %q, %v; want active", state, err)
	}
	want := []string{"--system", "show", "--property=ActiveState", "--value", "--no-pager", "--", "docker.service"}
	if !slices.Equal(args, want) {
		t.Fatalf("systemctl arguments = %v, want %v", args, want)
	}

	_, err = queryDockerUnitState(context.Background(), dockerUserSystemd, "docker.socket", func(_ context.Context, actual ...string) ([]byte, error) {
		if actual[0] != "--user" {
			t.Fatalf("user systemd scope used %v", actual)
		}
		return []byte("active\ninactive\n"), nil
	})
	if err == nil {
		t.Fatal("Docker unit query accepted an ambiguous state")
	}
}
