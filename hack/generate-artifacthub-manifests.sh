#!/usr/bin/env nix-shell
#!nix-shell -i bash ../shell.nix
# shellcheck shell=bash

set -eux

rm -rf ./manifest
go run cmd/gen/main.go generate
