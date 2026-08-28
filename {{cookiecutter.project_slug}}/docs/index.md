---
title: {{ cookiecutter.project_name }}
---

# {{ cookiecutter.project_name }}

{{ cookiecutter.project_description }}

This site is generated from the source tree on every push to `main` and
published to the `gh-pages` branch, so it always describes the code that
shipped.

## Reference

- **[Command line reference](cli/{{ cookiecutter.binary_name }}.md)** — every
  command and flag, generated from the Cobra tree.
- **[Package reference](api/)** — generated from the doc comments with
  [gomarkdoc](https://github.com/princjef/gomarkdoc).

## How the code is arranged

{{ cookiecutter.project_name }} is built as a hexagon: a core that holds the
rules, surrounded by adapters that connect it to the outside world.

| Layer | Package | Depends on |
| --- | --- | --- |
| Domain | `internal/core/domain` | nothing |
| Ports | `internal/core/port` | domain |
| Use cases | `internal/core/service` | domain, ports |
| Driving adapter | `internal/adapter/inbound/cli` | ports |
| Driven adapters | `internal/adapter/outbound/...` | ports |
| Composition root | `internal/bootstrap` | everything |

The dependency arrows only ever point inwards, and `golangci-lint` enforces it:
`depguard` fails the build if anything under `internal/core` imports an adapter,
Cobra, `net/http` or the OpenTelemetry SDK.

## Observability

Telemetry is on by default whenever an OTLP endpoint is configured, and silent
otherwise, so the tool is safe to drop into a pipeline that has no collector.

| Variable | Effect |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Where to send traces, metrics and logs. Unset means no export. |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` (default) or `grpc`. |
| `OTEL_EXPORTER_OTLP_HEADERS` | `key=value,key=value`, for a collector behind auth. |
| `OTEL_SERVICE_NAME` | Overrides the service name on every signal. |
| `OTEL_SDK_DISABLED` | `true` turns telemetry off entirely. |
| `TRACEPARENT` | Adopted as the parent span, so the run joins the pipeline's trace. |
| `LOG_LEVEL`, `LOG_FORMAT` | Console logging: `debug`…`error`, and `text` or `json`. |
