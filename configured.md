---
title: What's configured
description: Every moving part in a generated project, and why it is set up that way.
---

[Home](index.html) · [Using the template](usage.html) · **What's configured** · [How we write code](mentality.html)

---

# What's configured

## Layout

```
cmd/<binary>/           entry point; calls bootstrap and nothing else
internal/
  core/
    domain/             entities and rules — no dependencies
    port/               the interfaces across the boundary
    service/            the use cases
  adapter/
    inbound/cli/        Cobra commands (driving adapter)
    outbound/
      httpprobe/        real HTTP calls (driven adapter)
      report/           text and JSON presentation
      logging/          slog behind the Logger port
      telemetry/        span and metric decorators for the ports
      fake/             in-memory adapters for the tests
  observability/        OpenTelemetry SDK setup, CI detection
  bootstrap/            composition root, signals, exit codes
  buildinfo/            version stamping
test/
  functional/           BDD specs, whole app in-process, fake adapters
  e2e/                  BDD specs against the compiled binary
docs/                   generated; published to gh-pages
scripts/                cross-platform bootstrap
```

Who may import whom:

| Package | May import |
| --- | --- |
| `internal/core/domain` | nothing but the standard library |
| `internal/core/port` | `domain` |
| `internal/core/service` | `domain`, `port` |
| `internal/adapter/**` | `domain`, `port` |
| `internal/bootstrap` | everything |

## The dependency rule is enforced

`depguard` in `.golangci.yml` fails the build if anything under
`internal/core` imports an adapter, Cobra, `net/http`, or the OpenTelemetry
SDK, each with an explanation attached:

```yaml
core:
  files:
    - "**/internal/core/**"
    - "!$test"            # a core unit test may use the fake adapters
  deny:
    - pkg: "<module>/internal/adapter"
      desc: "the core must not import adapters; depend on a port instead"
    - pkg: "net/http"
      desc: "HTTP is a transport detail; keep it in an outbound adapter"
    - pkg: "go.opentelemetry.io"
      desc: "instrument the core from outside, by decorating its ports"
```

Try it. Add `import "net/http"` to `internal/core/service/health.go` and run
`make lint`:

```
internal/core/service/health.go:10:2: import 'net/http' is not allowed from
list 'core': HTTP is a transport detail; keep it in an outbound adapter
```

An architecture diagram in a README decays. A failing build does not.

## Linting and formatting

One tool does both. `golangci-lint` v2 runs the linters *and* owns formatting
through its `formatters` section, so there is no separate gofumpt or goimports
binary to install or keep in step.

| | |
| --- | --- |
| `make lint` | `golangci-lint run` |
| `make fmt` | `golangci-lint fmt` — gofumpt then goimports, with local imports grouped last |
| `make fmt-check` | `golangci-lint fmt --diff`, for CI |

Beyond the standard set, these are on: `bodyclose`, `copyloopvar`, `depguard`,
`errorlint`, `gocritic`, `goconst`, `misspell`, `nilerr`, `noctx`,
`nolintlint`, `revive`, `unconvert`, `usestdlibvars`, `whitespace`.

`noctx` is worth calling out: a tool that fans out to external systems must
carry a context on every outbound call, and this is the linter that says so.

## Testing — three tiers

| Tier | Command | Scope | Speed |
| --- | --- | --- | --- |
| Unit | `make test` | One package. Table-driven Go tests. | milliseconds |
| Functional | `make test-functional` | The whole application, in-process, external systems faked. Ginkgo. | milliseconds |
| End to end | `make e2e` | The compiled binary against a real local HTTP server. Ginkgo. | seconds |

### The functional tier is the interesting one

It calls `bootstrap.Run` — the same entry point `main` uses — with fake
adapters injected, and asserts on the real exit code, the real stdout and the
real telemetry, without opening a socket:

```go
func (h *harness) Run(args ...string) int {
	return bootstrap.Run(context.Background(), bootstrap.Options{
		Args:   args,
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Adapters: cli.Options{
			Prober:    h.Prober,     // fake.Prober — the only thing replaced
			Telemetry: h.telemetry,  // in-memory spans and metrics
		},
	})
}
```

The fakes live in `internal/adapter/outbound/fake` as ordinary shipped code,
not hidden in `_test.go` files, precisely so any test in the module can reach
them. `fake.Prober` is programmable per address, records its calls, and can
simulate latency, delay and failure. There are also `fake.Clock`,
`fake.Reporter` and `fake.Logger`.

### BDD and table-driven, together

Behaviour that varies only by input is a `DescribeTable`, so a new case is one
`Entry` rather than a new copy of a spec:

```go
DescribeTable("grading a target",
	func(tc checkCase) {
		app.Prober.With(address, tc.Response)
		args := append(append([]string{"check"}, tc.Flags...), address)

		Expect(app.Run(args...)).To(Equal(tc.WantExit))
		Expect(app.Stdout()).To(ContainSubstring("summary: " + tc.WantSummary))
	},

	Entry("a fast 200 is up, and the run succeeds", checkCase{
		Response: fake.Response{StatusCode: 200, Latency: 10 * time.Millisecond},
		WantExit: cli.ExitOK, WantSummary: "up",
	}),
	Entry("a 404 is degraded, but does not fail the run by default", checkCase{
		Response: fake.Response{StatusCode: 404, Latency: 10 * time.Millisecond},
		WantExit: cli.ExitOK, WantSummary: "degraded",
	}),
	Entry("a 500 is down and fails the run", checkCase{
		Response: fake.Response{StatusCode: 500, Latency: 10 * time.Millisecond},
		WantExit: cli.ExitUnhealthy, WantSummary: "down",
	}),
)
```

The e2e suite runs **the same table** through the compiled binary and a real
HTTP round trip. If the two ever disagree, the fake adapter has drifted from
the real one — which is the failure mode fakes are usually accused of, caught
automatically.

Pure domain rules are plain Go table tests, because Ginkgo buys you nothing
where there is no scenario to describe:

```go
tests := []struct {
	name  string
	probe domain.Probe
	want  domain.State
}{
	{name: "fast 200 is up", probe: domain.Probe{StatusCode: 200, Latency: 10 * time.Millisecond}, want: domain.StateUp},
	{name: "slow 200 is degraded", probe: domain.Probe{StatusCode: 200, Latency: 900 * time.Millisecond}, want: domain.StateDegraded},
	{name: "server error beats slowness", probe: domain.Probe{StatusCode: 500, Latency: time.Hour}, want: domain.StateDown},
}
```

## Observability

Traces, metrics and logs, wired for tools that run inside pipelines.

**Telemetry is a decorator, never a concern of the core.**
`internal/adapter/outbound/telemetry` wraps a port implementation and emits
spans and metrics around it:

```go
prober = telemetry.NewProber(prober, tracer, instruments, logger)
```

The core cannot tell whether it is talking to a real adapter or an instrumented
one. Adding instrumentation never means editing a use case.

**Nothing configured means nothing exported.** Without an OTLP endpoint the SDK
installs no-op providers. The tool still runs and still logs to stderr. A
pipeline without a collector is a supported configuration, not a failure — and
if the SDK fails to build for any other reason, the CLI prints a warning and
carries on with no-ops rather than failing your build.

**The run joins the pipeline's trace.** A `TRACEPARENT` in the environment is
adopted as the parent span, so the tool appears as a child of the pipeline step
that invoked it instead of as an orphan trace nobody finds.

**Runs are identifiable.** GitHub Actions, GitLab CI and generic `CI=true`
runners contribute resource attributes: workflow, run id, job, repository,
branch, commit.

**Logs carry trace correlation.** `log/slog` records fan out to the console and
to the OTLP log pipeline, each stamped with `trace_id` and `span_id`.

**Shutdown is bounded.** The final flush runs on a detached context with a
timeout, so a wedged collector cannot hang a build.

### Configuration

| Variable | Effect |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Where to send telemetry. Unset means no export. |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` (default) or `grpc`. |
| `OTEL_EXPORTER_OTLP_HEADERS` | `key=value,key=value`, for a collector behind auth. |
| `OTEL_SERVICE_NAME` | Overrides the service name on every signal. |
| `OTEL_SDK_DISABLED` | `true` turns telemetry off entirely. |
| `TRACEPARENT` | Adopted as the parent span. |
| `LOG_LEVEL`, `LOG_FORMAT` | `debug`…`error`; `text` or `json`. |

Every one has a flag equivalent (`--otel-endpoint`, `--otel-protocol`,
`--log-level`, `--log-format`) and the flag wins. To watch telemetry without a
collector at all:

```sh
my-cli check --otel-protocol stdout https://example.com
```

### Signals emitted

| Kind | Name | Attributes |
| --- | --- | --- |
| Span | `check all` | target count, summary state |
| Span | `probe <target>` | target, address, status code, latency; errors recorded |
| Metric | `probe.duration` | histogram, seconds, by target and outcome |
| Metric | `probe.total` | counter, by target and outcome |
| Metric | `check.total` | counter, by target and state |

## Exit codes

A tool in a pipeline is read by its exit status far more often than by its
output, so the codes are part of the contract.

| Code | Meaning |
| --- | --- |
| `0` | The run completed and nothing breached the `--fail-on` threshold. |
| `1` | The run could not complete: bad flags, an unparseable argument. |
| `2` | The run completed and the answer was bad news. |

`--fail-on` takes `never`, `degraded` or `down` (the default), so the same
binary can gate a deploy or merely feed a dashboard.

Note the distinction between 1 and 2. "I could not do the job" and "I did the
job and the news is bad" are different things, and a pipeline should be able to
tell them apart.

## Pre-commit hooks

Every hook is `repo: local`. Nothing is fetched from another repository, so a
commit never depends on someone else's tag moving, and the checks match what CI
runs.

| Stage | Hooks |
| --- | --- |
| `pre-commit` | format, lint `--fix`, `go mod tidy`, `go build`, regenerate CLI docs |
| `pre-push` | unit and functional tests |

Formatting and linting on commit; tests on push. Tests on every commit would
train you to use `--no-verify`, which is worse than not having the hook.

The Go tools are installed by pre-commit into its own isolated environment via
`language: golang`, which is what makes the hooks behave identically on
Windows, macOS and Linux — no `PATH` juggling, no `.exe` suffix.

```sh
make hooks       # install
make hooks-run   # run everything over the whole tree
```

## Cross-platform tooling

The only hard requirement is a Go toolchain.

- `ginkgo` and `gomarkdoc` are `tool` directives in `go.mod`, invoked as
  `go tool ginkgo`. Version-locked to the module, identical on every OS, and
  the Ginkgo CLI can never drift from the Ginkgo library.
- `golangci-lint` is installed into `./bin` at a version pinned in the
  `Makefile`, deliberately kept out of `go.mod` so its very large dependency
  graph stays out of yours. `make tools-sync` fails if that pin and the one in
  `.pre-commit-config.yaml` drift apart.
- The `Makefile` detects Windows for the `.exe` suffix and for file removal.
- `scripts/bootstrap.sh` and `scripts/bootstrap.ps1` cover the rest.

## Workflows

**`ci.yml`** — driven by the `go` command rather than by `make`, so it runs
unchanged on Linux, macOS and Windows runners.

| Job | Does |
| --- | --- |
| `lint` | tool-version sync, `golangci-lint fmt --diff`, `golangci-lint run`, `go vet`, and that `go mod tidy` is a no-op |
| `test` | build, race-enabled unit tests, functional and e2e suites, on all three OSes |
| `docs-current` | regenerates `docs/` and fails if anything changed *or was added* |

`docs-current` uses `git status --porcelain` rather than `git diff`, so a page
that was generated but never committed is caught too.

**`docs.yml`** — regenerates `docs/` and publishes it to `gh-pages` using a git
worktree, so history is preserved and an unchanged build commits nothing.
`docs/cli` comes from the Cobra tree, `docs/api` from the doc comments via
[gomarkdoc](https://github.com/princjef/gomarkdoc). Because both halves are
generated, the published reference cannot drift from the code that shipped.

## The template's own CI

There is only one thing worth asserting about a project template: that the
project it produces passes its own checks. `.github/workflows/template-ci.yml`
renders the template on all three operating systems, verifies no unsubstituted
Jinja markers remain, then hands over to the generated project's own `Makefile`
and `pre-commit`.
