#!/usr/bin/env bash
set -e

action="$(yq '.jobs.pr-super-lint.steps[-1].uses | line_comment' .github/workflows/pr-test.yml)"
go_version=$(docker run --rm --entrypoint '' "ghcr.io/super-linter/super-linter:slim-${action}" /bin/sh -c 'go version' | sed -e 's/^go version go\([0-9.]*\) .*$/\1/g')
sed -i -e "s/^go \([0-9.]*\)/go $go_version/g" go.mod
image_tag=$(grep golang Dockerfile | sed -e 's/FROM \(golang:[^@]*\).*$/\1/g')
new_image_tag=${image_tag//[0-9.]*-/$go_version-}

if [ "$image_tag" = "$new_image_tag" ]
then
  exit
fi

digest=$(docker buildx imagetools inspect "$new_image_tag" --format '{{json .Manifest.Digest}}' | tr -d '"')
sed -i -e "s/FROM $image_tag@sha256:[^ ]*/FROM $new_image_tag@$digest/g" Dockerfile
