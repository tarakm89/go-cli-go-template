# Changelog

All notable changes to this template are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versioning applies to **the template**, not to the projects generated from it.

- **Major** — a generated project's layout or contract changes in a way that
  makes the previous shape wrong. Regenerating is not a drop-in.
- **Minor** — new capability in generated projects, or a new prompt with a
  sensible default.
- **Patch** — fixes, dependency bumps, documentation.

To see what changed between two versions:

```sh
git diff v0.1.0..v0.2.0 -- '{{cookiecutter.project_slug}}'
```

## [Unreleased]

Nothing yet.

## [0.1.0] - 2026-08-27

The first release: the repository becomes a cookiecutter template rather than
an empty Go project.

### Added — scaffolding

- Cookiecutter template rooted at `{{cookiecutter.project_slug}}/`, with
  prompts for the project name, slug, binary name, description, GitHub owner,
  module path, author, Go version, licence and git initialisation.
- `pre_gen_project` hook that validates every answer before anything is
  written — a malformed module path or a Go version below 1.24 stops
  generation with an explanation.
- `post_gen_project` hook that stamps the licence year, runs `go mod tidy` so
  the module matches the chosen Go version, generates `docs/cli` and
  `docs/api` so the first commit is complete, and runs `git init`.
- `make generate` and `make test` at the template root. `make test` renders the
  template and runs the generated project's own `make check`.
- `template-ci.yml`, which renders the template on Linux, macOS and Windows,
  asserts no unsubstituted Jinja markers remain, then runs the generated
  project's build, tests, lint and pre-commit hooks.

### Added — architecture

- Hexagonal layout: `internal/core/{domain,port,service}` inside,
  `internal/adapter/{inbound,outbound}` outside, `internal/bootstrap` as the
  composition root.
- The dependency rule is enforced by `depguard`, not left to discipline: the
  build fails if anything under `internal/core` imports an adapter, Cobra,
  `net/http` or the OpenTelemetry SDK, each with an explanation attached.
- A worked example — a `check` command that probes external HTTP endpoints —
  so that every layer contains something real rather than a `TODO`.

### Added — CLI

- Cobra command tree with `check`, `version` and a hidden `docs` command.
- Exit codes as a contract: `0` succeeded, `1` could not run, `2` ran and the
  answer was bad news.
- `--fail-on never|degraded|down` so the same binary can gate a deploy or feed
  a dashboard.
- `--output text|json`, with empty collections serialised as `[]` rather than
  `null` so `jq` does not fall over.
- Version stamping via `-ldflags`, falling back to the embedded VCS stamps so
  `version` is never a lie.

### Added — observability

- OpenTelemetry traces, metrics and logs, over OTLP via HTTP or gRPC, plus
  `stdout` for debugging and `none` to disable.
- No-op providers when no collector is configured, so a pipeline without one
  still works. A telemetry failure warns and degrades rather than failing the
  run.
- Instrumentation applied by decorating the ports, so the core is never edited
  to add a span or a counter.
- `TRACEPARENT` in the environment is adopted as the parent span, so a run
  joins the trace of the pipeline step that invoked it.
- Resource attributes detected from GitHub Actions, GitLab CI and generic
  `CI=true` runners: workflow, run id, job, repository, branch, commit.
- `log/slog` records fan out to the console and the OTLP log pipeline, each
  stamped with `trace_id` and `span_id`.
- Shutdown flushes on a detached context with a timeout, so a wedged collector
  cannot hang a build.

### Added — testing

- Three tiers: table-driven unit tests, a Ginkgo functional suite that runs the
  whole application in-process, and a Ginkgo end-to-end suite against the
  compiled binary.
- `internal/adapter/outbound/fake` ships as ordinary code rather than hidden in
  `_test.go` files, so any test can wire the real command tree and fake only
  what would reach the network. Includes `Prober`, `Clock`, `Reporter` and
  `Logger`.
- The same behaviour table runs through both the functional and end-to-end
  suites, so a fake that drifts from the real adapter fails the build.
- `observability.WithProviders` as a test seam, so specs can assert on the
  spans and metrics a run produced.

### Added — tooling

- `golangci-lint` v2 for both linting and formatting, so there is no separate
  gofumpt or goimports binary to install.
- Pre-commit hooks, all `repo: local`: format, lint, `go mod tidy`, `go build`
  and doc regeneration on commit; unit and functional tests on push.
- Cross-platform by construction: `ginkgo` and `gomarkdoc` are `go.mod` `tool`
  directives, `golangci-lint` is a pinned `./bin` install, and `make
  tools-sync` fails if the two pins drift apart.
- `scripts/bootstrap.sh` and `scripts/bootstrap.ps1` for macOS, Linux and
  Windows.
- A `Makefile` that detects Windows for the executable suffix and file removal.

### Added — documentation

- `docs/cli` generated from the Cobra tree and `docs/api` from the doc comments
  via gomarkdoc, published to `gh-pages` by a workflow that uses a git worktree
  so history is preserved and an unchanged build commits nothing.
- CI fails if `make docs` produces a diff or an uncommitted addition, so a new
  flag cannot ship without its documentation.

### Removed

- The Jekyll workflow that published this repository's root. The root is now
  Jinja templates, and documentation publishing belongs to the generated
  project.

[Unreleased]: https://github.com/tarakm89/go-cli-go-template/compare/v0.1.0...main
[0.1.0]: https://github.com/tarakm89/go-cli-go-template/releases/tag/v0.1.0
