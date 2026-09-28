# Docktor

Docktor is a read-only health diagnostics CLI for Linux servers. The initial `docktor scan` command reports a few useful host indicators in a format that works for a person at a terminal and remains safe to run in automation.

## Example

```text
$ docktor scan
✓ PASS OS: Example Linux 1.0
✓ PASS Uptime: 2d 3h 14m
! WARN Memory: 6.5 / 8.0 GiB used (81.2%)
✓ PASS Root disk: 42.0 / 100.0 GiB used (42.0%)
✓ PASS Docker: local daemon reachable (version 29.7.2)

Summary: 4 pass, 1 warn, 0 fail
```

The example is illustrative; results depend on the host. Memory and disk usage warn at 80% and fail at 95%. A missing or invalid data source produces a warning so the other checks can still run. Completed scans return 0 regardless of health findings; operational or report output errors return 1, and CLI usage errors return 2.

## Read-only philosophy

Diagnostics must never change system configuration. The current checks read `/etc/os-release` (or `/usr/lib/os-release`), `/proc/uptime`, `/proc/meminfo`, and root filesystem statistics. The Docker check sends only `GET /version` to the local Docker Engine API over a Unix socket. Before connecting to the known systemd Docker sockets, it reads `docker.socket` and `docker.service` state and skips the probe if the socket is active but the service is not. Docktor does not explicitly start or restart services or send mutating Docker API requests. It needs no elevated privileges, though socket permissions may limit this check. Future checks must preserve this rule, including when they inspect systemd, SSH, or firewall state.

## Current scope

- Linux OS identification from `os-release`.
- Uptime from procfs.
- Memory utilization using `MemTotal` and `MemAvailable` from procfs.
- Root filesystem utilization from `statfs`, with reserved blocks accounted for in the percentage available to a regular user.
- Local Docker daemon reachability and Docker CLI availability.
- Plain terminal output with pass, warn, and fail counts.

The initial version uses only the Go standard library. The Docker check accepts an explicit local `unix://` endpoint from `DOCKER_HOST`; otherwise, it uses an existing socket at `$XDG_RUNTIME_DIR/docker.sock` or falls back to `/var/run/docker.sock`. It ignores remote `DOCKER_HOST` values and `DOCKER_CONTEXT`. It checks for the Docker CLI using a path lookup but never executes it; a missing CLI does not change the daemon result. For the standard system sockets and the selected rootless socket, an unavailable systemd state produces a warning without probing. No check makes configuration changes.

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

Add independent, read-only checks for systemd, SSH, firewall, package updates, and networking. Add JSON output for scripts while keeping check results independent of presentation.
