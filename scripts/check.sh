#!/bin/sh
set -eu
export GOWORK=off
if [ -n "$(gofmt -l .)" ]; then
  echo 'Run gofmt before publishing.' >&2
  exit 1
fi
go mod download
go test ./...
go vet ./...
go build ./...
# Build the standalone consumer outside the checkout. No local replace or workspace.
publication=$(mktemp -d)
trap 'rm -rf "$publication"' EXIT HUP INT TERM
cp -R examples/minimal/. "$publication/"
go run ./cmd/gazeta build "$publication"
go run ./cmd/gazeta check "$publication"
