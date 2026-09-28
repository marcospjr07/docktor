package reporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/marcospjr07/docktor/internal/check"
)

func TestWriteTerminal(t *testing.T) {
	results := []check.Result{
		{Name: "OS", Status: check.StatusPass, Message: "Example Linux"},
		{Name: "Memory", Status: check.StatusWarn, Message: "80.0% used"},
		{Name: "Root disk", Status: check.StatusFail, Message: "95.0% used"},
	}
	var output bytes.Buffer
	if err := WriteTerminal(&output, check.Report{Results: results, Summary: check.Summarize(results)}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"✓ PASS OS: Example Linux",
		"! WARN Memory: 80.0% used",
		"✗ FAIL Root disk: 95.0% used",
		"Summary: 1 pass, 1 warn, 1 fail",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output missing %q:\n%s", want, output.String())
		}
	}
}
