#!/usr/bin/env bash
set -e

action="$(yq '.jobs.pr-super-lint.steps[-1].uses | line_comment' .github/workflows/pr-test.yml)"
go_version=$(docker run --rm --entrypoint '' "ghcr.io/super-linter/super-linter:slim-${action}" /bin/sh -c 'go version' | sed -e 's/go version go\([0-9.]*\) .*$/\1/g')
sed -i -e "s/^go \([0-9.]*\)/go $go_version/g" go.mod
