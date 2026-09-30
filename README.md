# 🩺 Docktor

Docktor is a read-only health diagnostics CLI for Linux servers. `docktor scan` reports host indicators as terminal text by default. Use `docktor scan --json` for machine-readable output in automation.

**🇧🇷 Looking for the Portuguese version? See [🩺 Docktor [pt-br]](https://github.com/marcospjr07/docktor-pt-br).**

## Example

```text
$ docktor scan
✓ PASS OS: Example Linux 1.0
✓ PASS Uptime: 2d 3h 14m
! WARN Memory: 6.5 / 8.0 GiB used (81.2%)
✓ PASS Root disk: 42.0 / 100.0 GiB used (42.0%)
✓ PASS Systemd: no failed service units
! WARN SSH: root login enabled; password authentication enabled
! WARN Firewall: no supported firewall tooling found
! WARN Packages: 12 updates available; package metadata freshness unknown
✓ PASS Docker: local daemon reachable (version 29.7.2)

Summary: 5 pass, 4 warn, 0 fail
```

The example is illustrative; results depend on the host. Memory and disk usage warn at 80% and fail at 95%. A missing or invalid data source produces a warning so the other checks can still run. Completed scans return 0 regardless of health findings; operational or report output errors return 1, and CLI usage errors return 2.

## JSON output

Run `docktor scan --json` to write one JSON object to stdout followed by a newline. Errors and usage diagnostics go to stderr. The same checks run in both formats, with the same exit codes. A completed JSON scan returns 0 even when checks report `warn` or `fail`; scripts should inspect the statuses and summary to evaluate health.

The following example uses the same illustrative findings as the terminal example:

```json
{
  "schema_version": 1,
  "checks": [
    {"name": "OS", "status": "pass", "message": "Example Linux 1.0"},
    {"name": "Uptime", "status": "pass", "message": "2d 3h 14m"},
    {"name": "Memory", "status": "warn", "message": "6.5 / 8.0 GiB used (81.2%)"},
    {"name": "Root disk", "status": "pass", "message": "42.0 / 100.0 GiB used (42.0%)"},
    {"name": "Systemd", "status": "pass", "message": "no failed service units"},
    {"name": "SSH", "status": "warn", "message": "root login enabled; password authentication enabled"},
    {"name": "Firewall", "status": "warn", "message": "no supported firewall tooling found"},
    {"name": "Packages", "status": "warn", "message": "12 updates available; package metadata freshness unknown"},
    {"name": "Docker", "status": "pass", "message": "local daemon reachable (version 29.7.2)"}
  ],
  "summary": {"pass": 5, "warn": 4, "fail": 0, "total": 9}
}
```

Schema version 1 always includes `schema_version`, `checks`, and `summary`. Each check has string fields `name`, `status`, and `message`; statuses are `pass`, `warn`, or `fail`. Checks retain scan order, and an empty report uses `checks: []`. The summary always includes integer fields `pass`, `warn`, `fail`, and `total`, including zero counts. Messages describe the observed host state and may change; scripts should use the fields and status values rather than parse message text. A breaking schema change will increment `schema_version`.

## Read-only philosophy

Diagnostics must never change system configuration. The current checks read `/etc/os-release` (or `/usr/lib/os-release`), `/proc/uptime`, `/proc/meminfo`, root filesystem statistics, SSH server configuration files, and failed system service units through a read-only `systemctl list-units` query. The firewall check runs `ufw status` or, when `firewalld.service` is already running, `firewall-cmd --state`. It checks the systemd unit first because D-Bus access could activate a stopped firewalld daemon. It does not change firewall rules or explicitly start services. The Docker check sends only `GET /version` to the local Docker Engine API over a Unix socket. Before connecting to the known systemd Docker sockets, it reads `docker.socket` and `docker.service` state and skips the probe if the socket is active but the service is not. Docktor does not explicitly start or restart services or send mutating Docker API requests. It needs no elevated privileges, though file or socket permissions may limit individual checks. Future checks must preserve this rule.

The package check runs `apt-get -s -o Dir::Cache::pkgcache= -o Dir::Cache::srcpkgcache= dist-upgrade` with `LC_ALL=C`. Simulation disables locks, and the cache options disable persistent cache generation. Docktor never refreshes package indexes, downloads packages, or performs an installation or removal. It limits the captured output and does not execute a shell.

## Current scope

- Linux OS identification from `os-release`.
- Uptime from procfs.
- Memory utilization using `MemTotal` and `MemAvailable` from procfs.
- Root filesystem utilization from `statfs`, with reserved blocks accounted for in the percentage available to a regular user.
- Failed system service units from systemd.
- SSH server `PermitRootLogin` and `PasswordAuthentication` settings from `sshd_config` and its Includes.
- UFW and firewalld active state, when their command-line tooling is available.
- Available APT upgrades based on local package indexes, with a conservative metadata freshness warning.
- Local Docker daemon reachability and Docker CLI availability.
- Default terminal output and optional schema version 1 JSON, with pass, warn, fail, and total counts in JSON.

The initial version uses only the Go standard library. The SSH check reads the default `/etc/ssh/sshd_config` and supported Includes. It reports only `PermitRootLogin` and `PasswordAuthentication`; it does not verify a running daemon's command-line overrides, validate the entire SSH configuration, or assess other authentication methods such as keyboard-interactive. Missing settings, unreadable or unsupported Includes, and policies that may vary by `Match` return a warning. The firewall check supports only UFW and firewalld. It reports whether supported tooling is active, not whether rules protect a specific interface or port; raw nftables and iptables rules are not interpreted. If the systemd state of firewalld cannot be determined, it warns without contacting firewalld. The Docker check accepts an explicit local `unix://` endpoint from `DOCKER_HOST`; otherwise, it uses an existing socket at `$XDG_RUNTIME_DIR/docker.sock` or falls back to `/var/run/docker.sock`. It ignores remote `DOCKER_HOST` values and `DOCKER_CONTEXT`. It checks for the Docker CLI using a path lookup but never executes it; a missing CLI does not change the daemon result. For the standard system sockets and the selected rootless socket, an unavailable systemd state produces a warning without probing. No check makes configuration changes.

The Packages check initially supports only APT. The count comes from the simulation summary's **upgraded** field; new dependencies and removals are not added. Other package managers return a warning. The count reflects local indexes and the host's APT selection policy, not every available upstream version.

Zero upgrades can only justify PASS if metadata freshness is reliably established. This version deliberately returns WARN even for zero upgrades: neither the periodic `/var/lib/apt/periodic/update-stamp` nor an update success hook proves that every repository was refreshed. A regular stamp more than 24 hours old produces an old-record warning; manual updates may have happened without updating this stamp. A recent, absent, unreadable, or incoherent stamp leaves freshness unknown. It never uses the date of a single InRelease as proof. No complete-refresh marker is trusted by this initial backend.

## Build and develop

Requires Linux and Go 1.27.

```sh
go run ./cmd/docktor scan
go run ./cmd/docktor scan --json
go build -o bin/docktor ./cmd/docktor
make fmt
make vet
make test
make build
make check
```

`docktor` with no arguments and `docktor --help` show help. `docktor scan --help` shows command help. `make check` verifies formatting, vets, tests, and builds the CLI.

## Roadmap

Add independent checks for networking. Other package managers, a trustworthy record of complete index refreshes, and broader firewall rule interpretation can follow separate designs.
