package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/marcospjr07/docktor/internal/check"
	"github.com/marcospjr07/docktor/internal/linux"
	"github.com/marcospjr07/docktor/internal/reporter"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, scan))
}

func scan(ctx context.Context) check.Report {
	return check.NewRunner(linux.Checks()...).Run(ctx)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, scanFn func(context.Context) check.Report) int {
	if len(args) == 0 {
		writeHelp(stdout)
		return 0
	}

	switch args[0] {
	case "--help", "-h", "help":
		if len(args) != 1 {
			fmt.Fprintf(stderr, "docktor: unexpected argument %q\n", args[1])
			writeHelp(stderr)
			return 2
		}
		writeHelp(stdout)
		return 0
	case "scan":
		jsonOutput, help := false, false
		for _, arg := range args[1:] {
			switch {
			case arg == "--json" && !jsonOutput:
				jsonOutput = true
			case (arg == "--help" || arg == "-h") && !help:
				help = true
			default:
				fmt.Fprintf(stderr, "docktor scan: unexpected argument %q\n", arg)
				writeScanHelp(stderr)
				return 2
			}
		}
		if help {
			writeScanHelp(stdout)
			return 0
		}
		report := scanFn(ctx)
		writeReport := reporter.WriteTerminal
		if jsonOutput {
			writeReport = reporter.WriteJSON
		}
		if err := writeReport(stdout, report); err != nil {
			fmt.Fprintf(stderr, "docktor: cannot write report: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "docktor: unknown command %q\n", args[0])
		writeHelp(stderr)
		return 2
	}
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, "Docktor reads Linux server health indicators.\n\nUsage:\n  docktor scan [--json]\n  docktor --help\n\nCommands:\n  scan    Run read-only health diagnostics\n")
}

func writeScanHelp(w io.Writer) {
	fmt.Fprint(w, "Usage: docktor scan [--json]\n\nRun read-only Linux health diagnostics and print a summary.\n\nOptions:\n  --json  Write a JSON report instead of terminal output\n  --help  Show command help\n")
}
