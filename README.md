# hato-bot-go

これは、hato-botのameshコマンドをGo言語で実装したもので、気象庁のデータを使用して気象レーダー画像を生成します。
スタンドアロンプログラムとして実行するか、MisskeyボットとしてWebSocketストリーミング接続で動作させることができます。

## 機能

- 気象庁APIから最新のレーダーデータを取得
- Yahoo Maps APIを使用したジオコーディング対応
- 次の要素を含む合成画像を生成：
  - ベースマップタイル (OpenStreetMap)
  - 気象レーダーオーバーレイ
  - 距離円 (10km 〜 50km)
  - 落雷マーカー
- 地名と座標の両方を入力として受け入れ
- **Misskeyボット機能**:
  - メンションに自動応答
  - WebSocketストリーミング接続
  - 自動的に再接続する機能
  - エラーハンドリングと詳細ログ

## セットアップ

### 1. 依存関係のインストール

```bash
go mod tidy
```

**主な依存関係**:

- `github.com/gorilla/websocket`: WebSocket通信（Misskeyボット用）
- `github.com/cockroachdb/errors`: エラーハンドリング

### 2. 前提条件

1. [Yahoo Developer Network](https://developer.yahoo.co.jp/)からYahoo Maps APIキーを取得
2. 必要な環境変数を設定

### 3. MisskeyAPIトークンの取得（ボット使用時）

1. Misskeyインスタンスにログイン
2. 設定 → API → アクセストークンを生成
3. 次の権限を付与：
   - アカウントの情報を見る
   - ノートを作成・削除する
   - ドライブを操作する
   - リアクションを追加・削除する

## 使用方法

### Misskeyボットとして実行

```bash
# 環境変数を設定してから実行
export MISSKEY_DOMAIN=your-domain.com
export MISSKEY_API_TOKEN=your_token
export YAHOO_API_TOKEN=your_yahoo_token

# ソースから実行
go run cmd/misskey_bot/main.go
```

### スタンドアロンモードで実行

```bash
# 環境変数を設定
export YAHOO_API_TOKEN=your_api_key_here

# ソースから実行
go run cmd/cli/main.go 東京

# 座標で実行
go run cmd/cli/main.go "35.6762 139.6503"
```

### ビルド

```bash
# CLI版のビルド
go build -o hato-bot-go cmd/cli/main.go
./hato-bot-go 東京

# ボット版のビルド
go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go
./hato-bot-go-misskey-bot
```

### Docker Composeで実行

```bash
# 環境設定
cp .env.example .env
# .envファイルを編集してAPIトークンを設定

# Docker Composeで実行（推奨）
export TAG_NAME=$(git symbolic-ref --short HEAD | sed -e "s:/:-:g")
docker compose up -d

# 自動リロード付き開発モード（airを使用）
docker compose -f docker-compose.yml-f dev.docker-compose.yml up --build
```

## 使用するAPIエンドポイント

1. **気象庁タイムスタンプ**:
   - `https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N1.json`
   - `https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N2.json`
   - `https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N3.json`

2. **レーダータイル**:
   <!-- textlint-disable  ja-technical-writing/sentence-length -->
   - `https://www.jma.go.jp/bosai/jmatile/data/nowc/{timestamp}/none/{timestamp}/surf/hrpns/{z}/{x}/{y}.png`
   <!-- textlint-enable  ja-technical-writing/sentence-length -->

3. **落雷データ**:
   - `https://www.jma.go.jp/bosai/jmatile/data/nowc/{timestamp}/none/{timestamp}/surf/liden/data.geojson`

4. **ベースマップタイル**:
   - `https://tile.openstreetmap.org/{z}/{x}/{y}.png`

5. **Yahooジオコーディング**:
   - `https://map.yahooapis.jp/geocode/V1/geoCoder`

## コマンド（ボットモード）

### ameshコマンド

ボットにメンションして次のコマンドを送信してください。

```text
@bot amesh 東京
@bot amesh 大阪
@bot amesh 35.6762 139.6503
@bot amesh
```

- `amesh 地名`: 指定した地名の気象レーダー画像を生成
- `amesh 緯度 経度`: 指定した座標の気象レーダー画像を生成
- `amesh`: 東京の気象レーダー画像を生成（デフォルト）

## 出力

プログラムは`amesh_{地名}.png`という名前のPNG画像を生成します。画像には以下が含まれます。

- **ベースマップ**: OpenStreetMapタイル
- **気象レーダー**: 気象庁の雨雲データ（透明度付き）
- **落雷情報**: 落雷発生地点（シアンの円）
- **距離円**: 中心点から10km 〜 50kmの円

## 実装の詳細

### アーキテクチャ

- **`lib/amesh/amesh.go`**: 気象レーダー画像生成のコア機能
- **`cmd/cli/main.go`**: コマンドライン実行のためのCLI実装
- **`cmd/misskey_bot/main.go`**: MisskeyボットのWebSocket実装

### 技術仕様

- タイル計算にWebメルカトル投影を使用
- 透明度を持つ基本的な画像合成を実装
- 地理座標とピクセル座標間の座標変換を処理
- API失敗時のエラーハンドリングを含む
- WebSocketストリーミング接続（Misskeyボット）
- 自動的に再接続する機能とエラーハンドリング

## ログ（ボットモード）

ボットは次のログを出力します。

- WebSocket接続状況
- メンション受信状況
- コマンド処理状況
- エラー情報

## トラブルシューティング

### WebSocket接続エラー

```text
Failed to connect to WebSocket: ...
```

- `MISSKEY_DOMAIN`と`MISSKEY_API_TOKEN`がただしく設定されているか確認
- Misskeyインスタンスが正常に動作しているか確認
- APIトークンの権限がただしく設定されているか確認

### 画像生成エラー

```text
Error creating amesh image: ...
```

- `YAHOO_API_TOKEN`がただしく設定されているか確認
- Yahoo APIの利用制限に達していないか確認
- 気象庁APIが正常に動作しているか確認

### ファイルアップロードエラー

```text
Failed to upload file to Misskey: ...
```

- APIトークンにドライブ操作権限があるか確認
- Misskeyインスタンスのファイルサイズ制限を確認
- `/tmp`ディレクトリの書き込み権限を確認

## 機能の拡張

### 新しいコマンドの追加

1. `ParseAmeshCommand`関数を拡張してコマンドを解析
2. 対応する処理関数を`MisskeyBot`に追加
3. `messageHandler`で新しいコマンドを処理

## Python版との違い

- 簡素化された画像処理（複雑なマップスタイリングなし）
- 落雷マーカーの基本的な円描画
- キャッシュなしの直接API呼び出し
- 外部画像ライブラリに依存しないスタンドアロン実行ファイル

## ライセンス

この実装は、hato-botプロジェクトと同じライセンスに従います。
