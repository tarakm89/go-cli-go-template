# {{ cookiecutter.project_name }}

{{ cookiecutter.project_description }}

[![CI](https://github.com/{{ cookiecutter.github_owner }}/{{ cookiecutter.project_slug }}/actions/workflows/ci.yml/badge.svg)](https://github.com/{{ cookiecutter.github_owner }}/{{ cookiecutter.project_slug }}/actions/workflows/ci.yml)

```console
$ {{ cookiecutter.binary_name }} check https://api.example.com https://db.example.com
TARGET           STATE  LATENCY  DETAIL
api.example.com  up     42ms     status 200
db.example.com   down   0s       dial tcp: connection refused

summary: down
$ echo $?
2
```

## Getting started

```sh
# macOS and Linux
./scripts/bootstrap.sh

# Windows (PowerShell)
.\scripts\bootstrap.ps1
```

That installs the Go dependencies, the pinned dev tools and the git hooks.
Then:

```sh
make check   # format, lint, and every test tier
make build   # ./bin/{{ cookiecutter.binary_name }}
make run ARGS="check https://example.com"
```

`make` on its own lists every target.

## How the code is arranged

This is a hexagon — ports and adapters. The core holds the rules and knows
nothing about the outside world; everything external is reached through a port
that an adapter implements.

```
                    ┌──────────────────────────────┐
  driving           │                              │            driven
  adapter           │   internal/core              │            adapters
                    │                              │
  cli  ──────────▶  │   domain   rules, entities   │  ─────▶  httpprobe (real HTTP)
  (cobra)           │   port     the interfaces    │  ─────▶  report    (text / json)
                    │   service  the use cases     │  ─────▶  logging   (slog)
                    │                              │  ─────▶  fake      (in-memory)
                    └──────────────────────────────┘
```

| Package | Holds | May import |
| --- | --- | --- |
| `internal/core/domain` | Entities, value objects, the grading rules | nothing but the standard library |
| `internal/core/port` | Every interface across the boundary | `domain` |
| `internal/core/service` | The use cases | `domain`, `port` |
| `internal/adapter/inbound/cli` | The Cobra command tree | `port`, the outbound adapters |
| `internal/adapter/outbound/*` | HTTP, presentation, logging, fakes | `domain`, `port` |
| `internal/observability` | The OpenTelemetry SDK setup | — |
| `internal/bootstrap` | The composition root | everything |

**The dependency rule is enforced, not just documented.** `depguard` in
`.golangci.yml` fails the build if anything under `internal/core` imports an
adapter, Cobra, `net/http` or the OpenTelemetry SDK. Try it: add
`import "net/http"` to `internal/core/service/health.go` and run `make lint`.

### Adding a new external system

1. Add the interface to `internal/core/port`, in domain types.
2. Implement the use case in `internal/core/service` against that interface.
3. Write the real adapter under `internal/adapter/outbound/`.
4. Add a fake to `internal/adapter/outbound/fake` for the tests.
5. Wire both in `internal/adapter/inbound/cli.setup` — the only place that
   names concrete adapters.
6. Wrap the adapter in a decorator from `internal/adapter/outbound/telemetry`
   so it is traced and measured like everything else.

## Testing

Three tiers, all runnable from `make`.

| Tier | Command | What it does |
| --- | --- | --- |
| Unit | `make test` | Table-driven Go tests over the domain rules and each adapter in isolation. |
| Functional | `make test-functional` | Ginkgo/Gomega specs that run the **whole application in-process** with fake adapters. No network. |
| End to end | `make e2e` | Ginkgo specs against the **compiled binary** and a real local HTTP server. |

The functional suite is the one worth understanding. It calls `bootstrap.Run`
— the same entry point `main` uses — and swaps in `fake.Prober`, so it asserts
on the real exit code, the real output and the real telemetry without ever
opening a socket:

```go
app.Prober.With("https://api.example.com", fake.Response{StatusCode: 503})

Expect(app.Run("check", "https://api.example.com")).To(Equal(cli.ExitUnhealthy))
Expect(app.Stdout()).To(ContainSubstring("summary: down"))
Expect(app.SpanNames()).To(ContainElement("probe api.example.com"))
```

Behaviour that varies only by input is written as a `DescribeTable`, so a new
case is one `Entry` rather than a new copy of a spec.

## Observability

Traces, metrics and logs are wired out of the box, aimed at running inside a CI
pipeline.

- **Nothing configured, nothing exported.** Without an OTLP endpoint the SDK
  installs no-op providers; the tool still runs and still logs to stderr. A
  pipeline without a collector is a supported configuration, not a failure.
- **The run joins the pipeline's trace.** A `TRACEPARENT` in the environment is
  adopted as the parent span.
- **Resource attributes identify the run.** GitHub Actions, GitLab CI and
  generic `CI=true` environments contribute the workflow, run id, repository,
  branch and commit.
- **Telemetry is a decorator, never a concern of the core.** Spans and metrics
  come from `internal/adapter/outbound/telemetry` wrapping the ports.
- **Shutdown is bounded.** The final flush cannot hang a build.

| Variable | Effect |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Where to send telemetry. Unset means no export. |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` (default) or `grpc`. |
| `OTEL_EXPORTER_OTLP_HEADERS` | `key=value,key=value` for a collector behind auth. |
| `OTEL_SERVICE_NAME` | Overrides the service name on every signal. |
| `OTEL_SDK_DISABLED` | `true` turns telemetry off entirely. |
| `TRACEPARENT` | Adopted as the parent span. |
| `LOG_LEVEL` / `LOG_FORMAT` | `debug`…`error`; `text` or `json`. |

The same settings exist as flags (`--otel-endpoint`, `--otel-protocol`,
`--log-level`, `--log-format`) and the flags win.

To watch the telemetry without a collector:

```sh
{{ cookiecutter.binary_name }} check --otel-protocol stdout https://example.com
```

## Exit codes

A tool that runs in a pipeline is read by its exit status more often than by
its output, so these are part of the contract.

| Code | Meaning |
| --- | --- |
| `0` | The run completed and nothing breached the `--fail-on` threshold. |
| `1` | The run could not complete: bad flags, an unparseable target. |
| `2` | The run completed and the answer was bad news. |

`--fail-on` takes `never`, `degraded` or `down` (the default), so the same
binary can gate a deploy or merely report on a dashboard.

## Pre-commit hooks

Every hook is defined locally in `.pre-commit-config.yaml` — nothing is fetched
from another repository, so a commit never depends on someone else's tag
moving, and the checks match what CI runs.

| Stage | Hooks |
| --- | --- |
| `pre-commit` | format, lint, `go mod tidy`, `go build`, regenerate CLI docs |
| `pre-push` | unit and functional tests |

```sh
make hooks       # install them
make hooks-run   # run every hook over the whole tree
```

The Go tools are installed by pre-commit into an isolated environment, which is
what makes the hooks behave identically on Windows, macOS and Linux.

## Documentation

`docs/` is generated and published to the `gh-pages` branch on every push to
`main`:

- `docs/cli/` — one page per command, from the Cobra tree.
- `docs/api/` — one page per package, from the doc comments, via
  [gomarkdoc](https://github.com/princjef/gomarkdoc).

```sh
make docs
```

CI fails if `make docs` produces a diff, so a new flag cannot ship without its
documentation. To publish, enable **Settings → Pages → Deploy from a branch →
`gh-pages` / root** once.

## Layout

```
cmd/{{ cookiecutter.binary_name }}/     entry point; calls bootstrap and nothing else
internal/
  core/
    domain/        entities and rules — no dependencies
    port/          the interfaces across the boundary
    service/       the use cases
  adapter/
    inbound/cli/   Cobra commands (driving)
    outbound/
      httpprobe/   real HTTP calls
      report/      text and JSON presentation
      logging/     slog behind the Logger port
      telemetry/   span and metric decorators for the ports
      fake/        in-memory adapters for the tests
  observability/   OpenTelemetry SDK setup, CI detection
  bootstrap/       composition root, signals, exit codes
  buildinfo/       version stamping
test/
  functional/      BDD specs, whole app in-process, fake adapters
  e2e/             BDD specs against the compiled binary
docs/              generated; published to gh-pages
scripts/           cross-platform bootstrap
```
