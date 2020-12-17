#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

rm -rf ./manifest
go run cmd/gen/main.go generate

# ArtifactHub accept a logo but it has to live in the repository itself.
# This is a little hack to get the Tinkerbell logo deployed side by side with the artifacts
wget -O manifest/logo.png "https://avatars3.githubusercontent.com/u/62397138?s=200&v=4"
