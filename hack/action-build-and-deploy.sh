#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

sudo go run cmd/hub/main.go build --push --git-ref "HEAD..HEAD~1"
