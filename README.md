# Docktor

Docktor is a read-only health diagnostics CLI for Linux servers. The initial `docktor scan` command reports a few useful host indicators in a format that works for a person at a terminal and remains safe to run in automation.

## Example

```text
$ docktor scan
✓ PASS OS: Example Linux 1.0
✓ PASS Uptime: 2d 3h 14m
! WARN Memory: 6.5 / 8.0 GiB used (81.2%)
✓ PASS Root disk: 42.0 / 100.0 GiB used (42.0%)

Summary: 3 pass, 1 warn, 0 fail
```

The example is illustrative; results depend on the host. Memory and disk usage warn at 80% and fail at 95%. A missing or invalid data source produces a warning so the other checks can still run. Exit status is 0 for a completed scan regardless of pass, warn, or fail findings; 1 for an operational or output error; and 2 for a usage error.

## Read-only philosophy

Diagnostics must never change system configuration. The current checks only read `/etc/os-release` (or `/usr/lib/os-release`), `/proc/uptime`, `/proc/meminfo`, and root filesystem statistics. Docktor needs no elevated privileges for these checks. Future checks must preserve this rule, including when they inspect Docker, systemd, SSH, or firewall state.

## Current scope

- Linux OS identification from `os-release`.
- Uptime from procfs.
- Memory utilization using `MemTotal` and `MemAvailable` from procfs.
- Root filesystem utilization from `statfs`, with reserved blocks accounted for in the percentage available to a regular user.
- Plain terminal output with pass, warn, and fail counts.

The initial version uses only the Go standard library. It does not run shell commands or make configuration changes.

## Build and develop

Requires Linux and Go 1.27.

```sh
go run ./cmd/docktor scan
go build -o bin/docktor ./cmd/docktor
make fmt
make vet
make test
make build
make check
```

`docktor` with no arguments and `docktor --help` show help. `docktor scan --help` shows command help. `make check` verifies formatting, vets, tests, and builds the CLI.

## Roadmap

Add independent, read-only checks for Docker, systemd, SSH, firewall, package updates, and networking. Add JSON output for scripts while keeping check results independent of presentation.
