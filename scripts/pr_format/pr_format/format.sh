#!/usr/bin/env bash
set -e

uv_version="${UV_VERSION:?UV_VERSION is not set}"
python_version="${PYTHON_VERSION:?PYTHON_VERSION is not set}"

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

sed -i -e "s/required-version = .*/required-version = \"$uv_version\"/g" pyproject.toml

# Dependabotが対応しているPythonのうち、最新のバージョンに揃える
sed -i -e "s/requires-python = .*/requires-python = \"==$python_version\"/g" pyproject.toml

uv lock
