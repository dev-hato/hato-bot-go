#!/usr/bin/env bash
set -e

dependabot_python_dockerfile_path=python/Dockerfile

echo "Getting Python version from dependabot/dependabot-core/$dependabot_python_dockerfile_path" >&2
python_version="$(
	gh api "/repos/dependabot/dependabot-core/contents/$dependabot_python_dockerfile_path" --jq '.content' |
		base64 -d |
		sed -nE 's/^ARG PY_3_[0-9]+=([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' |
		sort -V |
		tail -n 1
)"

if [ -z "$python_version" ]; then
	echo "Error: failed to parse Python version from dependabot/dependabot-core/$dependabot_python_dockerfile_path" >&2
	exit 1
fi

echo "Detected Python version in Dependabot: $python_version" >&2

# setup-pythonへ渡すため、ステップの出力にする
echo "python_version=$python_version" >>"$GITHUB_OUTPUT"
