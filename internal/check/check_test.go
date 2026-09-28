package check

import (
	"context"
	"testing"
)

type fakeCheck struct {
	name   string
	result Result
	seen   context.Context
}

func (c *fakeCheck) Name() string { return c.name }

func (c *fakeCheck) Run(ctx context.Context) Result {
	c.seen = ctx
	return c.result
}

func TestSummarize(t *testing.T) {
	results := []Result{
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
		{Status: StatusPass},
	}
	want := Summary{Passed: 2, Warned: 1, Failed: 1}
	if got := Summarize(results); got != want {
		t.Fatalf("Summarize() = %+v, want %+v", got, want)
	}
	if got := want.Total(); got != len(results) {
		t.Fatalf("Total() = %d, want %d", got, len(results))
	}
}

func TestRunnerPreservesOrderNamesAndContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := &fakeCheck{name: "first", result: Result{Status: StatusPass, Message: "ok"}}
	second := &fakeCheck{name: "second", result: Result{Name: "wrong", Status: StatusWarn, Message: "limited"}}
	runner := NewRunner(first, second)
	report := runner.Run(ctx)

	if len(report.Results) != 2 || report.Results[0].Name != "first" || report.Results[1].Name != "second" {
		t.Fatalf("unexpected result order or names: %+v", report.Results)
	}
	if first.seen != ctx || second.seen != ctx {
		t.Fatal("runner did not pass the scan context to each check")
	}
	if report.Summary != (Summary{Passed: 1, Warned: 1}) {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestRunnerConvertsInvalidStatusToWarning(t *testing.T) {
	diagnostic := &fakeCheck{name: "invalid", result: Result{Status: "unknown"}}
	report := NewRunner(diagnostic).Run(context.Background())
	if report.Results[0].Status != StatusWarn || report.Summary != (Summary{Warned: 1}) {
		t.Fatalf("unexpected invalid-status handling: %+v", report)
	}
}

type panicCheck struct{}

func (panicCheck) Name() string { return "panicking" }

func (panicCheck) Run(context.Context) Result { panic("check failed") }

func TestRunnerContinuesAfterCheckPanic(t *testing.T) {
	next := &fakeCheck{name: "next", result: Result{Status: StatusPass}}
	report := NewRunner(panicCheck{}, next).Run(context.Background())
	if len(report.Results) != 2 || report.Results[0].Name != "panicking" || report.Results[0].Status != StatusWarn || report.Results[1].Status != StatusPass {
		t.Fatalf("unexpected results after panic: %+v", report.Results)
	}
	if report.Summary != (Summary{Passed: 1, Warned: 1}) {
		t.Fatalf("unexpected summary after panic: %+v", report.Summary)
	}
}
