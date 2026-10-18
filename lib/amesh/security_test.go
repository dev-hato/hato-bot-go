package amesh_test

import (
	"context"
	"image/color"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib/amesh"
	"hato-bot-go/lib/httpclient"
)

// validGeocodeResponse 正常なジオコーディングのレスポンス
const validGeocodeResponse = `{"Feature": [{"Name": "東京都", "Geometry": {"Coordinates": "139.6917,35.6895"}}]}`

// parseLocationWithMock 指定したレスポンスを返すモックを使ってParseLocationWithClientを呼び出す
func parseLocationWithMock(t *testing.T, place, response string) (*amesh.Location, error) {
	t.Helper()

	return amesh.ParseLocationWithClient(t.Context(), &amesh.ParseLocationWithClientParams{
		Client:         httpclient.NewMockHTTPClient(http.StatusOK, response),
		GeocodeRequest: amesh.GeocodeRequest{Place: place, APIKey: "test_key"},
	})
}

// createWhiteDummyTile 想定どおりの大きさの白いタイル画像を作成する
func createWhiteDummyTile(t *testing.T) []byte {
	t.Helper()

	dummyTile, err := createDummyPNGBytes(256, 256, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if err != nil {
		t.Fatal(err)
	}

	return dummyTile
}

// TestParseLocationWithClientCoordinateRange 範囲の境界ちょうどの座標は受け付け、範囲外や非数の座標は拒否することをテストする
func TestParseLocationWithClientCoordinateRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		place           string
		geocodeResponse string
		wantErr         error
		wantLat         float64
		wantLng         float64
	}{
		{name: "北東の境界", place: "85.05112878 180", geocodeResponse: validGeocodeResponse, wantLat: 85.05112878, wantLng: 180},
		{name: "南西の境界", place: "-85.05112878 -180", geocodeResponse: validGeocodeResponse, wantLat: -85.05112878, wantLng: -180},
		{
			name:            "ジオコーディング結果が境界",
			place:           "どこか",
			geocodeResponse: `{"Feature": [{"Name": "どこか", "Geometry": {"Coordinates": "180,85.05112878"}}]}`,
			wantLat:         85.05112878,
			wantLng:         180,
		},
		{name: "北極点", place: "90 139", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{name: "北極点の近く", place: "89.9999999 139", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{name: "南極点", place: "-90 139", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{name: "緯度がNaN", place: "NaN 139", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{name: "緯度が無限大", place: "Inf 0", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{name: "経度が巨大", place: "0 1e300", geocodeResponse: validGeocodeResponse, wantErr: amesh.ErrCoordinatesOutOfRange},
		{
			name:            "ジオコーディング結果の緯度が範囲外",
			place:           "どこか",
			geocodeResponse: `{"Feature": [{"Name": "どこか", "Geometry": {"Coordinates": "139,90"}}]}`,
			wantErr:         amesh.ErrCoordinatesOutOfRange,
		},
		{
			name:            "ジオコーディング結果の緯度がNaN",
			place:           "どこか",
			geocodeResponse: `{"Feature": [{"Name": "どこか", "Geometry": {"Coordinates": "139,NaN"}}]}`,
			wantErr:         amesh.ErrCoordinatesOutOfRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseLocationWithMock(t, tt.place, tt.geocodeResponse)

			// 拒否する座標では、エラーを返して結果を返さないことを確認する
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ParseLocationWithClient() error = %v, want %v", err, tt.wantErr)
				}
				if result != nil {
					t.Errorf("ParseLocationWithClient() result = %+v, want nil", result)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseLocationWithClient() error = %v", err)
			}
			if result.Lat != tt.wantLat || result.Lng != tt.wantLng {
				t.Errorf("ParseLocationWithClient() = (%v, %v), want (%v, %v)", result.Lat, result.Lng, tt.wantLat, tt.wantLng)
			}
		})
	}
}

// TestParseLocationWithClientRejectsTooLargeResponse 大きすぎるレスポンスを拒否することをテストする
func TestParseLocationWithClientRejectsTooLargeResponse(t *testing.T) {
	t.Parallel()

	// 上限を超える大きさのレスポンスを返す
	result, err := parseLocationWithMock(
		t,
		"東京",
		`{"Feature": [], "Padding": "`+strings.Repeat("a", 11*1024*1024)+`"}`,
	)
	if !errors.Is(err, amesh.ErrResponseTooLarge) {
		t.Errorf("ParseLocationWithClient() error = %v, want %v", err, amesh.ErrResponseTooLarge)
	}
	if result != nil {
		t.Errorf("ParseLocationWithClient() result = %+v, want nil", result)
	}
}

// TestCreateAmeshImageRejectsInvalidCoordinates 範囲外の座標で画像を作らないことをテストする
// 極付近では距離円の描画が終わらなくなるため、座標を検証せずに描画するとこのテストはタイムアウトする
func TestCreateAmeshImageRejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{name: "北極点", lat: 90, lng: 139},
		{name: "北極点の近く", lat: 89.9999999, lng: 139},
		{name: "経度が範囲外", lat: 35, lng: 181},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img, err := amesh.CreateAmeshImage(t.Context(), &amesh.CreateAmeshImageParams{
				Client:      httpclient.NewMockHTTPClient(http.StatusNotFound, "Not Found"),
				Lat:         tt.lat,
				Lng:         tt.lng,
				Zoom:        10,
				AroundTiles: 1,
			})
			if !errors.Is(err, amesh.ErrCoordinatesOutOfRange) {
				t.Errorf("CreateAmeshImage() error = %v, want %v", err, amesh.ErrCoordinatesOutOfRange)
			}
			if img != nil {
				t.Error("CreateAmeshImage() img should be nil")
			}
		})
	}
}

// TestCreateAmeshImageSkipsOversizedTile 想定より大きいタイル画像を描画しないことをテストする
func TestCreateAmeshImageSkipsOversizedTile(t *testing.T) {
	t.Parallel()

	// 想定の256x256を超える赤いタイル
	oversizedTile, err := createDummyPNGBytes(512, 512, color.RGBA{R: 255, A: 255})
	if err != nil {
		t.Fatal(err)
	}

	img, err := amesh.CreateAmeshImage(t.Context(), &amesh.CreateAmeshImageParams{
		Client:      createConfigurableMockHTTPClient(httpMockConfig{DummyTileBytes: oversizedTile}),
		Lat:         35.6895,
		Lng:         139.6917,
		Zoom:        10,
		AroundTiles: 1,
	})
	if err != nil {
		t.Fatalf("CreateAmeshImage() error = %v", err)
	}

	center := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	want := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// タイルを描画しなければ、中心は白い背景のままになる
	if center != want {
		t.Errorf("center color = %v, want %v", center, want)
	}
}

// recordingRoundTrip リクエストされたURLを記録するモック
type recordingRoundTrip struct {
	mu   *sync.Mutex
	urls *[]string
	base roundTrip
}

func (r recordingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	*r.urls = append(*r.urls, req.URL.String())
	r.mu.Unlock()

	return r.base.RoundTrip(req)
}

// TestCreateAmeshImageIgnoresInvalidTimestamp 不正な形式のタイムスタンプをURLに埋め込まないことをテストする
func TestCreateAmeshImageIgnoresInvalidTimestamp(t *testing.T) {
	t.Parallel()

	timestampsResponse := `[{"basetime": "../../evil", "validtime": "../../evil", "elements": ["hrpns_nd", "liden"]}]`
	var urls []string

	if _, err := amesh.CreateAmeshImage(t.Context(), &amesh.CreateAmeshImageParams{
		Client: &http.Client{Transport: recordingRoundTrip{
			mu:   &sync.Mutex{},
			urls: &urls,
			base: roundTrip{Config: httpMockConfig{
				TimestampsResponse: timestampsResponse,
				DummyTileBytes:     createWhiteDummyTile(t),
			}},
		}},
		Lat:  35.6895,
		Lng:  139.6917,
		Zoom: 10,
	}); err != nil {
		t.Fatalf("CreateAmeshImage() error = %v", err)
	}

	for _, u := range urls {
		if strings.Contains(u, "evil") {
			t.Errorf("不正なタイムスタンプがURLに埋め込まれた: %s", u)
		}
	}
}

// TestGenerateFileNameRemovesPathSeparators ファイル名にパス区切り文字を含めないことをテストする
func TestGenerateFileNameRemovesPathSeparators(t *testing.T) {
	t.Parallel()

	fileName := amesh.GenerateFileName(&amesh.Location{PlaceName: `../../etc/a\b`})

	if strings.ContainsAny(fileName, `/\`) {
		t.Errorf("GenerateFileName() = %q, パス区切り文字を含む", fileName)
	}
}

// geocodeResponseOfSize 指定した大きさちょうどの正常なジオコーディングのレスポンスを作成する
func geocodeResponseOfSize(size int) string {
	prefix := `{"Feature": [{"Name": "東京都", "Geometry": {"Coordinates": "139.6917,35.6895"}}], "Padding": "`
	const suffix = `"}`

	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}

// TestParseLocationWithClientResponseSizeBoundary レスポンスの大きさが上限ちょうどなら読み込み、
// 1バイトでも超えれば拒否することをテストする
func TestParseLocationWithClientResponseSizeBoundary(t *testing.T) {
	t.Parallel()

	// maxResponseBytes レスポンスの大きさの上限
	const maxResponseBytes = 10 * 1024 * 1024

	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "上限ちょうど", size: maxResponseBytes, wantErr: nil},
		{name: "上限を1バイト超える", size: maxResponseBytes + 1, wantErr: amesh.ErrResponseTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseLocationWithMock(t, "東京", geocodeResponseOfSize(tt.size))
			if tt.wantErr == nil && err != nil {
				t.Errorf("ParseLocationWithClient() error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseLocationWithClient() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// deadlineRoundTrip リクエストのcontextの期限を記録するモック
type deadlineRoundTrip struct {
	mu        *sync.Mutex
	deadlines *[]time.Time
	base      roundTrip
}

func (d deadlineRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()

	d.mu.Lock()

	if ok {
		*d.deadlines = append(*d.deadlines, deadline)
	} else {
		*d.deadlines = append(*d.deadlines, time.Time{})
	}

	d.mu.Unlock()

	return d.base.RoundTrip(req)
}

// TestCreateImageBufferWithClientSetsTimeout 画像作成中のリクエストに2分以内の期限が設定されていることをテストする
// 外部APIが応答しない場合に処理が終わらなくならないようにするため
func TestCreateImageBufferWithClientSetsTimeout(t *testing.T) {
	t.Parallel()

	var deadlines []time.Time

	// 期限のないcontextを渡して正常に画像を作成し、その間のリクエストに期限が付いていることを確認する
	if _, err := amesh.CreateImageBufferWithClient(context.Background(), &amesh.CreateImageBufferWithClientParams{
		Client: &http.Client{Transport: deadlineRoundTrip{
			mu:        &sync.Mutex{},
			deadlines: &deadlines,
			base: roundTrip{Config: httpMockConfig{
				TimestampsResponse: `[{"basetime": "20240101120000", "validtime": "20240101120000", "elements": ["hrpns_nd", "liden"]}]`,
				DummyTileBytes:     createWhiteDummyTile(t),
			}},
		}},
		Location: &amesh.Location{Lat: 35.6895, Lng: 139.6917, PlaceName: "東京都"},
	}); err != nil {
		t.Errorf("CreateImageBufferWithClient() error = %v, want nil", err)
	}

	// 期限は呼び出し中に設定されるため、呼び出し後の時刻から2分後を上限とする
	limit := time.Now().Add(2 * time.Minute)

	if len(deadlines) == 0 {
		t.Fatal("リクエストが送信されていない")
	}

	for _, deadline := range deadlines {
		if deadline.IsZero() {
			t.Error("リクエストのcontextに期限が設定されていない")
			continue
		}

		if deadline.After(limit) {
			t.Errorf("deadline = %v, want <= %v", deadline, limit)
		}
	}
}
