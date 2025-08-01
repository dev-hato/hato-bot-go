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
			return mockResponse(500, "Internal Server Error"), nil
		}
		return mockResponse(200, f.Config.TimestampsResponse), nil
	case strings.Contains(url, "liden/data.geojson"):
		if f.Config.LightningResponse == "" {
			return mockResponse(404, "Not Found"), nil
		}
		return mockResponse(200, f.Config.LightningResponse), nil
	case strings.Contains(url, ".png"):
		return createPNGResponse(f.Config.DummyTileBytes), nil
	default:
		return mockResponse(404, "Not Found"), nil
	}
}

// TestGeocodeWithClient GeocodeWithClient関数をモックHTTPレスポンスでテストする
func TestGeocodeWithClient(t *testing.T) {
	tests := []struct {
		name         string
		place        string
		apiKey       string
		responseCode int
		responseBody string
		expectError  error
		expected     *amesh.Location
	}{
		{
			name:         "成功したジオコーディング",
			place:        "東京",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`,
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京都",
			},
		},
		{
			name:         "空の場所は東京がデフォルト",
			place:        "",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "139.6917,35.6895"
						}
					}
				]
			}`,
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京都",
			},
		},
		{
			name:         "APIがエラーステータスを返す",
			place:        "東京",
			apiKey:       "invalid_key",
			responseCode: 400,
			responseBody: `{"Error": "Invalid API key"}`,
			expectError:  libHttp.ErrHTTPRequestError,
		},
		{
			name:         "結果が見つからない",
			place:        "nonexistent place",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{"Feature": []}`,
			expectError:  amesh.ErrNoResultsFound,
		},
		{
			name:         "無効な座標フォーマット",
			place:        "東京",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{
				"Feature": [
					{
						"Name": "東京都",
						"Geometry": {
							"Coordinates": "invalid_format"
						}
					}
				]
			}`,
			expectError: amesh.ErrInvalidCoordinatesFormat,
		},
		{
			name:         "不正なJSON",
			place:        "東京",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{"Feature": [invalid json}`,
			expectError:  amesh.ErrJSONUnmarshal,
		},
	}

	// jscpd:ignore-start
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockClient := libHttp.NewMockHTTPClient(tt.responseCode, tt.responseBody)

			result, err := amesh.GeocodeWithClient(context.Background(), mockClient, &amesh.GeocodeRequest{
				Place:  tt.place,
				APIKey: tt.apiKey,
			})
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("GeocodeWithClient(%q, %q) diff: %s", tt.place, tt.apiKey, diff)
			}
			if !errors.Is(err, tt.expectError) {
				t.Errorf("GeocodeWithClient(%q, %q) unexpected error: %v, excepted: %v", tt.place, tt.apiKey, err, tt.expectError)
			}
		})
	}
	// jscpd:ignore-end
}

// TestCreateAmeshImageWithClient CreateAmeshImageWithClient関数をテストする
func TestCreateAmeshImageWithClient(t *testing.T) {
	tests := []struct {
		name               string
		lat                float64
		lng                float64
		zoom               int
		aroundTiles        int
		timestampsResponse string
		lightningResponse  string
		checkCenterColor   bool
		expectedImageSize  int
		expectError        error
	}{
		{
			name:        "成功した画像作成",
			lat:         35.6895,
			lng:         139.6917,
			zoom:        10,
			aroundTiles: 1,
			timestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
			lightningResponse: `{
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
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name:               "空のタイムスタンプ結果",
			lat:                35.6895,
			lng:                139.6917,
			zoom:               10,
			aroundTiles:        1,
			timestampsResponse: `[]`,
			lightningResponse:  `{"features": []}`,
			checkCenterColor:   true,
			expectedImageSize:  768,
			expectError:        nil,
		},
		{
			name:        "タイルダウンロード失敗を適切に処理",
			lat:         35.6895,
			lng:         139.6917,
			zoom:        10,
			aroundTiles: 1,
			timestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
			lightningResponse: `{"features": []}`,
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name:               "不正なJSONタイムスタンプで処理継続",
			lat:                35.6895,
			lng:                139.6917,
			zoom:               10,
			aroundTiles:        1,
			timestampsResponse: `invalid json`,
			lightningResponse:  `{"features": []}`,
			checkCenterColor:   true,
			expectedImageSize:  768,
			expectError:        nil,
		},
		{
			name:               "すべてのタイムスタンプAPIが失敗",
			lat:                35.6895,
			lng:                139.6917,
			zoom:               10,
			aroundTiles:        1,
			timestampsResponse: "",
			lightningResponse:  `{"features": []}`,
			checkCenterColor:   true,
			expectedImageSize:  768,
			expectError:        nil,
		},
		{
			name:        "落雷データJSONエラー",
			lat:         35.6895,
			lng:         139.6917,
			zoom:        10,
			aroundTiles: 1,
			timestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
			lightningResponse: `invalid json`,
			checkCenterColor:  true,
			expectedImageSize: 768,
			expectError:       nil,
		},
		{
			name:        "小さなタイル数でのテスト",
			lat:         35.6895,
			lng:         139.6917,
			zoom:        5,
			aroundTiles: 0,
			timestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
			lightningResponse: `{
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
			checkCenterColor:  false,
			expectedImageSize: 256,
			expectError:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dummyTileBytes, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			if err != nil {
				t.Error(err)
			}
			mockClient := createConfigurableMockHTTPClient(httpMockConfig{
				TimestampsResponse: tt.timestampsResponse,
				LightningResponse:  tt.lightningResponse,
				DummyTileBytes:     dummyTileBytes,
			})

			result, err := amesh.CreateAmeshImageWithClient(context.Background(), mockClient, &amesh.CreateImageRequest{
				Lat:         tt.lat,
				Lng:         tt.lng,
				Zoom:        tt.zoom,
				AroundTiles: tt.aroundTiles,
			})
			if !errors.Is(err, tt.expectError) {
				t.Errorf("CreateAmeshImageWithClient() unexpected error: %v, expected: %v", err, tt.expectError)
				return
			}

			if result == nil {
				t.Errorf("CreateAmeshImageWithClient() returned nil image")
				return
			}

			bounds := result.Bounds()
			if bounds.Dx() != tt.expectedImageSize || bounds.Dy() != tt.expectedImageSize {
				t.Errorf("CreateAmeshImageWithClient() image size = %dx%d, want %dx%d",
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
	tests := []struct {
		name               string
		request            *amesh.CreateImageReaderRequest
		timestampsResponse string
		lightningResponse  string
		expectError        bool
	}{
		{
			name: "成功したio.Reader作成",
			request: &amesh.CreateImageReaderRequest{
				Location: &amesh.Location{
					Lat:       35.6895,
					Lng:       139.6917,
					PlaceName: "東京",
				},
			},
			timestampsResponse: `[
				{
					"basetime": "20240101120000",
					"validtime": "20240101120000", 
					"elements": ["hrpns_nd", "liden"]
				}
			]`,
			lightningResponse: `{"features": []}`,
			expectError:       false,
		},
		{
			name:        "nilリクエスト",
			request:     nil,
			expectError: true,
		},
		{
			name: "nilクライアント",
			request: &amesh.CreateImageReaderRequest{
				Client: nil,
				Location: &amesh.Location{
					Lat:       35.6895,
					Lng:       139.6917,
					PlaceName: "東京",
				},
			},
			expectError: true,
		},
		{
			name: "nilロケーション",
			request: &amesh.CreateImageReaderRequest{
				Location: nil,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// モッククライアントを設定（リクエストがnilでない場合のみ）
			if tt.request != nil && tt.request.Client == nil && !tt.expectError {
				dummyTileBytes, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
				if err != nil {
					t.Fatal(err)
				}
				tt.request.Client = createConfigurableMockHTTPClient(httpMockConfig{
					TimestampsResponse: tt.timestampsResponse,
					LightningResponse:  tt.lightningResponse,
					DummyTileBytes:     dummyTileBytes,
				})
			}

			result, err := amesh.CreateImageReaderWithClient(context.Background(), tt.request)

			if (err != nil) != tt.expectError {
				t.Errorf("CreateImageReaderWithClient() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
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
		name         string
		place        string
		apiKey       string
		responseCode int
		responseBody string
		expectError  error
		expected     *amesh.Location
	}{
		{
			name:        "座標文字列の解析",
			place:       "35.6895 139.6917",
			apiKey:      "dummy_key",
			expectError: nil,
			expected: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "35.69,139.69",
			},
		},
		{
			name:         "無効な座標文字列",
			place:        "invalid coordinates",
			apiKey:       "test_key",
			responseCode: 400,
			responseBody: `{"Error": "Invalid place"}`,
			expectError:  libHttp.ErrHTTPRequestError,
		},
		{
			name:         "APIエラー",
			place:        "東京",
			apiKey:       "invalid_key",
			responseCode: 400,
			responseBody: `{"Error": "Invalid API key"}`,
			expectError:  libHttp.ErrHTTPRequestError,
		},
		{
			name:         "結果が見つからない",
			place:        "nonexistent place",
			apiKey:       "test_key",
			responseCode: 200,
			responseBody: `{"Feature": []}`,
			expectError:  errors.New("no results found for place"),
		},
	}

	// jscpd:ignore-start
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockClient := libHttp.NewMockHTTPClient(tt.responseCode, tt.responseBody)

			result, err := amesh.ParseLocationWithClient(context.Background(), &amesh.ParseLocationRequest{
				Client: mockClient,
				GeocodeRequest: amesh.GeocodeRequest{
					Place:  tt.place,
					APIKey: tt.apiKey,
				},
			})
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("ParseLocationWithClient() diff: %s", diff)
			}
			if !errors.Is(err, tt.expectError) {
				t.Errorf("ParseLocationWithClient() error = %v, expectError %v", err, tt.expectError)
				return
			}
		})
	}
	// jscpd:ignore-end
}

// TestGenerateFileName GenerateFileName関数をテストする
func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		name        string
		location    *amesh.Location
		expectError bool
	}{
		{
			name: "基本的なファイル名生成",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京",
			},
			expectError: false,
		},
		{
			name: "座標",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "35.6895,139.6917",
			},
			expectError: false,
		},
		{
			name: "空の地名",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := amesh.GenerateFileName(tt.location)

			if !tt.expectError {
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
					t.Errorf("GenerateFileName() result = %v, expected to contain place name %v", result, expectedPlaceName)
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
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(dummyTileBytes)),
		Header:     make(http.Header),
	}
}
