FROM golang:1.25.3-bookworm@sha256:ee420c17fa013f71eca6b35c3547b854c838d4f26056a34eb6171bba5bf8ece4 AS builder

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
COPY lib lib

# アプリケーションをビルド
ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go build -a -installsuffix cgo -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go && \
    go build -a -installsuffix cgo -o health-check cmd/health_check/main.go

# 開発用イメージ
FROM builder AS dev

# airをインストール
# hadolint ignore=DL3062
RUN go install github.com/air-verse/air

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
COPY --from=builder /app/hato-bot-go-misskey-bot /hato-bot-go-misskey-bot
COPY --from=builder /app/health-check /health-check

# nonrootユーザーで実行（UID 65534）
USER 65534:65534

# ポートを公開（必要に応じて）
EXPOSE 8080

# 実行
CMD ["./hato-bot-go-misskey-bot"]
HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=40s CMD ./health-check
