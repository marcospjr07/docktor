// Package reporter formats diagnostic reports for terminals and automation.
package reporter

import (
	"fmt"
	"io"

	"github.com/marcospjr07/docktor/internal/check"
)

// WriteTerminal writes findings and a summary without terminal control codes.
func WriteTerminal(w io.Writer, report check.Report) error {
	for _, result := range report.Results {
		symbol, label := statusDisplay(result.Status)
		if _, err := fmt.Fprintf(w, "%s %-4s %s: %s\n", symbol, label, result.Name, result.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nSummary: %d pass, %d warn, %d fail\n",
		report.Summary.Passed, report.Summary.Warned, report.Summary.Failed)
	return err
}

func statusDisplay(status check.Status) (string, string) {
	switch status {
	case check.StatusPass:
		return "✓", "PASS"
	case check.StatusWarn:
		return "!", "WARN"
	case check.StatusFail:
		return "✗", "FAIL"
	default:
		return "?", "UNKNOWN"
	}
}
