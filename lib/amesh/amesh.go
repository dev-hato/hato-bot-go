package amesh

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// HTTPClient はHTTPリクエストを行うインターフェース
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient はデフォルトのHTTPクライアント
var DefaultHTTPClient HTTPClient = &http.Client{}

// GeocodeResult ジオコーディングの結果を表す構造体
type GeocodeResult struct {
	Lat  float64
	Lng  float64
	Name string
}

const Version = "1.0"

// handleHTTPResponse HTTPレスポンスの共通処理を行う
func handleHTTPResponse(resp *http.Response) ([]byte, error) {
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to io.ReadAll")
	}
	return body, nil
}

// makeHTTPRequest HTTPリクエストを送信し、レスポンスボディを取得する共通処理
func makeHTTPRequest(client HTTPClient, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Get")
	}

	if resp.StatusCode != 200 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, errors.Wrap(closeErr, "Failed to Close")
		}
		return nil, fmt.Errorf("ステータスコード: %d", resp.StatusCode)
	}

	body, err := handleHTTPResponse(resp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to handleHTTPResponse")
	}

	return body, nil
}

// HTTPRequestResult HTTPリクエストの結果を表す構造体
type HTTPRequestResult struct {
	Body    []byte
	IsEmpty bool
}

// makeHTTPRequestAllowEmpty HTTPリクエストを送信し、非200ステータスコードの場合は空を返す
func makeHTTPRequestAllowEmpty(client HTTPClient, url string) (*HTTPRequestResult, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Get")
	}

	if resp.StatusCode != 200 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, errors.Wrap(closeErr, "Failed to Close")
		}
		return &HTTPRequestResult{Body: nil, IsEmpty: true}, nil
	}

	body, err := handleHTTPResponse(resp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to handleHTTPResponse")
	}

	return &HTTPRequestResult{Body: body, IsEmpty: false}, nil
}

// エラー定数
var (
	ErrGeocodingAPIError        = errors.New("geocoding API returned error status")
	ErrNoResultsFound           = errors.New("no results found for place")
	ErrInvalidCoordinatesFormat = errors.New("invalid coordinates format")
	ErrJSONUnmarshal            = errors.New("failed to json.Unmarshal")
)

// TimeJSONElement targetTimes JSON要素の構造体
type TimeJSONElement struct {
	BaseTime  string   `json:"basetime"`
	ValidTime string   `json:"validtime"`
	Elements  []string `json:"elements"`
}

// LightningPoint 落雷データを表す構造体
type LightningPoint struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Type int     `json:"type"`
}

// LightningGeoJSON 落雷データのGeoJSON構造体
type LightningGeoJSON struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Type int `json:"type"`
		} `json:"properties"`
	} `json:"features"`
}

// CreateAmeshImageWithClient HTTPクライアントを指定してameshレーダー画像を作成する
func CreateAmeshImageWithClient(client HTTPClient, lat, lng float64, zoom, aroundTiles int) (*image.RGBA, error) {
	// 最新のタイムスタンプを取得
	timestamps, err := getLatestTimestampsWithClient(client)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to getLatestTimestampsWithClient")
	}

	hrpnsTimestamp := timestamps["hrpns_nd"]
	lidenTimestamp := timestamps["liden"]

	// 落雷データを取得
	lightningData, err := getLightningDataWithClient(client, lidenTimestamp)
	if err != nil {
		log.Printf("落雷データの取得に失敗: %v", err)
		lightningData = []LightningPoint{}
	}

	// ピクセル座標を計算
	centerX, centerY := getWebMercatorPixel(lat, lng, zoom)
	centerTileX, centerTileY := getTileFromPixel(centerX, centerY)

	// ベース画像を作成
	imageSize := (2*aroundTiles + 1) * 256
	img := image.NewRGBA(image.Rect(0, 0, imageSize, imageSize))

	// 白い背景で塗りつぶし
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	// タイルをダウンロードして合成
	for dy := -aroundTiles; dy <= aroundTiles; dy++ {
		for dx := -aroundTiles; dx <= aroundTiles; dx++ {
			tileX := centerTileX + dx
			tileY := centerTileY + dy

			// ベースマップタイル（OpenStreetMap）をダウンロード
			baseURL := fmt.Sprintf("https://tile.openstreetmap.org/%d/%d/%d.png", zoom, tileX, tileY)

			baseTile, err := downloadTileWithClient(client, baseURL)
			if err != nil {
				log.Printf("ベースタイルのダウンロードに失敗: %v", err)
				continue
			}

			// ベースタイルを描画
			destRect := image.Rect(
				(dx+aroundTiles)*256,
				(dy+aroundTiles)*256,
				(dx+aroundTiles+1)*256,
				(dy+aroundTiles+1)*256,
			)
			draw.Draw(img, destRect, baseTile, image.Point{}, draw.Over)

			// レーダータイルをダウンロードしてオーバーレイ
			radarURL := fmt.Sprintf("https://www.jma.go.jp/bosai/jmatile/data/nowc/%s/none/%s/surf/hrpns/%d/%d/%d.png", hrpnsTimestamp, hrpnsTimestamp, zoom, tileX, tileY)
			radarTile, err := downloadTileWithClient(client, radarURL)
			if err != nil {
				log.Printf("レーダータイルのダウンロードに失敗: %v", err)
				continue
			}

			// レーダータイルを透明度付きで描画
			draw.DrawMask(img, destRect, radarTile, image.Point{}, &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 128}}, image.Point{}, draw.Over)
		}
	}

	// 距離円を描画
	for d := 10; d <= 50; d += 10 {
		drawDistanceCircle(img, lat, lng, float64(d), zoom, aroundTiles, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	}

	// 落雷マーカーを描画
	for _, lightning := range lightningData {
		drawLightningMarker(img, lightning.Lat, lightning.Lng, lat, lng, zoom, aroundTiles)
	}

	return img, nil
}

// CreateAmeshImage ameshレーダー画像を作成する
func CreateAmeshImage(lat, lng float64, zoom, aroundTiles int) (*image.RGBA, error) {
	return CreateAmeshImageWithClient(DefaultHTTPClient, lat, lng, zoom, aroundTiles)
}

// GeocodeWithClient HTTPクライアントを指定して地名を座標に変換する
func GeocodeWithClient(client HTTPClient, place, apiKey string) (GeocodeResult, error) {
	if place == "" {
		place = "東京"
	}

	requestURL := fmt.Sprintf("https://map.yahooapis.jp/geocode/V1/geoCoder?appid=%s&query=%s&output=json", apiKey, url.QueryEscape(place))

	resp, err := client.Get(requestURL)
	if err != nil {
		return GeocodeResult{}, errors.Wrap(err, "Failed to Get")
	}

	if resp.StatusCode != 200 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return GeocodeResult{}, errors.Wrap(closeErr, "Failed to Close")
		}
		return GeocodeResult{}, errors.Wrapf(ErrGeocodingAPIError, "ステータス %d", resp.StatusCode)
	}

	body, err := handleHTTPResponse(resp)
	if err != nil {
		return GeocodeResult{}, errors.Wrap(err, "Failed to handleHTTPResponse")
	}

	var result struct {
		Feature []struct {
			Name     string `json:"Name"`
			Geometry struct {
				Coordinates string `json:"Coordinates"`
			} `json:"Geometry"`
		} `json:"Feature"`
	}

	if unmarshalErr := json.Unmarshal(body, &result); unmarshalErr != nil {
		return GeocodeResult{}, errors.Wrap(ErrJSONUnmarshal, unmarshalErr.Error())
	}

	if len(result.Feature) == 0 {
		return GeocodeResult{}, errors.Wrapf(ErrNoResultsFound, "%s", place)
	}

	feature := result.Feature[0]
	coords := strings.Split(feature.Geometry.Coordinates, ",")
	if len(coords) < 2 {
		return GeocodeResult{}, ErrInvalidCoordinatesFormat
	}

	lng, err := strconv.ParseFloat(coords[0], 64)
	if err != nil {
		return GeocodeResult{}, errors.Wrap(err, "Failed to strconv.ParseFloat")
	}

	lat, err := strconv.ParseFloat(coords[1], 64)
	if err != nil {
		return GeocodeResult{}, errors.Wrap(err, "Failed to strconv.ParseFloat")
	}

	return GeocodeResult{
		Lat:  lat,
		Lng:  lng,
		Name: feature.Name,
	}, nil
}

// GeocodePlace Yahoo APIを使用して地名を座標に変換する
func GeocodePlace(place, apiKey string) (GeocodeResult, error) {
	return GeocodeWithClient(DefaultHTTPClient, place, apiKey)
}

// fetchTimeDataFromURLWithClient HTTPクライアントを指定してタイムデータを取得する
func fetchTimeDataFromURLWithClient(client HTTPClient, apiURL string) ([]TimeJSONElement, error) {
	body, err := makeHTTPRequest(client, apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to makeHTTPRequest")
	}

	var timeData []TimeJSONElement
	if err := json.Unmarshal(body, &timeData); err != nil {
		return nil, errors.Wrap(err, "Failed to json.Unmarshal")
	}

	return timeData, nil
}

// getLatestTimestampsWithClient HTTPクライアントを指定して最新のタイムスタンプを取得する
func getLatestTimestampsWithClient(client HTTPClient) (map[string]string, error) {
	urls := []string{
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N1.json",
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N2.json",
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N3.json",
	}

	var allTimeData []TimeJSONElement

	for _, apiURL := range urls {
		timeData, err := fetchTimeDataFromURLWithClient(client, apiURL)
		if err != nil {
			continue
		}
		allTimeData = append(allTimeData, timeData...)
	}

	// 一意な要素を抽出
	elementMap := make(map[string]bool)
	for _, td := range allTimeData {
		for _, element := range td.Elements {
			elementMap[element] = true
		}
	}

	// 各要素の最新タイムスタンプを検索
	result := make(map[string]string)
	for element := range elementMap {
		latestTime := ""
		for _, td := range allTimeData {
			if td.BaseTime != td.ValidTime {
				continue
			}
			for _, e := range td.Elements {
				if e == element && td.BaseTime > latestTime {
					latestTime = td.BaseTime
				}
			}
		}
		result[element] = latestTime
	}

	return result, nil
}

// getLightningDataWithClient HTTPクライアントを指定して落雷データを取得する
func getLightningDataWithClient(client HTTPClient, timestamp string) ([]LightningPoint, error) {
	apiURL := fmt.Sprintf("https://www.jma.go.jp/bosai/jmatile/data/nowc/%s/none/%s/surf/liden/data.geojson", timestamp, timestamp)

	result, err := makeHTTPRequestAllowEmpty(client, apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to makeHTTPRequestAllowEmpty")
	}
	if result.IsEmpty {
		return []LightningPoint{}, nil
	}

	var geoJSON LightningGeoJSON
	if err := json.Unmarshal(result.Body, &geoJSON); err != nil {
		return nil, errors.Wrap(err, "Failed to json.Unmarshal")
	}

	var lightningPoints []LightningPoint
	for _, feature := range geoJSON.Features {
		if len(feature.Geometry.Coordinates) < 2 {
			continue
		}
		lightningPoints = append(lightningPoints, LightningPoint{
			Lat:  feature.Geometry.Coordinates[1],
			Lng:  feature.Geometry.Coordinates[0],
			Type: feature.Properties.Type,
		})
	}

	return lightningPoints, nil
}

// deg2rad 度数をラジアンに変換する
func deg2rad(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// getWebMercatorPixel 地理座標をWebメルカトルピクセル座標に変換する
func getWebMercatorPixel(lat, lng float64, zoom int) (float64, float64) {
	if zoom < 0 || zoom > 30 {
		return 0, 0
	}
	zoomFactor := float64(int(1) << uint(zoom))
	x := 256.0 * zoomFactor * (lng + 180) / 360.0
	y := 256.0 * zoomFactor * (0.5 - math.Log(math.Tan(math.Pi/4+deg2rad(lat)/2))/(2.0*math.Pi))
	return x, y
}

// getTileFromPixel ピクセル座標をタイル座標に変換する
func getTileFromPixel(pixelX, pixelY float64) (int, int) {
	return int(pixelX / 256), int(pixelY / 256)
}

// downloadTileWithClient HTTPクライアントを指定してマップタイルをダウンロードする
func downloadTileWithClient(client HTTPClient, tileURL string) (image.Image, error) {
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequest")
	}
	req.Header.Set("User-Agent", "hato-bot-go/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("タイルのダウンロードに失敗: ステータス %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to image.Decode")
	}
	return img, nil
}

// drawDistanceCircle 画像上に距離円を描画する
func drawDistanceCircle(img *image.RGBA, centerLat, centerLng, radiusKm float64, zoom, aroundTiles int, col color.RGBA) {
	// 線分で円を近似
	numSegments := 64
	earthRadius := 6371.0 // 地球半径（キロメートル）

	for i := 0; i < numSegments; i++ {
		angle1 := float64(i) * 2 * math.Pi / float64(numSegments)
		angle2 := float64(i+1) * 2 * math.Pi / float64(numSegments)

		// 円上の点を計算
		lat1 := centerLat + (radiusKm/earthRadius)*math.Cos(angle1)*180/math.Pi
		lng1 := centerLng + (radiusKm/earthRadius)*math.Sin(angle1)*180/math.Pi/math.Cos(deg2rad(centerLat))

		lat2 := centerLat + (radiusKm/earthRadius)*math.Cos(angle2)*180/math.Pi
		lng2 := centerLng + (radiusKm/earthRadius)*math.Sin(angle2)*180/math.Pi/math.Cos(deg2rad(centerLat))

		// ピクセル座標に変換
		x1, y1 := getWebMercatorPixel(lat1, lng1, zoom)
		x2, y2 := getWebMercatorPixel(lat2, lng2, zoom)

		// 画像座標に変換
		centerX, centerY := getWebMercatorPixel(centerLat, centerLng, zoom)
		imageSize := (2*aroundTiles + 1) * 256

		imgX1 := int(x1 - centerX + float64(imageSize/2))
		imgY1 := int(y1 - centerY + float64(imageSize/2))
		imgX2 := int(x2 - centerX + float64(imageSize/2))
		imgY2 := int(y2 - centerY + float64(imageSize/2))

		// 線分を描画
		drawLine(img, imgX1, imgY1, imgX2, imgY2, col)
	}
}

// drawLine 二点間に直線を描画する
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA) {
	// シンプルな直線描画アルゴリズム
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	sy := 1

	if x1 > x2 {
		sx = -1
	}
	if y1 > y2 {
		sy = -1
	}

	delta := dx - dy
	x, y := x1, y1

	for {
		if x >= 0 && y >= 0 && x < img.Bounds().Dx() && y < img.Bounds().Dy() {
			img.Set(x, y, col)
		}

		if x == x2 && y == y2 {
			break
		}

		d2 := 2 * delta
		if d2 > -dy {
			delta -= dy
			x += sx
		}
		if d2 < dx {
			delta += dx
			y += sy
		}
	}
}

// abs 整数の絶対値を返す
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// drawLightningMarker 画像上に落雷マーカーを描画する
func drawLightningMarker(img *image.RGBA, lightningLat, lightningLng, centerLat, centerLng float64, zoom, aroundTiles int) {
	// ピクセル座標に変換
	x, y := getWebMercatorPixel(lightningLat, lightningLng, zoom)
	centerX, centerY := getWebMercatorPixel(centerLat, centerLng, zoom)

	// 画像座標に変換
	imageSize := (2*aroundTiles + 1) * 256
	imgX := int(x - centerX + float64(imageSize/2))
	imgY := int(y - centerY + float64(imageSize/2))

	// 落雷記号を描画（シンプルな円）
	radius := 7
	lightningColor := color.RGBA{G: 255, B: 255, A: 255}

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy > radius*radius {
				continue
			}
			x := imgX + dx
			y := imgY + dy
			if x >= 0 && y >= 0 && x < img.Bounds().Dx() && y < img.Bounds().Dy() {
				img.Set(x, y, lightningColor)
			}
		}
	}
}
