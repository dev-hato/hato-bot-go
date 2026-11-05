#!/usr/bin/env bash
set -eo pipefail

dockerfile="$(gh api /repos/dependabot/dependabot-core/contents/uv/Dockerfile --jq '.content' | base64 -d)"
uv_versions="$(sed -nE 's?^FROM ghcr\.io/astral-sh/uv:([0-9]+\.[0-9]+\.[0-9]+).*?\1?p' <<<"$dockerfile")"
uv_version="${uv_versions%%$'\n'*}"

if [ -z "$uv_version" ]; then
	echo 'Error: failed to parse uv version from dependabot/dependabot-core/uv/Dockerfile' >&2
	exit 1
fi

echo "Detected uv version in Dependabot: $uv_version" >&2
echo "uv_version=$uv_version" >>"$GITHUB_OUTPUT"
