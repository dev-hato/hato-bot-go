#!/usr/bin/env bash
set -e

go install tool github.com/daixiang0/gci
go install tool go.uber.org/mock/mockgen
go mod tidy
go generate ./...
gci write -s default -s standard -s "prefix($(go list -m))" .

bash "${GITHUB_WORKSPACE}/scripts/install_npm.sh"
npm install

# .rulesync/ のルールから、各AIエージェント向けのファイルを生成する
npx --no-install rulesync generate
