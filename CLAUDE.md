# CLAUDE.md

このファイルは、Claude Code (claude.ai/code) がこのリポジトリでコードを扱う際のガイダンスを提供します。

## 概要

これは「hato-bot」の気象レーダー機能のGo実装で、気象庁のデータを使用して気象レーダー画像を生成するameshコマンドを提供します。スタンドアロンプログラムとして実行するか、WebSocketストリーミング接続を使用したMisskeyボットとして動作させることができます。

**注意**: これは元のPython版hato-botの一部をGoに移植したもので、気象レーダー画像生成とMisskey統合に焦点を当てています。

## 開発コマンド

### セットアップ

```bash
# Go依存関係のインストール
go mod download

# Node.js依存関係のインストール（リンティング用）
npm install

# pre-commitフックのインストール（gitleaksによるシークレットスキャン）
uv tool run pre-commit install
```

### ボットの実行

```bash
# 環境設定
cp .env.example .env
# .envファイルを編集してAPIトークンを設定

# CLI版のビルドと実行
go build -o hato-bot-go cmd/cli/main.go
./hato-bot-go 東京

# Misskeyボットのビルドと実行
go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go
./hato-bot-go-misskey-bot

# Docker Composeで実行（推奨）
export TAG_NAME=$(git symbolic-ref --short HEAD | sed -e "s:/:-:g")
docker compose up -d --wait

# 自動リロード付き開発モード
docker compose -f docker-compose.yml -f dev.docker-compose.yml up --build
```

### テストとリンティング

```bash
# Goテストの実行
go test ./...

# 包括的なリンティング実行
npm run lint

# 個別のリントコマンド
npm run lint:markdown     # Markdownファイル
npm run lint:text         # textlintによるテキストリンティング
npm run lint:dockerfile   # Dockerfileリンティング
npm run lint:secret       # gitleaksによるシークレットスキャン

# Goコードのフォーマット
go mod tidy
gosimports -w .

# 各プラットフォーム向けビルド
go build -o hato-bot-go cmd/cli/main.go
go build -o hato-bot-go-misskey-bot cmd/misskey_bot/main.go
```

## アーキテクチャ

### コアコンポーネント

- **`cmd/cli/main.go`**: スタンドアロンのCLIエントリーポイント
- **`cmd/misskey_bot/main.go`**: WebSocketストリーミング付きMisskeyボットエントリーポイント
- **`cmd/health_check/main.go`**: コンテナオーケストレーション用ヘルスチェックサービス
- **`lib/amesh/amesh.go`**: 気象レーダー画像生成のコア機能
- **`lib/misskey/misskey.go`**: Misskey APIクライアントとWebSocket処理
- **Docker設定**: 開発環境と本番環境用のマルチステージビルド

### プラットフォームサポート

このGo実装は以下に焦点を当てています。

- **Misskey**: 自動的に再接続する機能付きWebSocketストリーミング接続
- **スタンドアロンCLI**: テストと開発用の直接コマンドライン実行

### 外部API

複数の気象・地図サービスと統合。

- **気象庁**: 気象レーダーデータと落雷情報
- **Yahoo Maps API**: 位置ベースクエリのジオコーディング
- **OpenStreetMap**: 画像合成用ベースマップタイル

### 主要依存関係

- **WebSocket**: Misskeyストリーミング接続用の`github.com/gorilla/websocket`
- **エラーハンドリング**: 拡張エラー管理用の`github.com/cockroachdb/errors`
- **画像処理**: レーダー画像合成用のGoビルトイン`image`パッケージ
- **HTTPクライアント**: カスタムタイムアウト設定付き標準`net/http`

### コマンドシステム

amesh気象レーダーコマンドを処理。

1. **Misskeyボット**: WebSocket経由でメンションを監視し`amesh`コマンドを処理
2. **CLIモード**: 位置引数による直接コマンド実行
3. **画像生成**: 気象データを取得し、ベースマップにレーダーオーバーレイを合成、距離円と落雷マーカーを追加

### 環境設定

主要な環境変数（`.env`で定義）。

- `MISSKEY_API_TOKEN`, `MISSKEY_DOMAIN`: Misskeyボット統合
- `YAHOO_API_TOKEN`: ジオコーディング用Yahoo Maps API

**必要なMisskey API権限**：

- アカウント情報へのアクセス
- ノートの作成・削除
- ドライブファイル操作
- リアクション管理

### テスト

テストファイルはパッケージ別に整理。

- `lib/amesh/amesh_test.go`: HTTPモッキング付き気象レーダー機能テスト
- `lib/misskey/misskey_test.go`: Misskey APIクライアントテスト
- テーブル駆動テストとHTTPクライアントモッキングを使用したGo標準テストパッケージを使用

## 開発プロセス（t-wada式TDD推奨）

このプロジェクトでは、和田卓人氏（t-wada）が推奨するテスト駆動開発（TDD）の実践に従います。

### TDDの基本サイクル

1. **TODOリストの作成**
   - 実装したい機能要件を小さなタスクに分割してリストアップ
   - テストしやすく、優先度の高い要件から選択
   - 不安を取り除き、開発のロードマップとして活用

2. **Red-Green-Refactorサイクル**
   1. Red: 失敗するテストを書く
   2. Green: テストを通す最小限のコードを書く
   3. Refactor: テストを保ったままコードを改善
   4. 繰り返し

3. **TDDの3つの本質**
   - **心の状態の制御**: 大きな機能を小さなTODOに分割し、不安を取り除く
   - **開発のロードマップ**: テストが正しい道筋を示し、素早い復旧を可能にする
   - **開発の予測可能性**: 人間の不確実性を減らし、反復的な検証を自動化

### 実践原則

- **「作る」よりも先に「使う」**: 使いやすさを念頭に置いた設計
- **One assertion per test**: 1つのテストに1つのアサーション
- **テストは動作する仕様書**: コードの振る舞いを文書化
- **最終目標**: 「動作するきれいなコード」

### Go言語でのTDD実践

```bash
# 1. TODOリストを作成してからテストを書く
# 2. テストを実行して失敗を確認
go test ./...

# 3. 最小限のコードで通す
# 4. リファクタリング（フォーマット含む）
go mod tidy
gosimports -w .

# 5. すべてのテストが通ることを確認
go test ./...
```

### テスト構造

- **Prepare**: テストインスタンスのセットアップ
- **Execute**: テスト値の作成・実行
- **Verify**: assertEqualなどでの検証

### テストでのモック使用原則

外部依存関係を持つ機能をテストする際は、実際の外部リソースにアクセスしないよう**依存関係注入とモック**を使用する。

- **HTTPリクエストを伴うテスト**: `HTTPClient`インターフェースを通してモックHTTPクライアントを注入し、実際のAPIコールを回避する
- **ファイル書き込みを伴うテスト**: `FileWriter`インターフェースを通してモックファイルライターを注入し、実際のファイル操作を回避する
- **データベースアクセスを伴うテスト**: データベースインターフェースを通してモック実装を注入する
- **外部サービス呼び出しを伴うテスト**: サービスインターフェースを通してモック実装を注入する

#### モックテストの例

```go
// MockHTTPClient テスト用のHTTPクライアントモック
type MockHTTPClient struct {
    GetFunc func(url string) (*http.Response, error)
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
    if m.GetFunc != nil {
        return m.GetFunc(url)
    }
    return nil, nil
}

// MockFileWriter テスト用のファイルライターモック
type MockFileWriter struct {
    CreateFunc func(name string) (io.WriteCloser, error)
}

func (m *MockFileWriter) Create(name string) (io.WriteCloser, error) {
    if m.CreateFunc != nil {
        return m.CreateFunc(name)
    }
    return &MockWriteCloser{}, nil
}
```

この原則により、テストは高速で予測可能になり、外部依存関係の影響を受けずに実行できる。

## コーディング規約

### コメント記述

- **ソースコード内のコメントは日本語で記述すること**
- 関数の説明、変数の説明、処理の説明はすべて日本語を使用
- 英語のコメントは避け、日本語で分かりやすく記述する
- パッケージレベルのドキュメントコメントも日本語で記述

#### コメント記述の例

```go
// CreateAmeshImage 気象レーダー画像を作成する
func CreateAmeshImage(lat, lng float64, zoom, aroundTiles int) (*image.RGBA, error) {
    // 最新のタイムスタンプを取得
    timestamps, err := getLatestTimestamps()
    if err != nil {
        return nil, errors.Wrap(err, "Failed to getLatestTimestamps")
    }
    // ...
}
```

### 関数設計の原則

- **context.Context型の引数を除いたメソッドの引数が3つ以上ある場合はそれらを構造体としてまとめる**
  - 引数の数を減らし、可読性を向上させる
  - 引数の順序に依存しないよう設計する
  - 構造体のフィールドには適切な日本語コメントを付ける

- **errorを除いたメソッドの返り値が2つ以上ある場合はそれらを構造体としてまとめる**
  - 返り値の意味を明確にする
  - 将来の拡張に対して柔軟性を保つ
  - 構造体のフィールドには適切な日本語コメントを付ける

#### 関数設計の原則の例

```go
// DrawCircleRequest 円の描画リクエスト構造体
type DrawCircleRequest struct {
    Img      *image.RGBA // 描画対象の画像
    CenterX  int         // 中心のX座標
    CenterY  int         // 中心のY座標
    Radius   int         // 半径
    Color    color.RGBA  // 描画色
}

// DrawCircle 円を描画する
func DrawCircle(ctx context.Context, req *DrawCircleRequest) error {
    // 実装...
}
```

## 開発ノート

- **Goモジュール**: Go 1.24とモジュール依存関係管理を使用
- **Dockerコンテナ化**: 開発・本番環境用マルチステージビルド
- **エラーハンドリング**: 全体を通した包括的なエラーラッピングとログ記録
- **WebSocket復元力**: 指数バックオフによる自動再接続
- **画像合成**: 外部依存なしでGoのimageパッケージを使用したカスタム実装
- **API統合**: レート制限とタイムアウトシナリオを適切に処理
- **テスト**: 外部API呼び出し用HTTPクライアントモッキング付き広範なユニットテスト
