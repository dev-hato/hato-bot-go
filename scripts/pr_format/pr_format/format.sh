#!/usr/bin/env bash

go install tool github.com/rinchsan/gosimports/cmd/gosimports
go mod tidy
gosimports -w .
