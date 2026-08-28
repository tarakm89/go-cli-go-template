# Development entry points for {{ cookiecutter.project_name }}.
#
# Everything here runs on Windows, macOS and Linux. The only hard requirement
# is a Go toolchain: every Go tool is either a `tool` directive in go.mod
# (invoked with `go tool`) or a pinned binary installed into ./bin, so there is
# nothing to install by hand and nothing to keep in step with a global PATH.
#
# Run `make` on its own for the list.

BINARY  := {{ cookiecutter.binary_name }}
MODULE  := {{ cookiecutter.module_path }}
CMD_PKG := ./cmd/{{ cookiecutter.binary_name }}

# Pinned dev tools. Keep GOLANGCI_LINT_VERSION in step with the version pinned
# in .pre-commit-config.yaml; the `tools-sync` target checks that for you.
GOLANGCI_LINT_VERSION := v2.10.1

# ---------------------------------------------------------------- portability
ifeq ($(OS),Windows_NT)
EXE        := .exe
VERSION    ?= dev
COMMIT     ?=
BUILD_DATE ?=
RM_DIR     := rmdir /s /q
RM_FILE    := del /q
else
EXE        :=
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
RM_DIR     := rm -rf
RM_FILE    := rm -f
endif

BIN_DIR       := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint$(EXE)
BINARY_PATH   := $(BIN_DIR)/$(BINARY)$(EXE)

BUILDINFO := $(MODULE)/internal/buildinfo
LDFLAGS   := -s -w \
	-X $(BUILDINFO).version=$(VERSION) \
	-X $(BUILDINFO).commit=$(COMMIT) \
	-X $(BUILDINFO).date=$(BUILD_DATE)

GINKGO_FLAGS := --randomize-all --fail-on-pending --keep-going --timeout=10m

.DEFAULT_GOAL := help
.PHONY: help bootstrap tools tools-sync hooks hooks-run hooks-update \
        fmt fmt-check lint lint-fix vet tidy \
        test test-race cover test-functional e2e test-all check ci \
        build install run docs docs-cli docs-api clean

# ----------------------------------------------------------------------- help
help:
	@echo {{ cookiecutter.project_name }} - make targets
	@echo
	@echo   bootstrap        install dev tools and git hooks, download modules
	@echo   tools            install pinned dev tools into ./bin
	@echo   hooks            install the pre-commit hooks into .git/hooks
	@echo   hooks-run        run every pre-commit hook over the whole tree
	@echo
	@echo   fmt              format the code
	@echo   fmt-check        fail if anything is unformatted
	@echo   lint             run golangci-lint
	@echo   lint-fix         run golangci-lint with --fix
	@echo   vet              run go vet
	@echo   tidy             run go mod tidy
	@echo
	@echo   test             unit tests
	@echo   test-race        unit tests under the race detector
	@echo   cover            unit tests with a coverage profile
	@echo   test-functional  BDD functional suite, fake adapters, no network
	@echo   e2e              BDD end-to-end suite against the built binary
	@echo   test-all         every test tier
	@echo   check            fmt-check, lint, and every test tier
	@echo
	@echo   build            build ./bin/$(BINARY)$(EXE)
	@echo   install          go install the binary
	@echo   run              run the CLI: make run ARGS="check https://example.com"
	@echo   docs             regenerate docs/ from the code
	@echo   clean            remove build and test output

# ------------------------------------------------------------------ bootstrap
bootstrap: tools hooks
	go mod download
	@echo
	@echo Ready. Try: make check

tools: $(GOLANGCI_LINT)

$(GOLANGCI_LINT):
	@echo installing golangci-lint $(GOLANGCI_LINT_VERSION) into $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Fails if the Makefile and .pre-commit-config.yaml have drifted apart, so the
# linter you get on commit is the linter CI runs.
tools-sync:
	@grep -q "golangci-lint@$(GOLANGCI_LINT_VERSION)" .pre-commit-config.yaml \
		|| (echo "ERROR: .pre-commit-config.yaml does not pin golangci-lint $(GOLANGCI_LINT_VERSION)"; exit 1)
	@echo tool versions are in sync

# ---------------------------------------------------------------------- hooks
# pre-commit is a Python tool; scripts/bootstrap.* install it on any platform.
hooks:
	pre-commit install --install-hooks || \
		(echo; echo pre-commit not found - run scripts/bootstrap.sh or scripts/bootstrap.ps1 first; exit 1)

hooks-run:
	pre-commit run --all-files

hooks-update:
	pre-commit autoupdate

# ------------------------------------------------------- formatting and lints
fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

fmt-check: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix

vet:
	go vet ./...

tidy:
	go mod tidy

# ---------------------------------------------------------------------- tests
# Unit tests: the domain rules and each adapter in isolation, table driven.
test:
	go test ./internal/...

test-race:
	go test -race ./internal/...

cover:
	go test -coverprofile=coverage.out -covermode=atomic ./internal/...
	go tool cover -func=coverage.out

# Functional: the whole application in-process, external systems faked.
test-functional:
	go tool ginkgo $(GINKGO_FLAGS) -p ./test/functional

# End to end: the compiled binary against a real HTTP server.
e2e: build
	go tool ginkgo $(GINKGO_FLAGS) ./test/e2e

test-all: test-race test-functional e2e

check: fmt-check lint vet test-race test-functional e2e

ci: tools-sync check docs build

# ---------------------------------------------------------------------- build
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY_PATH) $(CMD_PKG)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD_PKG)

run:
	go run $(CMD_PKG) $(ARGS)

# ----------------------------------------------------------------------- docs
# docs/ is what the gh-pages workflow publishes. Both halves are generated, so
# the published reference cannot drift from the code.
docs: docs-cli docs-api

# One page per command, straight from the Cobra tree.
docs-cli:
	go run $(CMD_PKG) docs --output-dir docs/cli

# One page per package, straight from the doc comments. The test directories
# hold only external _test packages, which gomarkdoc has nothing to document.
docs-api:
	go tool gomarkdoc --output 'docs/api/{% raw %}{{.Dir}}{% endraw %}.md' ./cmd/... ./internal/...

# ---------------------------------------------------------------------- clean
clean:
	go clean -testcache
	-$(RM_DIR) $(BIN_DIR)
	-$(RM_DIR) docs/api
	-$(RM_DIR) docs/cli
	-$(RM_FILE) coverage.out
