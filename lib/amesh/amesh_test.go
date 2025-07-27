package amesh_test

import (
	"bytes"
	"hato-bot-go/lib/amesh"
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

// MockHTTPClient はテスト用のHTTPクライアントのモック
type MockHTTPClient struct {
	GetFunc func(url string) (*http.Response, error)
	DoFunc  func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(url)
	}
	return nil, nil
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, nil
}

// mockResponse ヘルパー関数でHTTPレスポンスを作成
func mockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
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
		expected     amesh.GeocodeResult
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
			expected: amesh.GeocodeResult{
				Lat:  35.6895,
				Lng:  139.6917,
				Name: "東京都",
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
			expected: amesh.GeocodeResult{
				Lat:  35.6895,
				Lng:  139.6917,
				Name: "東京都",
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
			mockClient := &MockHTTPClient{
				GetFunc: func(_ string) (*http.Response, error) {
					return mockResponse(tt.responseCode, tt.responseBody), nil
				},
			}

			result, err := amesh.GeocodeWithClient(mockClient, tt.place, tt.apiKey)
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("GeocodeWithClient(%q, %q) diff: %s", tt.place, tt.apiKey, diff)
			}
			if !errors.Is(err, tt.expectError) {
				t.Errorf("GeocodeWithClient(%q, %q) unexpected error: %v, excepted: %v", tt.place, tt.apiKey, err, tt.expectError)
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
		return nil, errors.Wrap(err, "failed to encode png")
	}
	return buf.Bytes(), nil
}

// testCase CreateAmeshImageWithClientのテストケース構造体
type testCase struct {
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
}

// createMockClient テスト用のモックHTTPクライアントを作成する
func createMockClient(tc testCase, dummyTileBytes []byte) *MockHTTPClient {
	return &MockHTTPClient{
		GetFunc: func(url string) (*http.Response, error) {
			switch {
			case strings.Contains(url, "targetTimes"):
				if tc.timestampsResponse == "" {
					return mockResponse(500, "Internal Server Error"), nil
				}
				return mockResponse(200, tc.timestampsResponse), nil
			case strings.Contains(url, "liden/data.geojson"):
				if tc.lightningResponse == "" {
					return mockResponse(404, "Not Found"), nil
				}
				return mockResponse(200, tc.lightningResponse), nil
			case strings.Contains(url, ".png"):
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(dummyTileBytes)),
					Header:     make(http.Header),
				}, nil
			default:
				return mockResponse(404, "Not Found"), nil
			}
		},
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), ".png") {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(dummyTileBytes)),
					Header:     make(http.Header),
				}, nil
			}
			return mockResponse(404, "Not Found"), nil
		},
	}
}

// validateImageResult CreateAmeshImageWithClientの結果を検証する
func validateImageResult(t *testing.T, result *image.RGBA, tc testCase) {
	if result == nil {
		t.Errorf("CreateAmeshImageWithClient() returned nil image")
		return
	}

	bounds := result.Bounds()
	if bounds.Dx() != tc.expectedImageSize || bounds.Dy() != tc.expectedImageSize {
		t.Errorf("CreateAmeshImageWithClient() image size = %dx%d, want %dx%d",
			bounds.Dx(), bounds.Dy(), tc.expectedImageSize, tc.expectedImageSize)
		return
	}

	if tc.checkCenterColor {
		centerColor := result.RGBAAt(bounds.Dx()/2, bounds.Dy()/2)
		if centerColor.R != 255 || centerColor.G != 255 || centerColor.B != 255 || centerColor.A != 255 {
			t.Errorf("Expected white center pixel but got R=%d, G=%d, B=%d, A=%d",
				centerColor.R, centerColor.G, centerColor.B, centerColor.A)
		}
	}
}

// getTestCases CreateAmeshImageWithClientのテストケースを返す
func getTestCases() []testCase {
	return []testCase{
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
}

// TestCreateAmeshImageWithClient CreateAmeshImageWithClient関数をテストする
func TestCreateAmeshImageWithClient(t *testing.T) {
	tests := getTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dummyTileBytes, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			if err != nil {
				t.Error(err)
			}
			mockClient := createMockClient(tt, dummyTileBytes)

			result, err := amesh.CreateAmeshImageWithClient(mockClient, tt.lat, tt.lng, tt.zoom, tt.aroundTiles)
			if !errors.Is(err, tt.expectError) {
				t.Errorf("CreateAmeshImageWithClient() unexpected error: %v, expected: %v", err, tt.expectError)
				return
			}

			validateImageResult(t, result, tt)
		})
	}
}
