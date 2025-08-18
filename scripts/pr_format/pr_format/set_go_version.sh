#!/usr/bin/env bash

action="$(yq '.jobs.pr-super-lint.steps[-1].uses | line_comment' .github/workflows/pr-test.yml)"
sed -i -e "s/^go .*/go $(docker run --rm --entrypoint '' "ghcr.io/super-linter/super-linter:slim-${action}" /bin/sh -c "go version | awk '{print \$3}' | sed -e 's/^go//g'")/g" go.mod
