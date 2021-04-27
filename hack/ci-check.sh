#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

failed=0
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
test -z "$(git status --porcelain go.mod go.sum)"

go vet ./...

go test -v ./...

# We check the list of actions to rebuild only for the entire pull_request.
# The push event does not have a GITHUB_BASE_REF set. That's why here we use
# that environment variable to figure out if it is a PR event or a commit push
if [[ -z $GITHUB_BASE_REF ]]; then
	echo "Skipping: This should only run on pull_request."
	exit 0
fi
go run cmd/hub/main.go build --dry-run --git-ref "remotes/upstream/$GITHUB_BASE_REF..$GITHUB_HEAD_REF"
