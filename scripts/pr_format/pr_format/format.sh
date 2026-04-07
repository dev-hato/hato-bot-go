#!/usr/bin/env bash
set -e

go install tool github.com/daixiang0/gci
go install tool go.uber.org/mock/mockgen
go mod tidy
go generate ./...
gci write -s default -s standard -s "prefix($(go list -m))" .
