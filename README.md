# gh_pages

Source of the documentation site at
<https://tarakm89.github.io/go-cli-go-template/>.

The template itself lives on `main`. These pages are hand-written; when the
template changes in a way that affects them, update them in the same pull
request. The one exception is the version list, which is generated — see below.

## Files

| Path | What it is |
| --- | --- |
| `index.md` | Landing page |
| `usage.md` | Generating a project, the prompts, day-to-day commands |
| `configured.md` | Every moving part in a generated project, and why |
| `mentality.md` | How we expect code to be written |
| `versions.md` | Intro text plus a `<!-- CHANGELOG -->` marker |
| `_layouts/default.html` | The whole page shell |
| `assets/css/style.css` | Theme tokens, typography, syntax colours |
| `assets/js/site.js` | Theme toggle, TOC, copy buttons, diagrams |
| `scripts/inject-changelog.py` | Splices `main`'s `CHANGELOG.md` into `versions.md` |

There is no remote theme. `_layouts/default.html` plus `assets/` is the theme,
so dark mode is ours to control rather than something to override.

## The version list is generated

`versions.md` holds only the introduction and a `<!-- CHANGELOG -->` marker. At
build time the workflow checks out `CHANGELOG.md` from `main` and
`scripts/inject-changelog.py` splices it in, linking each release heading to
its tag and pointing *Unreleased* at the diff since the last one.

So a release is written down in exactly one place: `CHANGELOG.md` on `main`.

Because that file is on another branch, a push to `main` does **not** rebuild
this site. After tagging a release, refresh the page with either:

```sh
gh workflow run "Deploy documentation site" --ref gh_pages
gh api repos/tarakm89/go-cli-go-template/dispatches \
  -f event_type=changelog-updated
```

## Dark mode

Light values sit on bare `:root`. Dark is defined twice — once under
`prefers-color-scheme: dark` guarded with `:root:not([data-theme="light"])`, and
once under `:root[data-theme="dark"]` — so the toggle wins in both directions
and an unset preference still follows the system. A small inline script in the
`<head>` applies the stored choice before first paint, so there is no flash.

When adding a colour, define it as a token in **all three** blocks. A colour
whose only definition is inside a media query will be missing in the other
theme.

## Diagrams

Fenced `mermaid` blocks are rendered in the browser and follow the page theme:

    ```mermaid
    flowchart LR
      A[Port] --> B[Adapter]
    ```

Mermaid is loaded from jsDelivr, pinned by `mermaid_version` in `_config.yml`.
If it cannot be fetched, the block falls back to showing its source rather than
disappearing. Because Mermaid bakes colours into the SVG, the theme toggle
re-renders every diagram.

Fenced `plantuml` blocks are supported too, but are **off by default**:
PlantUML has no browser renderer, so the diagram source has to be sent to a
server. Set `plantuml_server` in `_config.yml` to switch it on — for example
`https://www.plantuml.com/plantuml`, which is a third party, or your own
instance. Until then those blocks render a note explaining how to enable it.

Prefer Mermaid. It renders locally, themes itself, and sends nothing anywhere.

## Editing gotcha

Jekyll runs Liquid over these pages **before** Markdown, including inside code
fences. Any `{{` or `{%` you want to appear literally — and the template docs
are full of them — must sit inside a `raw` block, or Jekyll will try to
evaluate it. `scripts/inject-changelog.py` wraps the injected changelog for the
same reason.

## Deploying

A push to this branch triggers `.github/workflows/pages.yml`, which injects the
changelog, builds with Jekyll and deploys. The repository's Pages source is
"GitHub Actions", not "deploy from a branch", so this workflow is what
publishes — there is no built-in Jekyll build to fall back on.
