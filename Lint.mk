
.PHONY: lint
lint: _lint

LINT_ARCH := $(shell uname -m)
LINT_OS := $(shell uname)
LINT_OS_LOWER := $(shell echo $(LINT_OS) | tr '[:upper:]' '[:lower:]')
LINT_ROOT := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# shellcheck and hadolint lack arm64 native binaries: rely on x86-64 emulation
ifeq ($(LINT_OS),Darwin)
	ifeq ($(LINT_ARCH),arm64)
		LINT_ARCH=x86_64
	endif
endif

LINTERS :=
FIXERS :=

SHELLCHECK_VERSION ?= v0.11.0
SHELLCHECK_BIN := out/linters/shellcheck-$(SHELLCHECK_VERSION)-$(LINT_ARCH)
$(SHELLCHECK_BIN):
	mkdir -p out/linters
	rm -rf out/linters/shellcheck-*
	curl -sSfL https://github.com/koalaman/shellcheck/releases/download/$(SHELLCHECK_VERSION)/shellcheck-$(SHELLCHECK_VERSION).$(LINT_OS_LOWER).$(LINT_ARCH).tar.xz | tar -C out/linters -xJf -
	mv out/linters/shellcheck-$(SHELLCHECK_VERSION)/shellcheck $@
	rm -rf out/linters/shellcheck-$(SHELLCHECK_VERSION)/shellcheck

LINTERS += shellcheck-lint
shellcheck-lint: $(SHELLCHECK_BIN)
	$(SHELLCHECK_BIN) $(shell find . -name "*.sh")

FIXERS += shellcheck-fix
shellcheck-fix: $(SHELLCHECK_BIN)
	$(SHELLCHECK_BIN) $(shell find . -name "*.sh") -f diff | { read -t 1 line || exit 0; { echo "$$line" && cat; } | git apply -p2; }

HADOLINT_VERSION ?= v2.14.0
HADOLINT_BIN := out/linters/hadolint-$(HADOLINT_VERSION)-$(LINT_ARCH)
$(HADOLINT_BIN):
	mkdir -p out/linters
	rm -rf out/linters/hadolint-*
	curl -sfL https://github.com/hadolint/hadolint/releases/download/$(HADOLINT_VERSION)/hadolint-$(LINT_OS)-$(LINT_ARCH) > $@
	chmod u+x $@

LINTERS += hadolint-lint
hadolint-lint: $(HADOLINT_BIN)
	$(HADOLINT_BIN) --no-fail $(shell find . -name "*Dockerfile")

GOLANGCI_LINT_CONFIG := $(LINT_ROOT)/.golangci.yml
GOLANGCI_LINT_VERSION ?= v2.12.2
# golangci-lint release assets use Go's arch naming (amd64/arm64), not uname -m.
GOLANGCI_LINT_ARCH := $(if $(filter x86_64,$(shell uname -m)),amd64,$(if $(filter aarch64,$(shell uname -m)),arm64,$(shell uname -m)))
GOLANGCI_LINT_DIST := golangci-lint-$(GOLANGCI_LINT_VERSION:v%=%)-$(LINT_OS_LOWER)-$(GOLANGCI_LINT_ARCH)
GOLANGCI_LINT_BIN := $(LINT_ROOT)/out/linters/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(GOLANGCI_LINT_ARCH)
$(GOLANGCI_LINT_BIN):
	mkdir -p out/linters
	rm -rf out/linters/golangci-lint-*
	curl -sSfL https://github.com/golangci/golangci-lint/releases/download/$(GOLANGCI_LINT_VERSION)/$(GOLANGCI_LINT_DIST).tar.gz | tar -C out/linters -xzf -
	mv out/linters/$(GOLANGCI_LINT_DIST)/golangci-lint $@
	rm -rf out/linters/$(GOLANGCI_LINT_DIST)

LINTERS += golangci-lint-lint
golangci-lint-lint: $(GOLANGCI_LINT_BIN)
	@find . -name go.mod -execdir sh -c '"$(GOLANGCI_LINT_BIN)" run -c "$(GOLANGCI_LINT_CONFIG)" | sed "/\.go:[0-9]\+:/ s|^|$$(pwd)/|"' \;

FIXERS += golangci-lint-fix
golangci-lint-fix: $(GOLANGCI_LINT_BIN)
	find . -name go.mod -execdir "$(GOLANGCI_LINT_BIN)" run -c "$(GOLANGCI_LINT_CONFIG)" --fix \;

YAMLLINT_VERSION ?= 1.38.0
YAMLLINT_ROOT := out/linters
YAMLLINT_DIR := $(YAMLLINT_ROOT)/yamllint-$(YAMLLINT_VERSION)
YAMLLINT_BIN := $(YAMLLINT_DIR)/bin/yamllint
$(YAMLLINT_BIN):
	mkdir -p out/linters
	rm -rf out/linters/yamllint*
	pip3 install --target "$(YAMLLINT_DIR)" yamllint==$(YAMLLINT_VERSION) --no-cache-dir --no-warn-script-location

LINTERS += yamllint-lint
yamllint-lint: $(YAMLLINT_BIN)
	PYTHONPATH="$(YAMLLINT_DIR)" python3 -m yamllint .

.PHONY: _lint $(LINTERS)
_lint: $(LINTERS)

.PHONY: fix $(FIXERS)
fix: $(FIXERS)
