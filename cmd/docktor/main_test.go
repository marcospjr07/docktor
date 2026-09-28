package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marcospjr07/docktor/internal/check"
)

func TestRunCompletedScanReturnsZeroForEveryFinding(t *testing.T) {
	for _, status := range []check.Status{check.StatusPass, check.StatusWarn, check.StatusFail} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.WithValue(context.Background(), contextKey{}, "scan context")
			var stdout, stderr bytes.Buffer
			called := false
			scanFn := func(got context.Context) check.Report {
				called = true
				if got != ctx {
					t.Error("scan did not receive the CLI context")
				}
				results := []check.Result{{Name: "Example", Status: status, Message: "finding"}}
				return check.Report{Results: results, Summary: check.Summarize(results)}
			}
			if code := run(ctx, []string{"scan"}, &stdout, &stderr, scanFn); code != 0 {
				t.Errorf("run() exit code = %d, want 0", code)
			}
			if !called || !strings.Contains(stdout.String(), "Summary:") || stderr.Len() != 0 {
				t.Errorf("unexpected scan output: called=%v, stdout=%q, stderr=%q", called, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunHelpDoesNotScan(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"scan", "--help"}, {"scan", "-h"}} {
		var stdout, stderr bytes.Buffer
		scanFn := func(context.Context) check.Report {
			t.Fatal("help executed a host scan")
			return check.Report{}
		}
		if code := run(context.Background(), args, &stdout, &stderr, scanFn); code != 0 {
			t.Errorf("run(%v) exit code = %d, want 0", args, code)
		}
		if !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
			t.Errorf("run(%v) output: stdout=%q, stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunInvalidUsageReturnsTwo(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"scan", "extra"}, {"--help", "extra"}} {
		var stdout, stderr bytes.Buffer
		scanFn := func(context.Context) check.Report {
			t.Fatal("invalid usage executed a host scan")
			return check.Report{}
		}
		if code := run(context.Background(), args, &stdout, &stderr, scanFn); code != 2 {
			t.Errorf("run(%v) exit code = %d, want 2", args, code)
		}
		if stderr.Len() == 0 {
			t.Errorf("run(%v) did not explain the usage error", args)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestRunReportWriteErrorReturnsOne(t *testing.T) {
	var stderr bytes.Buffer
	scanFn := func(context.Context) check.Report { return check.Report{} }
	if code := run(context.Background(), []string{"scan"}, failingWriter{}, &stderr, scanFn); code != 1 {
		t.Errorf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cannot write report: output unavailable") {
		t.Errorf("missing report write error: %q", stderr.String())
	}
}

type contextKey struct{}
