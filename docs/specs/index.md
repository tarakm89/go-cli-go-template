---
layout: "doc"
nav_id: "docs"
title: "Specs"
eyebrow: "Spec"
doc_path: "docs/specs/README.md"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/specs/README.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/specs/README.md"
doc_updated: "2026-08-27"
doc_commit: "03b166d"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/03b166df3b4acf244c5c193e648964b04d76df59"
doc_subject: "Add the spec-driven documentation set"
doc_index: false
---

{% raw %}
One document per capability, written before it is built.

| Spec | Status | What it covers |
| --- | --- | --- |
| [0001](0001-hexagonal-core.html) | Implemented | The hexagonal core and its enforced dependency rule |
| [0002](0002-observability-in-ci.html) | Implemented | OpenTelemetry for tools that run inside pipelines |
| [0003](0003-spec-driven-docs.html) | Implemented | Publishing these documents, and showing what changed |

Start a new one by copying [`TEMPLATE.md`](TEMPLATE.html) to
`NNNN-short-name.md`, taking the next free number.

## Status values

| Status | Means |
| --- | --- |
| `Draft` | Being written or argued about. Not safe to build from. |
| `Accepted` | Agreed. Build it. |
| `Implemented` | Shipped, and the acceptance criteria are covered by tests. |
| `Superseded by NNNN` | A later spec replaced this decision. Kept for the record. |
| `Rejected` | Considered and declined. Kept so the question is not reopened blind. |

Nothing is deleted and numbers are never reused: a rejected spec is a record of
a decision, and that is worth as much as an accepted one.
{% endraw %}
