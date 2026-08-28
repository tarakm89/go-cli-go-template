# Specs

One document per capability, written before it is built.

| Spec | Status | What it covers |
| --- | --- | --- |
| [0001](0001-hexagonal-core.md) | Implemented | The hexagonal core and its enforced dependency rule |
| [0002](0002-observability-in-ci.md) | Implemented | OpenTelemetry for tools that run inside pipelines |
| [0003](0003-spec-driven-docs.md) | Implemented | Publishing these documents, and showing what changed |

Start a new one by copying [`TEMPLATE.md`](TEMPLATE.md) to
`NNNN-short-name.md`, taking the next free number.

The table above is maintained by hand and is the one thing here that can go
stale. The [published documentation
index](https://tarakm89.github.io/go-cli-go-template/docs/index.html) is
generated from the files themselves, along with when each last changed and
which release carried it — trust that one if the two disagree.

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
