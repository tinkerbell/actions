#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

failed=false

if ! git ls-files '*.sh' | xargs shfmt -l -d; then
	failed=true
fi

if ! git ls-files '*.sh' | xargs shellcheck; then
	failed=true
fi

if ! go mod tidy; then
	failed=true
fi

# this actually shows the diff(s) that will cause the error exit which is nice because its helpful
if ! git diff | (! grep .); then
	failed=true
fi

if $failed; then
	exit 1
fi

go vet ./...

go test -v ./...

GIT_REF="remotes/upstream/$GITHUB_HEAD_REF..remotes/origin/$GITHUB_BASE_REF"

# GITHUB_BASE_REF gets populated only for the event pull_request.
# But this job runs for push as well. In that case we want to assert the current commit.
# It means that HEAD..HEAD~1 is enough.
if [[ -z $GITHUB_BASE_REF ]]; then
	GIT_REF="HEAD..HEAD~1"
fi

go run cmd/hub/main.go build --git-ref ${GIT_REF}
