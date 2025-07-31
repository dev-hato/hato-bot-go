package amesh_test

import (
	"bytes"
	"hato-bot-go/lib/amesh"
	libHttp "hato-bot-go/lib/http"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/go-cmp/cmp"
)

// mockFileWriter はテスト用のファイルライターのモック
type mockFileWriter struct {
	CreateFunc  func(name string) (io.WriteCloser, error)
	ShouldError bool
}

func (m *mockFileWriter) Create(name string) (io.WriteCloser, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(name)
	}
	if m.ShouldError {
		return nil, errors.New("mock file creation error")
	}
	return &mockWriteCloser{}, nil
}

// mockWriteCloser はテスト用のWriteCloserのモック
type mockWriteCloser struct {
	WriteData []byte
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	m.WriteData = append(m.WriteData, p...)
	return len(p), nil
}

func (m *mockWriteCloser) Close() error {
	return nil
}

// httpMockConfig モックHTTPクライアントの設定
type httpMockConfig struct {
	TimestampsResponse string
	LightningResponse  string
	DummyTileBytes     []byte
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
		expected     amesh.Location
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
			expected: amesh.Location{
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
			expected: amesh.Location{
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
			expectError:  amesh.ErrGeocodingAPIError,
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockClient := libHttp.NewMockHTTPClient(tt.responseCode, tt.responseBody)

			result, err := amesh.GeocodeWithClient(mockClient, &amesh.GeocodeRequest{
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

			result, err := amesh.CreateAmeshImageWithClient(mockClient, &amesh.CreateImageRequest{
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

// TestCreateAndSaveImageWithClient CreateAndSaveImageWithClient関数をモックを使用してテストする
func TestCreateAndSaveImageWithClient(t *testing.T) {
	tests := []struct {
		name         string
		location     *amesh.Location
		basePath     string
		fileError    bool
		expectError  bool
		expectedPath string
	}{
		{
			name: "成功した画像作成と保存",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京",
			},
			basePath:     "/tmp",
			fileError:    false,
			expectError:  false,
			expectedPath: "/tmp/amesh_東京_", // タイムスタンプが付与されるため部分一致
		},
		{
			name:        "nilロケーション",
			location:    nil,
			basePath:    "/tmp",
			fileError:   false,
			expectError: true,
		},
		{
			name: "ファイル作成エラー",
			location: &amesh.Location{
				Lat:       35.6895,
				Lng:       139.6917,
				PlaceName: "東京",
			},
			basePath:    "/tmp",
			fileError:   true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// ダミータイルデータを作成
			dummyTileBytes, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			if err != nil {
				t.Fatal(err)
			}

			// モックHTTPクライアントを作成
			mockHTTPClient := createConfigurableMockHTTPClient(httpMockConfig{
				TimestampsResponse: `[
			{
				"basetime": "20240101120000",
				"validtime": "20240101120000", 
				"elements": ["hrpns_nd", "liden"]
			}
		]`,
				LightningResponse: `{"features": []}`,
				DummyTileBytes:    dummyTileBytes,
			})

			// モックファイルライターを作成
			mockFileWriter := &mockFileWriter{
				ShouldError: tt.fileError,
			}

			filePath, err := amesh.CreateAndSaveImageWithClient(&amesh.CreateAndSaveImageRequest{
				Client:   mockHTTPClient,
				Writer:   mockFileWriter,
				Location: tt.location,
				BasePath: tt.basePath,
			})

			if (err != nil) != tt.expectError {
				t.Errorf("CreateAndSaveImageWithClient() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && tt.expectedPath != "" {
				if !strings.HasPrefix(filePath, tt.expectedPath) {
					t.Errorf("CreateAndSaveImageWithClient() filePath = %v, expected to start with %v", filePath, tt.expectedPath)
				}
			}
		})
	}
}

// TestCreateAndSaveImage CreateAndSaveImage関数をテストする（基本的なテストのみ）
func TestCreateAndSaveImage(t *testing.T) {
	tests := []struct {
		name        string
		location    *amesh.Location
		basePath    string
		expectError bool
	}{
		{
			name:        "nilロケーション",
			location:    nil,
			basePath:    "/tmp",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := amesh.CreateAndSaveImage(tt.location, tt.basePath)
			if (err != nil) != tt.expectError {
				t.Errorf("CreateAndSaveImage() error = %v, expectError %v", err, tt.expectError)
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
			expectError:  errors.New("geocoding API returned error status"),
		},
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
			name:         "APIエラー",
			place:        "東京",
			apiKey:       "invalid_key",
			responseCode: 400,
			responseBody: `{"Error": "Invalid API key"}`,
			expectError:  errors.New("geocoding API returned error status"),
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockClient := libHttp.NewMockHTTPClient(tt.responseCode, tt.responseBody)

			result, err := amesh.ParseLocationWithClient(&amesh.ParseLocationRequest{
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

// createConfigurableMockHTTPClient 設定可能なモックHTTPクライアントを作成
func createConfigurableMockHTTPClient(config httpMockConfig) *libHttp.MockHTTPClient {
	return &libHttp.MockHTTPClient{
		GetFunc: func(url string) (*http.Response, error) {
			switch {
			case strings.Contains(url, "targetTimes"):
				if config.TimestampsResponse == "" {
					return mockResponse(500, "Internal Server Error"), nil
				}
				return mockResponse(200, config.TimestampsResponse), nil
			case strings.Contains(url, "liden/data.geojson"):
				if config.LightningResponse == "" {
					return mockResponse(404, "Not Found"), nil
				}
				return mockResponse(200, config.LightningResponse), nil
			case strings.Contains(url, ".png"):
				return createPNGResponse(config.DummyTileBytes), nil
			default:
				return mockResponse(404, "Not Found"), nil
			}
		},
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), ".png") {
				return createPNGResponse(config.DummyTileBytes), nil
			}
			return mockResponse(404, "Not Found"), nil
		},
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
