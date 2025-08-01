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
go mod download
```

**主な依存関係**:

- `github.com/gorilla/websocket`: WebSocket通信（Misskeyボット用）
- `github.com/cockroachdb/errors`: エラーハンドリング

### 1.1. 開発環境のセットアップ

開発時はpre-commitフックをインストールしてください。

```bash
# pre-commitのインストール（gitleaksによるシークレットスキャン）
uv tool run pre-commit install
```

これにより、コミット時に自動的にgitleaksによるシークレットスキャンが実行されます。

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
# Misskeyの環境変数設定
export MISSKEY_API_TOKEN=your_misskey_api_token
export MISSKEY_DOMAIN=your-misskey-instance.com
# Yahoo APIの環境変数設定
export YAHOO_API_TOKEN=your_yahoo_api_token

# ソースから実行
go run cmd/misskey_bot/main.go
```

### スタンドアロンモードで実行

```bash
# Yahoo APIの環境変数設定
export YAHOO_API_TOKEN=your_yahoo_api_token


# ソースから実行
go run cmd/cli/main.go amesh 東京

# 座標で実行
go run cmd/cli/main.go amesh "35.6762 139.6503"
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
docker compose -f docker-compose.yml -f dev.docker-compose.yml up --build
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

### 描画アルゴリズムの技術的詳細

気象レーダー画像の生成には複数の描画アルゴリズムと座標変換技術を組み合わせています。

#### 1. 座標変換システム

**Webメルカトル投影 (`getWebMercatorPixel`)**

```go
zoomFactor := float64(int(1) << uint(params.Zoom))
x := 256.0 * zoomFactor * (params.Lng + 180) / 360.0
y := 256.0 * zoomFactor * (0.5 - math.Log(math.Tan(math.Pi/4+deg2rad(params.Lat)/2))/(2.0*math.Pi))
```

- 地理座標（度数）をピクセル座標に変換
- ズームレベルに応じたスケール調整
- 地図タイルの標準的な座標系を使用

#### 2. タイル合成システム

**マルチレイヤー画像合成**

1. **ベースマップ描画**: OpenStreetMapタイルを`draw.Over`モードで合成
2. **レーダーオーバーレイ**: 気象庁データを半透明（Alpha=128）で重ね合わせ
3. **装飾要素**: 距離円と落雷マーカーをピクセル単位で描画

```go
// ベースタイル描画
draw.Draw(img, destRect, baseTile, image.Point{}, draw.Over)

// レーダータイル透明度付き描画
draw.DrawMask(img, destRect, radarTile, image.Point{},
    image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 128}),
    image.Point{}, draw.Over)
```

#### 3. 距離円描画アルゴリズム

**地理的距離円の線分近似 (`drawDistanceCircle`)**

- **64個の線分**で円を近似
- **地球の曲率**を考慮した地理的な距離の計算
- **球面座標系**での円上の点計算

```go
// 地理座標での円上の点計算（地球の曲率を考慮）
lat1 := params.CreateImageRequest.Lat + (params.RadiusKm/earthRadius)*math.Cos(angle1)*180/math.Pi
lng1 := params.CreateImageRequest.Lng + (params.RadiusKm/earthRadius)*math.Sin(angle1)*180/math.Pi/math.Cos(deg2rad(params.CreateImageRequest.Lat))
```

**技術的特徴:**

- 地球半径（6371km）を使用した正確な距離計算
- 緯度による経度補正を適用
- ピクセル座標変換後にブレゼンハム直線描画

#### 4. 直線描画アルゴリズム

**ブレゼンハムライン描画 (`drawLine`)**

```go
// デルタエラー法による効率的な直線描画
delta := dx - dy
for {
    params.Img.Set(x, y, params.Col)

    d2 := 2 * delta
    if -dy < d2 {
        delta -= dy
        x += sx
    }
    if d2 < dx {
        delta += dx
        y += sy
    }
}
```

**アルゴリズムの利点:**

- 整数演算のみで高速処理
- アンチエイリアシングなしのクリアな線描画
- メモリ効率的なピクセル単位操作

#### 5. 落雷マーカー描画

**円形塗りつぶしアルゴリズム (`drawLightningMarker`)**

```go
// ピタゴラスの定理による円内判定
for dy := -radius; dy <= radius; dy++ {
    for dx := -radius; dx <= radius; dx++ {
        if radius*radius < dx*dx+dy*dy {
            continue
        }
        params.Img.Set(x, y, lightningColor)
    }
}
```

**特徴:**

- 半径7ピクセルの塗りつぶし円
- 境界チェックによるオーバーフロー防止
- シアン色（`color.RGBA{G: 255, B: 255, A: 255}`）での視認性向上

#### 6. 描画パフォーマンス最適化

**効率的な描画処理:**

- タイル単位（256x256ピクセル）での並列処理
- HTTPクライアントの再利用
- エラー発生時のグレースフルデグラデーション
- メモリ効率的なRGBA画像操作

この実装は、地理情報システム（GIS）の基本的な描画技術を組み合わせ、効率的で正確な気象レーダー画像を生成します。

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
2. 対応する処理関数を`Bot`に追加
3. `messageHandler`で新しいコマンドを処理

## Python版との違い

- 簡素化された画像処理（複雑なマップスタイリングなし）
- 落雷マーカーの基本的な円描画
- キャッシュなしの直接API呼び出し
- 外部画像ライブラリに依存しないスタンドアロン実行ファイル

## ライセンス

この実装は、hato-botプロジェクトと同じライセンスに従います。
