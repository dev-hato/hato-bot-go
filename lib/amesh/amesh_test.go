package amesh_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/go-cmp/cmp"

	"hato-bot-go/lib"
	"hato-bot-go/lib/amesh"
	libHttp "hato-bot-go/lib/http"
)

// httpMockConfig モックHTTPクライアントの設定
type httpMockConfig struct {
	TimestampsResponse string
	LightningResponse  string
	DummyTileBytes     []byte
}

type roundTrip struct {
	Config httpMockConfig
}

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	switch {
	case strings.Contains(url, "targetTimes"):
		if f.Config.TimestampsResponse == "" {
			return mockResponse(http.StatusInternalServerError, "Internal Server Error"), nil
		}
		return mockResponse(http.StatusOK, f.Config.TimestampsResponse), nil
	case strings.Contains(url, "liden/data.geojson"):
		if f.Config.LightningResponse == "" {
			return mockResponse(http.StatusNotFound, "Not Found"), nil
		}
		return mockResponse(http.StatusOK, f.Config.LightningResponse), nil
	case strings.Contains(url, ".png"):
		return createPNGResponse(f.Config.DummyTileBytes), nil
	default:
		return mockResponse(http.StatusNotFound, "Not Found"), nil
	}
}

// TestCreateAmeshImage CreateAmeshImage関数をテストする
func TestCreateAmeshImage(t *testing.T) {
	dummyTileBytes, err := createDummyPNGBytes(
		256,
		256,
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name              string
		params            *amesh.CreateAmeshImageParams
		checkCenterColor  bool
		expectedImageSize int
		expectError       error
	}{
		{
			name: "成功した画像作成",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `{
				"features": [
					{
						"geometry": {
							"coordinates": [139.7, 35.7]
						},
						"properties": {
							"type": 1
						}
					}
				]
			}`,
					DummyTileBytes: dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "空のタイムスタンプ結果",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[]`,
					LightningResponse:  `{"features": []}`,
					DummyTileBytes:     dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "タイルダウンロード失敗を適切に処理",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `{"features": []}`,
					DummyTileBytes:    dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "不正なJSONタイムスタンプで処理継続",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `invalid json`,
					LightningResponse:  `{"features": []}`,
					DummyTileBytes:     dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "すべてのタイムスタンプAPIが失敗",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: "",
					LightningResponse:  `{"features": []}`,
					DummyTileBytes:     dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "落雷データJSONエラー",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `invalid json`,
					DummyTileBytes:    dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        10,
				AroundTiles: 1,
			},
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name: "小さなタイル数でのテスト",
			params: &amesh.CreateAmeshImageParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `{
				"features": [
					{
						"geometry": {
							"coordinates": [139.6917, 35.6895]
						},
						"properties": {
							"type": 1
						}
					},
					{
						"geometry": {
							"coordinates": [139.7, 35.7, 100]
						},
						"properties": {
							"type": 2
						}
					}
				]
			}`,
					DummyTileBytes: dummyTileBytes,
				}),
				Lat:         35.6895,
				Lng:         139.6917,
				Zoom:        5,
				AroundTiles: 0,
			},
			checkCenterColor:  false,
			expectedImageSize: 256,
			expectError:       nil,
		},
		{
			name:        "nilリクエスト",
			params:      nil,
			expectError: lib.ErrParamsNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := amesh.CreateAmeshImage(context.Background(), tt.params)
			if !errors.Is(err, tt.expectError) {
				t.Errorf("CreateAmeshImage() unexpected error: %v, expected: %v", err, tt.expectError)
				return
			}

			if result == nil {
				if tt.expectError == nil {
					t.Errorf("CreateAmeshImage() returned nil image")
				}

				return
			}

			bounds := result.Bounds()
			if bounds.Dx() != tt.expectedImageSize || bounds.Dy() != tt.expectedImageSize {
				t.Errorf("CreateAmeshImage() image size = %dx%d, want %dx%d",
					bounds.Dx(), bounds.Dy(), tt.expectedImageSize, tt.expectedImageSize)
				return
			}

			if tt.checkCenterColor {
				centerColor := result.RGBAAt(bounds.Dx()/2, bounds.Dy()/2)
				if centerColor.R != 255 || centerColor.G != 255 || centerColor.B != 255 || centerColor.A != 255 {
					t.Errorf("Expected white center pixel but got R=%d, G=%d, B=%d, A=%d",
						centerColor.R, centerColor.G, centerColor.B, centerColor.A)
				}
			}
		})
	}
}

// TestCreateImageReaderWithClient CreateImageReaderWithClient関数をテストする
func TestCreateImageReaderWithClient(t *testing.T) {
	dummyTileBytes, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		params      *amesh.CreateImageReaderWithClientParams
		expectError error
	}{
		{
			name: "成功したio.Reader作成",
			params: &amesh.CreateImageReaderWithClientParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `{"features": []}`,
					DummyTileBytes:    dummyTileBytes,
				}),
				Location: &amesh.Location{
					Lat:       35.6895,
					Lng:       139.6917,
					PlaceName: "東京",
				},
			},
			expectError: nil,
		},
		{
			name:        "nilリクエスト",
			params:      nil,
			expectError: lib.ErrParamsNil,
		},
		{
			name: "nilクライアント",
			params: &amesh.CreateImageReaderWithClientParams{
				Client: nil,
				Location: &amesh.Location{
					Lat:       35.6895,
					Lng:       139.6917,
					PlaceName: "東京",
				},
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name: "nilロケーション",
			params: &amesh.CreateImageReaderWithClientParams{
				Client: createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
					LightningResponse: `{"features": []}`,
					DummyTileBytes:    dummyTileBytes,
				}),
				Location: nil,
			},
			expectError: lib.ErrParamsNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := amesh.CreateImageReaderWithClient(context.Background(), tt.params)
			if !errors.Is(err, tt.expectError) {
				t.Errorf("CreateImageReaderWithClient() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError == nil {
				if result == nil {
					t.Error("CreateImageReaderWithClient() returned nil reader")
					return
				}

				// io.Readerからデータを読み取って、有効なPNGデータかチェック
				data, err := io.ReadAll(result)
				if err != nil {
					t.Error(err)
					return
				}

				// PNGデータのデコードを試行
				if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

// TestParseLocationWithClient ParseLocationWithClient関数をモックHTTPクライアントでテストする
func TestParseLocationWithClient(t *testing.T) {
	tests := []struct {
		name        string
		params      *amesh.ParseLocationWithClientParams
		expectError error
		expected    *amesh.Location
	}{
		{
			name: "成功したジオコーディング",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "test_key",
				},
			},
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京都",
			},
		},
		{
			name: "座標文字列の解析",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "35.6895 139.6917",
					APIKey: "dummy_key",
				},
			},
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "35.69,139.69",
			},
		},
		{
			name: "空の場所は東京がデフォルト",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "",
					APIKey: "test_key",
				},
			},
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京都",
			},
		},
		{
			name: "座標文字列（整数）",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "35 139",
					APIKey: "dummy",
				},
			},
			expected: &amesh.Location{
				Lat:       35.0,
				Lng:       139.0,
				PlaceName: "35.00,139.00",
			},
		},
		{
			name: "無効な座標文字列（1つの数値のみ）",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "34",
					APIKey: "test_key",
				},
			},
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京都",
			},
		},
		{
			name: "無効な座標文字列",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusBadRequest, `{"Error": "Invalid place"}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "invalid coordinates",
					APIKey: "test_key",
				},
			},
			expectError: libHttp.ErrHTTPRequestError,
		},
		{
			name: "無効な座標フォーマット",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "invalid_format"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "test_key",
				},
			},
			expectError: amesh.ErrInvalidCoordinatesFormat,
		},
		{
			name: "APIがエラーステータスを返す",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusBadRequest, `{"Error": "Invalid API key"}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "invalid_key",
				},
			},
			expectError: libHttp.ErrHTTPRequestError,
		},
		{
			name: "結果が見つからない",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{"Feature": []}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "nonexistent place",
					APIKey: "test_key",
				},
			},
			expectError: amesh.ErrNoResultsFound,
		},
		{
			name: "不正なJSON",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{"Feature": [invalid json}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "test_key",
				},
			},
			expectError: amesh.ErrJSONUnmarshal,
		},
		{
			name: "座標数が足りない場合",
			params: &amesh.ParseLocationWithClientParams{
				Client: libHttp.NewMockHTTPClient(http.StatusOK, `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917"
						}
					}
				]
			}`),
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "test_key",
				},
			},
			expectError: amesh.ErrInvalidCoordinatesFormat,
		},
		{
			name:        "nilリクエスト",
			params:      nil,
			expectError: lib.ErrParamsNil,
		},
		{
			name: "nilクライアント",
			params: &amesh.ParseLocationWithClientParams{
				Client: nil,
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  "東京",
					APIKey: "test_key",
				},
			},
			expectError: lib.ErrParamsNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := amesh.ParseLocationWithClient(context.Background(), tt.params)
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("ParseLocationWithClient() diff: %s", diff)
			}
			if !errors.Is(err, tt.expectError) {
				t.Errorf("ParseLocationWithClient() error = %v, expectError = %v", err, tt.expectError)
				return
			}
		})
	}
}

// TestGenerateFileName GenerateFileName関数をテストする
func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		name     string
		location *amesh.Location
	}{
		{
			name: "基本的なファイル名生成",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京",
			},
		},
		{
			name: "座標",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "35.6895,139.6917",
			},
		},
		{
			name: "空の地名",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "",
			},
		},
		{
			name: "特殊文字を含む地名",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京/新宿区",
			},
		},
		{
			name: "非常に長い地名",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: strings.Repeat("長い地名", 100),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := amesh.GenerateFileName(tt.location)

			// ファイル名の基本フォーマットをチェック
			if !strings.HasPrefix(result, "amesh_") {
				t.Errorf("GenerateFileName() result = %v, expected to start with 'amesh_'", result)
			}
			if !strings.HasSuffix(result, ".png") {
				t.Errorf("GenerateFileName() result = %v, expected to end with '.png'", result)
			}

			// 地名がファイル名に含まれているかチェック（スペースはアンダースコアに変換）
			expectedPlaceName := strings.ReplaceAll(tt.location.PlaceName, " ", "_")
			if !strings.Contains(result, expectedPlaceName) {
				t.Errorf(
					"GenerateFileName() result = %v, expected to contain place name %v",
					result,
					expectedPlaceName,
				)
			}

			// タイムスタンプが含まれているかチェック（数字が含まれていることを確認）
			hasNumber := false
			for _, char := range result {
				if char >= '0' && char <= '9' {
					hasNumber = true
					break
				}
			}
			if !hasNumber {
				t.Errorf("GenerateFileName() result = %v, expected to contain timestamp numbers", result)
			}
		})
	}
}

// createDummyPNGBytes ダミーのPNG画像バイトを作成する
func createDummyPNGBytes(width, height int, c color.Color) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, errors.Wrap(err, "Failed to png.Encode")
	}
	return buf.Bytes(), nil
}

// createConfigurableMockHTTPClient 設定可能なモックHTTPクライアントを作成
func createConfigurableMockHTTPClient(config httpMockConfig) *http.Client {
	return &http.Client{
		Transport: roundTrip{
			Config: config,
		},
	}
}

// mockResponse ヘルパー関数でHTTPレスポンスを作成
func mockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// createPNGResponse PNGファイル用のHTTPレスポンスを作成
func createPNGResponse(dummyTileBytes []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(dummyTileBytes)),
		Header:     make(http.Header),
	}
}
