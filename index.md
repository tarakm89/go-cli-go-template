---
title: go-cli-go-template
description: A cookiecutter template for Go command line tools — hexagonal core, OpenTelemetry out of the box, three tiers of tests.
---

[Using the template](usage.html) · [What's configured](configured.html) · [How we write code](mentality.html)

---

A [cookiecutter](https://cookiecutter.readthedocs.io/) template for Go command
line tools. It scaffolds a [Cobra](https://github.com/spf13/cobra) CLI built as
a hexagon, wired for OpenTelemetry, tested at three levels, and set up to run
inside CI pipelines.

```sh
pipx install cookiecutter
cookiecutter gh:tarakm89/go-cli-go-template
cd my-cli
./scripts/bootstrap.sh    # .\scripts\bootstrap.ps1 on Windows
make check
```

`make check` is green on a freshly generated project. That is the point: you
start from something that already passes formatting, linting, `go vet`,
race-enabled unit tests, a functional suite and an end-to-end suite, and you
keep it that way.

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

## The three pages

**[Using the template](usage.html)** — what the prompts mean, what happens when
you generate, the day-to-day commands, and how to strip the worked example out.

**[What's configured](configured.html)** — every moving part and why it is set
up the way it is: the layout, the linter rules, the three test tiers, the
telemetry pipeline, the hooks, the workflows.

**[How we write code](mentality.html)** — the part that matters most. The rules
of thumb we expect you to follow, what belongs in which layer, and how to add a
new external system without eroding the boundary.

## Why this shape

Two properties do most of the work, and they are the same property viewed from
two angles: **the core talks to the outside world only through interfaces.**

Because of that, telemetry is a decorator — you wrap a port implementation and
emit spans around it, and the core is never edited to add instrumentation.

Also because of that, tests are fast — the functional suite wires the real
command tree and the real use cases, and fakes only what would reach the
network:

```go
app.Prober.With("https://api.example.com", fake.Response{StatusCode: 503})

Expect(app.Run("check", "https://api.example.com")).To(Equal(cli.ExitUnhealthy))
Expect(app.Stdout()).To(ContainSubstring("summary: down"))
Expect(app.SpanNames()).To(ContainElement("probe api.example.com"))
```

That is the whole idea. Everything else is plumbing in service of it.
