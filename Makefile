# Development entry points for the template itself.
#
# The only assertion worth making about a project template is that the project
# it generates passes its own checks. `make test` does exactly that.

OUT_DIR     := $(CURDIR)/.out
FIXTURE     := probe-cli
COOKIECUTTER ?= cookiecutter

# Answers used when rendering the template for a test run.
FIXTURE_ARGS := \
	project_name="Probe CLI" \
	github_owner=acme \
	author_name="Template CI" \
	author_email=ci@example.com \
	init_git_repo=no

.DEFAULT_GOAL := help
.PHONY: help generate test lint-template clean

help:
	@echo go-cli-go-template - make targets
	@echo
	@echo   generate   render the template into ./.out
	@echo   test       render the template, then run the generated project's own make check
	@echo   clean      remove ./.out
	@echo
	@echo Requires cookiecutter: pipx install cookiecutter

# ------------------------------------------------------------------- generate
generate: clean
	$(COOKIECUTTER) --no-input -o $(OUT_DIR) $(CURDIR) $(FIXTURE_ARGS)
	@echo
	@echo generated $(OUT_DIR)/$(FIXTURE)

# ----------------------------------------------------------------------- test
# A template that generates a project which cannot pass its own checks is
# broken, however well the template itself is written.
test: generate
	$(MAKE) -C $(OUT_DIR)/$(FIXTURE) tools-sync
	$(MAKE) -C $(OUT_DIR)/$(FIXTURE) check
	$(MAKE) -C $(OUT_DIR)/$(FIXTURE) docs
	$(MAKE) -C $(OUT_DIR)/$(FIXTURE) build
	@echo
	@echo the generated project passes its own checks

# ---------------------------------------------------------------------- clean
clean:
	rm -rf $(OUT_DIR)
