#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

failed=0

if ! git ls-files '*.yml' '*.json' '*.md' | xargs prettier --check; then
	failed=1
fi

if ! git ls-files '*.sh' | xargs shfmt -l -d; then
	failed=1
fi

if ! git ls-files '*.sh' | xargs shellcheck; then
	failed=1
fi

if [[ "$failed" -eq 1 ]]; then
	exit "$failed"
fi

go mod tidy
test -z "$(git status --porcelain)"

go vet ./...

go test -v ./...

test -z "$(git status --porcelain)"
