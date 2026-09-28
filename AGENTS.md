# Architecture and contribution guardrails

- `cmd/docktor` owns CLI argument handling and exit codes. In v0, only CLI usage errors exit nonzero (2); health findings and report output errors exit 0.
- `internal/check` defines `Status`, `Result`, `Check`, the ordered runner, and summary counts. Keep this package free of Linux and terminal details.
- `internal/linux` contains read-only Linux checks. `internal/reporter` formats the resulting report. Add a new check through `linux.Checks()` without coupling it to the reporter.
- Checks may read host data and query read-only system APIs. They must not write files, change configuration, restart services, or invoke commands with side effects.
- Pass `context.Context` through the runner. Treat unavailable or malformed system data as a warning and continue other checks. Inject small file-reading or stat functions in tests instead of relying on the host.
- Keep the initial foundation standard-library-only. Add dependencies only as an explicit project decision.
- Run `make check` with Go 1.27 before merging. Keep tests focused on parsing, calculations, reporting, and CLI behavior.
