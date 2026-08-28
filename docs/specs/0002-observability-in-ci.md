---
layout: "doc"
nav_id: "specification"
title: "0002 — OpenTelemetry for tools that run inside pipelines"
eyebrow: "Spec"
doc_path: "docs/specs/0002-observability-in-ci.md"
doc_status: "Implemented"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/specs/0002-observability-in-ci.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/specs/0002-observability-in-ci.md"
doc_updated: "2026-08-27"
doc_commit: "03b166d"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/03b166df3b4acf244c5c193e648964b04d76df59"
doc_subject: "Add the spec-driven documentation set"
doc_index: false
---

{% raw %}
- **Status:** Implemented
- **Date:** 2026-08-27

## Problem

When a pipeline step built on one of our tools is slow or flaky, there is
nothing to look at. The tool exits with a code and some log lines that nobody
kept. We cannot answer which external call was slow, how often it fails, or
whether a given commit made it worse.

Bolting telemetry on later does not work either: by then the calls are spread
across the code, and instrumenting them means editing the logic.

## Goals

- Traces, metrics and logs from every run, without editing the core to get
  them.
- A run appears inside the trace of the pipeline step that invoked it, not as
  an orphan.
- Telemetry can never fail a build.
- A pipeline with no collector still works.

## Non-goals

- Choosing a backend. Anything speaking OTLP is in scope; nothing else is.
- Sampling strategy beyond "keep CLI runs". They are short and rare; sampling
  one away loses the only trace anyone will look at.

## Proposal

**Instrument by decoration.** `internal/adapter/outbound/telemetry` wraps a
port implementation and emits spans and metrics around it:

```go
prober = telemetry.NewProber(prober, tracer, instruments, logger)
```

The core never mentions OpenTelemetry — `depguard` sees to that, per
[0001](0001-hexagonal-core.html).

**Degrade, never fail.** With no OTLP endpoint configured the SDK installs
no-op providers. If it cannot be built for any other reason, the CLI warns on
stderr and continues with no-ops.

**Join the pipeline's trace.** A `TRACEPARENT` in the environment is adopted as
the parent span.

**Identify the run.** Resource attributes are detected from GitHub Actions,
GitLab CI and generic `CI=true` runners: workflow, run id, job, repository,
branch, commit.

**Bound the exit.** The final flush runs on a detached context with a timeout,
so a wedged collector cannot hang a build.

### Signals

| Kind | Name | Attributes |
| --- | --- | --- |
| Span | `check all` | target count, summary state |
| Span | `probe <target>` | target, address, status code, latency; errors recorded |
| Metric | `probe.duration` | histogram, seconds, by target and outcome |
| Metric | `probe.total` | counter, by target and outcome |
| Metric | `check.total` | counter, by target and state |

## Acceptance criteria

- [x] Given no `OTEL_EXPORTER_OTLP_ENDPOINT`, when the tool runs, then it
      succeeds, exports nothing, and still logs to stderr.
- [x] Given `OTEL_SDK_DISABLED=true`, then no exporter is constructed.
- [x] Given a run that probes one target, then the trace contains `check all`
      with `probe <target>` as its child.
- [x] Given a `TRACEPARENT` in the environment, then it is adopted as the
      parent span.
- [x] Given a failing probe, then the span records the error and
      `probe.total` counts it as `unreachable`.
- [x] Telemetry assertions live in the functional suite, so they run on every
      commit.

## Alternatives considered

**Spans written inline in the use cases.** Simplest to write, and the reason
observability code ends up unreadable. Rejected: it also puts the SDK inside
the core, which 0001 forbids.

**Logs only, scraped from CI output.** Cheap, and enough to answer "did it
fail". Useless for "which call was slow", which is the question actually being
asked.

## Risks and costs

The OTLP exporters for HTTP and gRPC are a meaningful dependency for a small
binary. We accept that: a tool nobody can diagnose costs more.
{% endraw %}
