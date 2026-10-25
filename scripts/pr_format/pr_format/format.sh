#!/usr/bin/env bash
set -e

go install tool github.com/daixiang0/gci
go install tool go.uber.org/mock/mockgen
go mod tidy
go generate ./...
gci write -s default -s standard -s "prefix($(go list -m))" .

bash "${GITHUB_WORKSPACE}/scripts/install_npm.sh"

# 書き込み権限のあるジョブで動くため、依存パッケージのスクリプトは実行しない
npm install --ignore-scripts

# .rulesync/ のルールから、各AIエージェント向けのファイルを生成する
npx --no-install rulesync generate

dependabot_dockerfile_path=uv/Dockerfile

echo "Getting uv version from dependabot/dependabot-core/$dependabot_dockerfile_path" >&2
uv_version="$(
	gh api "/repos/dependabot/dependabot-core/contents/$dependabot_dockerfile_path" --jq '.content' |
		base64 -d |
		sed -nE 's?^FROM ghcr\.io/astral-sh/uv:([0-9]+\.[0-9]+\.[0-9]+).*?\1?p' |
		head -n 1
)"

if [ -z "$uv_version" ]; then
	echo "Error: failed to parse uv version from dependabot/dependabot-core/$dependabot_dockerfile_path" >&2
	exit 1
fi

echo "Detected uv version in Dependabot: $uv_version" >&2

sed -i -e "s/required-version = .*/required-version = \"$uv_version\"/g" pyproject.toml
