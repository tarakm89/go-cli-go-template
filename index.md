---
nav_id: overview
title: go-cli-go-template
description: A cookiecutter template for Go command line tools — hexagonal core, OpenTelemetry out of the box, three tiers of tests.
---

<p class="eyebrow">Cookiecutter template</p>

# Go CLI scaffolding that already passes its own checks

A template for Go command line tools. It scaffolds a
[Cobra](https://github.com/spf13/cobra) CLI built as a hexagon, wired for
OpenTelemetry, tested at three levels, and set up to run inside CI pipelines.

```sh
pipx install cookiecutter
cookiecutter gh:tarakm89/go-cli-go-template
cd my-cli
./scripts/bootstrap.sh    # .\scripts\bootstrap.ps1 on Windows
make check
```

`make check` is green on a freshly generated project — formatting, linting,
`go vet`, race-enabled unit tests, a functional suite and an end-to-end suite.
That is the point. You start from something that already holds the line, and
you keep it there.

[Get started](usage.html) · [See what's configured](configured.html) · [Read the philosophy](mentality.html)

## The shape

The core holds the rules and knows nothing about the outside world. Everything
external is reached through a port that an adapter implements. Arrows only ever
point inward.

```mermaid
flowchart LR
  CLI["Cobra CLI<br/><small>driving adapter</small>"]

  subgraph core["internal/core"]
    direction TB
    D["domain<br/><small>entities, rules</small>"]
    P["port<br/><small>the interfaces</small>"]
    S["service<br/><small>use cases</small>"]
    S --> P
    S --> D
    P --> D
  end

  HTTP["httpprobe<br/><small>real HTTP</small>"]
  REP["report<br/><small>text / json</small>"]
  LOG["logging<br/><small>slog</small>"]
  FAKE["fake<br/><small>in-memory, for tests</small>"]

  CLI --> S
  HTTP -.implements.-> P
  REP -.implements.-> P
  LOG -.implements.-> P
  FAKE -.implements.-> P

  classDef adapter fill:transparent,stroke-dasharray:4 3;
  class HTTP,REP,LOG,FAKE adapter;
```

## What you get

| | |
| --- | --- |
| **Architecture** | Hexagonal — domain, ports, use cases, adapters — with the dependency rule **enforced by a linter**, not left to discipline |
| **CLI** | Cobra command tree, meaningful exit codes, `--output text\|json`, a `--fail-on` gate for pipelines |
| **Observability** | OpenTelemetry traces, metrics and logs; OTLP over HTTP or gRPC, `stdout` for debugging, silent no-ops when no collector is configured |
| **CI awareness** | Adopts `TRACEPARENT` so a run joins the pipeline's trace; detects GitHub Actions, GitLab CI and generic runners |
| **Testing** | Table-driven unit tests, Ginkgo **functional** specs that run the whole app in-process against fake adapters, **e2e** specs against the compiled binary |
| **Quality gates** | golangci-lint v2 for lint *and* format, repo-local pre-commit hooks, race detector |
| **Docs** | gomarkdoc for packages, Cobra for commands, published to `gh-pages`; CI fails if they drift |
| **Cross-platform** | Windows, macOS and Linux, for development and for CI |

## The idea worth keeping

Two properties do most of the work, and they are the same property seen from
two angles: **the core talks to the outside world only through interfaces.**

Because of that, **telemetry is a decorator**. You wrap a port implementation
and emit spans around it; the core is never edited to add instrumentation.

```go
prober = telemetry.NewProber(prober, tracer, instruments, logger)
```

Also because of that, **tests are fast**. The functional suite wires the real
command tree and the real use cases, and fakes only what would reach the
network:

```go
app.Prober.With("https://api.example.com", fake.Response{StatusCode: 503})

Expect(app.Run("check", "https://api.example.com")).To(Equal(cli.ExitUnhealthy))
Expect(app.Stdout()).To(ContainSubstring("summary: down"))
Expect(app.SpanNames()).To(ContainElement("probe api.example.com"))
```

Everything else on this site is plumbing in service of that.

## Where to go next

| Page | For |
| --- | --- |
| [Getting started](usage.html) | The prompts, what generation does, the day-to-day commands, and how to strip the worked example out |
| [What's configured](configured.html) | Every moving part and why it is set up that way |
| [How we write code](mentality.html) | The rules of thumb we expect you to follow, and what gets pushed back on in review |
| [Versions](versions.html) | Every release, with links to the diffs |
