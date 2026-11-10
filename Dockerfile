FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS base-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Go modulesを有効にする
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

# go.modとgo.sumをコピー
COPY go.mod go.sum ./

FROM base-builder AS builder

# 依存関係をダウンロード
RUN go mod download

# ソースコードをコピー
COPY "cmd/health_check" "cmd/health_check"
COPY "cmd/misskey_bot" "cmd/misskey_bot"
COPY "cmd/mixi2_bot" "cmd/mixi2_bot"
COPY lib lib

# アプリケーションをビルド
RUN go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go && \
    go build -o hato-bot-go-mixi2-bot cmd/mixi2_bot/main.go && \
    go build -o health-check cmd/health_check/main.go

# 開発用airを対象アーキテクチャ向けにビルド
FROM base-builder AS air-builder

# airをインストール
# hadolint ignore=DL3062
RUN go build -o /air github.com/air-verse/air

# 開発用イメージ
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS dev

# ビルド済みの対象アーキテクチャ向けairをコピー
COPY --from=air-builder /air /go/bin/air

# 実行用の最小イメージ
FROM scratch

# CA証明書をコピー（HTTPS接続用）
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# タイムゾーンデータをコピー
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# ビルドした実行ファイルをコピー
COPY --from=builder /app/health-check /health-check
COPY --from=builder /app/hato-bot-go-misskey-bot /hato-bot-go-misskey-bot
COPY --from=builder /app/hato-bot-go-mixi2-bot /hato-bot-go-mixi2-bot

# nonrootユーザーで実行（UID 65534）
USER 65534:65534

HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=40s CMD ["./health-check"]
