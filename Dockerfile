FROM --platform=$BUILDPLATFORM golang:1.26.4-bookworm@sha256:b305420a68d0f229d91eb3b3ed9e519fcf2cf5461da4bef997bf927e8c0bfd2b AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Go modulesを有効にする
ENV GO111MODULE=on

# go.modとgo.sumをコピー
COPY go.mod go.sum ./

# 依存関係をダウンロード
RUN go mod download

# ソースコードをコピー
COPY "cmd/health_check" "cmd/health_check"
COPY "cmd/misskey_bot" "cmd/misskey_bot"
COPY "cmd/mixi2_bot" "cmd/mixi2_bot"
COPY lib lib

# アプリケーションをビルド
ENV CGO_ENABLED=0
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o hato-bot-go-mixi2-bot cmd/mixi2_bot/main.go && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o health-check cmd/health_check/main.go

# 開発用airを対象アーキテクチャ向けにビルド
FROM --platform=$BUILDPLATFORM golang:1.26.4-bookworm@sha256:b305420a68d0f229d91eb3b3ed9e519fcf2cf5461da4bef997bf927e8c0bfd2b AS air-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

ENV GO111MODULE=on
COPY go.mod go.sum ./

# airをインストール
# hadolint ignore=DL3062
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /air github.com/air-verse/air

# 開発用イメージ
FROM golang:1.26.4-bookworm@sha256:b305420a68d0f229d91eb3b3ed9e519fcf2cf5461da4bef997bf927e8c0bfd2b AS dev

WORKDIR /app

# ビルド済みの対象アーキテクチャ向けairをコピー
COPY --from=air-builder /air /go/bin/air

# air設定ファイルをコピー
COPY .air.toml .

# ポートを公開（必要に応じて）
EXPOSE 8080

# airで実行
CMD ["air", "-c", ".air.toml"]

# 実行用の最小イメージ
FROM scratch AS prod

# CA証明書をコピー（HTTPS接続用）
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# タイムゾーンデータをコピー
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# ビルドした実行ファイルをコピー
COPY --from=builder /app/health-check /health-check

# nonrootユーザーで実行（UID 65534）
USER 65534:65534

# ポートを公開（必要に応じて）
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=40s CMD ./health-check

FROM prod AS prod_misskey

# ビルドした実行ファイルをコピー
COPY --from=builder /app/hato-bot-go-misskey-bot /hato-bot-go-misskey-bot

# 実行
CMD ["./hato-bot-go-misskey-bot"]

FROM prod AS prod_mixi2

# ビルドした実行ファイルをコピー
COPY --from=builder /app/hato-bot-go-mixi2-bot /hato-bot-go-mixi2-bot

# 実行
CMD ["./hato-bot-go-mixi2-bot"]
