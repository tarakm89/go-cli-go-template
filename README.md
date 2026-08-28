# go-cli-go-template

A [Cookiecutter](https://cookiecutter.readthedocs.io/) template for Go command
line tools built on a hexagonal core, wired for OpenTelemetry, and meant to run
inside CI pipelines.

```sh
cookiecutter gh:tarakm89/go-cli-go-template
```

## What you get

| | |
| --- | --- |
| **CLI** | [Cobra](https://github.com/spf13/cobra) command tree with meaningful exit codes, `--output text\|json`, and a `--fail-on` gate for pipelines |
| **Architecture** | Hexagonal — domain, ports, use cases, adapters — with the dependency rule **enforced by `depguard`**, not just documented |
| **Observability** | OpenTelemetry traces, metrics and logs out of the box; OTLP over HTTP or gRPC, `stdout` for debugging, and silent no-ops when no collector is configured |
| **CI awareness** | Adopts `TRACEPARENT` so a run joins the pipeline's trace; detects GitHub Actions, GitLab CI and generic runners for resource attributes |
| **Testing** | Three tiers: table-driven unit tests, Ginkgo/Gomega **functional** specs that run the whole app in-process against fake adapters, and **e2e** specs against the compiled binary |
| **Quality gates** | `golangci-lint` v2 for linting *and* formatting, local-only pre-commit hooks, `go vet`, race detector |
| **Docs** | `gomarkdoc` for the packages and Cobra for the commands, published to `gh-pages` automatically |
| **Cross-platform** | Windows, macOS and Linux for both development and CI |

## Usage

```sh
pipx install cookiecutter        # or: uv tool install cookiecutter
cookiecutter gh:tarakm89/go-cli-go-template
```

You will be asked for:

| Prompt | Default | Notes |
| --- | --- | --- |
| `project_name` | `My CLI` | Human readable name |
| `project_slug` | derived | Directory and repository name |
| `binary_name` | derived | The executable, and the OTel service name |
| `project_description` | … | One line; appears in `--help` and the docs |
| `github_owner` | `your-org` | User or organisation |
| `module_path` | derived | The Go module path |
| `author_name` / `author_email` | … | Used in the licence |
| `go_version` | `1.25.0` | Must be 1.24 or newer for the `tool` directive |
| `license` | `MIT` | Or `None` |
| `init_git_repo` | `yes` | Runs `git init` and stages the tree |

Then:

```sh
cd my-cli
./scripts/bootstrap.sh     # or .\scripts\bootstrap.ps1 on Windows
make check
```

`make check` runs formatting, linting, `go vet`, race-enabled unit tests, the
functional suite and the end-to-end suite. It should be green on a freshly
generated project before you write a line of your own code.

## What the generated project looks like

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

The generated project ships a working example — a `check` command that probes
external HTTP endpoints — so that every layer has something real in it. Delete
the example and keep the shape.

### The idea worth keeping

Observability and testing both hang off the same property: the core talks to
the outside world only through interfaces.

- **Telemetry is a decorator.** `internal/adapter/outbound/telemetry` wraps a
  port implementation and emits spans and metrics around it. The core is never
  edited to add instrumentation.
- **Fakes are adapters.** `internal/adapter/outbound/fake` ships as ordinary
  code, not hidden in `_test.go` files, so the functional suite can wire the
  real command tree and the real use cases while faking only what would reach
  the network.

```go
app.Prober.With("https://api.example.com", fake.Response{StatusCode: 503})

Expect(app.Run("check", "https://api.example.com")).To(Equal(cli.ExitUnhealthy))
Expect(app.Stdout()).To(ContainSubstring("summary: down"))
Expect(app.SpanNames()).To(ContainElement("probe api.example.com"))
```

## Working on the template

```sh
make generate   # render the template into ./.out
make test       # render it, then run the full check suite inside it
make clean
```

`make test` is what CI runs: a template that generates a project which does not
pass its own `make check` is broken, and that is the only assertion worth
making here.

## Layout of this repository

```
cookiecutter.json                  the prompts
hooks/pre_gen_project.py           validates the answers
hooks/post_gen_project.py          licence year, git init
{{cookiecutter.project_slug}}/     the template itself
.github/workflows/template-ci.yml  generates a project and runs its checks
```

Files under `.github/workflows/` in the template are listed in
`_copy_without_render`, because GitHub Actions' `${{ ... }}` and Jinja's
`{{ ... }}` would otherwise collide. Anything project-specific those workflows
need comes from the `Makefile`, which *is* rendered.

## Versions

[`CHANGELOG.md`](CHANGELOG.md) records every release, and the
[documentation site](https://tarakm89.github.io/go-cli-go-template/versions.html)
renders it with links to the diffs.

Versioning applies to the template, not to projects generated from it. To see
what changed in the scaffold between two releases:

```sh
git diff v0.1.0..v0.2.0 -- '{{cookiecutter.project_slug}}'
```

## Licence

MIT.
