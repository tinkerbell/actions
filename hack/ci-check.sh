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

GIT_REF="remotes/upstream/$GITHUB_BASE_REF..$GITHUB_HEAD_REF"

# GITHUB_BASE_REF gets populated only for the event pull_request.
# But this job runs for push as well. In that case we want to assert the current commit.
# It means that HEAD..HEAD~1 is enough.
if [[ -z $GITHUB_BASE_REF ]]; then
	GIT_REF="HEAD..HEAD~1"
fi
sudo go run cmd/hub/main.go build --git-ref ${GIT_REF}
