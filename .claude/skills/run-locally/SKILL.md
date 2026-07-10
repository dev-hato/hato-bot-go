---
name: run-locally
description: 依存関係のセットアップ、CLI/Misskeyボット/mixi2ボットのビルドと起動、Docker Composeでの実行方法（開発モードでのボット切り替えを含む）
---

## セットアップ

```bash
# Go依存関係のインストール
go mod download

# Node.js依存関係のインストール（リンティング用）
npm install

# pre-commitフックのインストール（gitleaksによるシークレットスキャン）
uv tool run pre-commit install
```

## ボットの実行

```bash
# 環境設定
cp .env.example .env
# .envファイルを編集してAPIトークンを設定

# CLI版のビルドと実行
go build -o hato-bot-go cmd/cli/main.go
./hato-bot-go amesh 東京

# Misskeyボットのビルドと実行
go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go
./hato-bot-go-misskey-bot

# mixi2ボットのビルドと実行
go build -o hato-bot-go-mixi2-bot cmd/mixi2_bot/main.go
./hato-bot-go-mixi2-bot

# Docker Composeで実行（推奨）
export TAG_NAME=$(git symbolic-ref --short HEAD | sed -e "s:/:-:g")

# Misskeyボット
docker compose -f docker-compose.yml -f misskey.docker-compose.yml up -d --wait

# mixi2ボット
docker compose -f docker-compose.yml -f mixi2.docker-compose.yml up -d --wait

# 自動リロード付き開発モード（デフォルトはMisskeyボット）
# mixi2ボットで起動する場合は .air.toml のL.8をコメントアウトしL.9のコメントアウトを解除する
docker compose -f docker-compose.yml -f dev.docker-compose.yml up --build
```