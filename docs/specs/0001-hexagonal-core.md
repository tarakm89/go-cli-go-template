---
layout: "doc"
nav_id: "docs"
title: "0001 — A hexagonal core with an enforced dependency rule"
eyebrow: "Spec"
doc_path: "docs/specs/0001-hexagonal-core.md"
doc_status: "Implemented"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/specs/0001-hexagonal-core.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/specs/0001-hexagonal-core.md"
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

Our command line tools call a lot of external systems. Left alone, transport
concerns — HTTP status handling, retries, pagination — migrate into the code
that expresses the actual decision, until nobody can state the rule without
reading a client library. The same drift makes the logic untestable without a
network, so it stops being tested.

## Goals

- The rules of a tool can be read, and tested, without any transport in sight.
- Adding a second external system does not require touching the rules.
- The boundary survives deadline pressure without relying on discipline.

## Non-goals

- Supporting transports other than a CLI on day one. The shape should make a
  second driving adapter cheap, not deliver one.
- A dependency-injection framework. The composition root is a function.

## Proposal

Ports and adapters, with `internal/core` as the hexagon:

- `internal/core/domain` — entities and rules. Imports nothing beyond the
  standard library.
- `internal/core/port` — every interface across the boundary, expressed in
  domain types.
- `internal/core/service` — the use cases, depending only on the two above.
- `internal/adapter/inbound/*` — driving adapters (Cobra).
- `internal/adapter/outbound/*` — driven adapters (HTTP, presentation, logging,
  fakes).
- `internal/bootstrap` and `cli.setup` — the only places that name concrete
  types.

The rule is enforced by `depguard` in `.golangci.yml`, which fails the build if
anything under `internal/core` imports an adapter, Cobra, `net/http` or the
OpenTelemetry SDK. Test files are exempt, so a unit test may use the fakes.

## Acceptance criteria

- [x] Given an import of `net/http` in `internal/core/service`, when `make lint`
      runs, then it fails naming the rule and the reason.
- [x] Given an import of an adapter package in `internal/core`, when `make lint`
      runs, then it fails.
- [x] Given a unit test in `internal/core` importing
      `internal/adapter/outbound/fake`, when `make lint` runs, then it passes.
- [x] The use case can be exercised with no network, by supplying a fake
      through its port.

## Alternatives considered

**A flat `cmd/` plus `pkg/` layout.** Cheaper on day one. Rejected because it
offers no place for the rules to live that the transport cannot reach, which is
the entire problem.

**Convention without enforcement** — the layout, documented, and code review to
hold it. Rejected on evidence: this is exactly what erodes first when a release
is due.

## Risks and costs

More indirection than a small tool needs at the start, and a linter that will
occasionally be wrong. The cost is paid on day one; the benefit arrives with
the second external system.
{% endraw %}
