#!/usr/bin/env bash
set -e

action="$(yq '.jobs.pr-super-lint.steps[-1].uses | line_comment' .github/workflows/pr-test.yml)"
go_version=$(docker run --rm --entrypoint '' "ghcr.io/super-linter/super-linter:slim-${action}" /bin/sh -c 'go version' | sed -e 's/^go version go\([0-9.]*\) .*$/\1/g')
sed -i -e "s/^go \([0-9.]*\)/go $go_version/g" go.mod
image_tag=$(sed -nE 's/^FROM( --platform=[^[:space:]]+)? (golang:[^@[:space:]]+)@sha256:[^[:space:]]+.*/\2/p' Dockerfile | sort -u)
if [ -z "$image_tag" ]; then
	echo "Could not find a pinned golang base image in Dockerfile" >&2
	exit 1
fi
if [[ "$image_tag" == *$'\n'* ]]; then
	echo "Found multiple golang base images in Dockerfile: $image_tag" >&2
	exit 1
fi
new_image_tag=$(sed -E "s/^golang:[0-9.]+-/golang:${go_version}-/" <<<"$image_tag")

if [ "$image_tag" = "$new_image_tag" ]; then
	exit
fi

digest=$(docker buildx imagetools inspect "$new_image_tag" --format '{{json .Manifest.Digest}}' | tr -d '"')
sed -i -E "s#^(FROM( --platform=[^[:space:]]+)? )golang:[0-9.]+-[^@[:space:]]+@sha256:[^[:space:]]+#\1${new_image_tag}@${digest}#" Dockerfile
