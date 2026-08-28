---
title: Using the template
description: Generating a project, what the prompts mean, and the day-to-day commands.
---

[Home](index.html) · **Using the template** · [What's configured](configured.html) · [How we write code](mentality.html)

---

# Using the template

## Prerequisites

| Tool | Needed for | Install |
| --- | --- | --- |
| Go 1.24+ | everything | [go.dev/dl](https://go.dev/dl/) — 1.24 is the floor because of the `tool` directive in `go.mod` |
| cookiecutter | generating | `pipx install cookiecutter`, `uv tool install cookiecutter`, or `brew install cookiecutter` |
| pre-commit | git hooks | installed for you by `scripts/bootstrap.*` |
| GNU Make | convenience | present on macOS and Linux; on Windows `winget install ezwinports.make`, or use the `go` commands directly |

## Generating a project

```sh
cookiecutter gh:tarakm89/go-cli-go-template
```

You will be asked for:

| Prompt | Default | What it is |
| --- | --- | --- |
| `project_name` | `My CLI` | Human readable name. Appears in the README and the docs site. |
| `project_slug` | derived from the name | The directory and repository name. Lowercase, dashes. |
| `binary_name` | same as the slug | The executable name — and the OpenTelemetry service name, and the `User-Agent` sent to external systems. |
| `project_description` | … | One line. Appears in `--help` and on the generated docs site. |
| `github_owner` | `your-org` | Your user or organisation. |
| `module_path` | `github.com/<owner>/<slug>` | The Go module path. Override it if you host elsewhere. |
| `author_name`, `author_email` | … | Used in the licence. |
| `go_version` | `1.25.0` | Must be 1.24 or newer. Use the full patch version — the toolchain normalises it, and a bare `1.25` would make the first `go mod tidy` dirty the tree. |
| `license` | `MIT` | Or `None`, which deletes the `LICENSE` file. |
| `init_git_repo` | `yes` | Runs `git init` and stages the tree. |

Answers are validated **before** anything is written. A malformed module path
or a Go version below 1.24 stops generation with an explanation rather than
producing a project that will not build.

### What happens after the prompts

The post-generation hook does four things, none of them fatal if they fail:

1. Stamps the current year into `LICENSE`, or deletes it if you chose `None`.
2. Runs `go mod tidy`, so the module is correct for the Go version you picked.
3. Generates `docs/cli` and `docs/api`, so the first commit is complete and the
   documentation-drift check in CI passes straight away.
4. Runs `git init` and stages everything.

If Go is not on your `PATH`, or you are offline, it says so and tells you what
to run later.

## First run

```sh
cd my-cli

./scripts/bootstrap.sh      # macOS, Linux
.\scripts\bootstrap.ps1     # Windows

make check
```

`bootstrap` checks your Go version against `go.mod`, downloads the modules,
installs the pinned `golangci-lint` into `./bin`, then installs `pre-commit`
using whichever of `uv`, `pipx`, `brew`, `scoop` or `pip --user` it can find,
and installs the git hooks. It is safe to re-run.

## Day to day

`make` on its own prints the list. The ones you will actually use:

| Command | Does |
| --- | --- |
| `make check` | Format check, lint, vet, and every test tier. This is what CI runs. |
| `make test` | Unit tests only — fast. |
| `make test-functional` | The BDD functional suite. No network, so it is fast too. |
| `make e2e` | Builds the binary and runs the end-to-end suite against it. |
| `make fmt` | Rewrites files with gofumpt and goimports. |
| `make lint-fix` | golangci-lint with `--fix`. |
| `make build` | `./bin/<binary>`, version-stamped from git. |
| `make run ARGS="check https://example.com"` | Runs the CLI without building. |
| `make docs` | Regenerates `docs/cli` and `docs/api`. |
| `make hooks-run` | Runs every pre-commit hook over the whole tree. |

On Windows without Make, the same work is four commands:

```powershell
.\bin\golangci-lint.exe run
go test -race .\internal\...
go tool ginkgo --randomize-all -p .\test\functional
go tool ginkgo --randomize-all .\test\e2e
```

## Publishing the documentation

The generated project ships a workflow that regenerates `docs/` on every push
to `main` and pushes it to a `gh-pages` branch. Turn it on once:

**Settings → Pages → Source: Deploy from a branch → `gh-pages` / `(root)`**

If your organisation prefers `gh_pages` with an underscore, change
`PAGES_BRANCH` at the top of `.github/workflows/docs.yml`. That is the only
place the name appears.

## Making it yours

The generated project contains a **worked example** — a `check` command that
probes external HTTP endpoints and grades them. It is there so that every layer
has something real in it rather than a `TODO`. Read it once, then replace it.

To strip it out:

1. Delete `internal/core/domain/domain.go` and write your own entities and
   rules. Keep the shape: no imports beyond the standard library.
2. Replace the interfaces in `internal/core/port/port.go`.
3. Replace `internal/core/service/health.go` with your use cases.
4. Replace `internal/adapter/outbound/httpprobe` with the adapters your systems
   need, and `internal/adapter/outbound/fake` with fakes for them.
5. Rewrite `internal/adapter/inbound/cli/check.go` as your command.
6. Update the wiring in `setup()` in `internal/adapter/inbound/cli/cli.go` —
   the one function that names concrete adapters.
7. Delete the tests that went with the example and write yours in the same
   three tiers.

What you should **not** change without a reason: the layer boundaries, the
`depguard` rules that enforce them, or the three-tier test split. Those are the
template.

## Working on the template itself

```sh
git clone git@github.com:tarakm89/go-cli-go-template.git
cd go-cli-go-template

make generate   # render into ./.out
make test       # render, then run the generated project's own `make check`
make clean
```

`make test` is the only assertion worth making about a template: a template
that produces a project which cannot pass its own checks is broken, however
tidy the template looks.

Two traps when editing it:

{% raw %}
- **Go source must never contain a literal `{{`.** Jinja would eat it. Write
  composite literals with a newline — `[]T{`, newline, `{...},` — not `[]T{{`.
  Where a Go or Cobra template genuinely needs doubled braces, wrap them in
  Jinja's `raw` / `endraw` block tags.
- **`.github/workflows/*.yml` inside the template is not rendered.** It is
  listed in `_copy_without_render`, because GitHub Actions' `${{ }}` expressions
  collide with Jinja's `{{ }}`. Anything project-specific those workflows need
  has to reach them through the `Makefile`, which *is* rendered.
{% endraw %}
