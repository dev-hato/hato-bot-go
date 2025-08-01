#!/usr/bin/env bash

go install tool github.com/daixiang0/gci
go mod tidy
gci write -s default -s standard -s "prefix($(go list -m))" .
