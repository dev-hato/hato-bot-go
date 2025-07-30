package amesh

import (
	"encoding/json"
	"fmt"
	libHttp "hato-bot-go/lib/http"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// FileWriter はファイル書き込み操作を行うインターフェース
type FileWriter interface {
	Create(name string) (io.WriteCloser, error)
}

// OSFileWriter は実際のファイルシステムを使用するFileWriterの実装
type OSFileWriter struct{}

// Create 実際のファイルを作成する
func (w *OSFileWriter) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

// CreateImageRequest レーダー画像作成のリクエスト構造体
type CreateImageRequest struct {
	Lat         float64 // 緯度
	Lng         float64 // 経度
	Zoom        int     // ズームレベル
	AroundTiles int     // 周囲のタイル数
}

// GeocodeRequest ジオコーディングのリクエスト構造体
type GeocodeRequest struct {
	Place  string // 地名
	APIKey string // APIキー
}

// Location 位置情報の構造体
type Location struct {
	Lat       float64 // 緯度
	Lng       float64 // 経度
	PlaceName string  // 地名
}

// CreateAndSaveImageRequest 画像作成・保存のリクエスト構造体
type CreateAndSaveImageRequest struct {
	Client   libHttp.Client // HTTPクライアント
	Writer   FileWriter     // ファイルライター
	Location *Location      // 位置情報
	BasePath string         // 保存先ベースパス
}

// ParseLocationRequest 位置解析のリクエスト構造体
type ParseLocationRequest struct {
	Client         libHttp.Client // HTTPクライアント
	GeocodeRequest GeocodeRequest
}

type drawDistanceCircleParams struct {
	Img                *image.RGBA
	CreateImageRequest *CreateImageRequest
	RadiusKm           float64
	Col                color.RGBA
}

type drawLineParams struct {
	Img *image.RGBA
	X1  int
	Y1  int
	X2  int
	Y2  int
	Col color.RGBA
}

type drawLightningMarkerParams struct {
	Img                *image.RGBA
	Lightning          lightningPoint
	CreateImageRequest *CreateImageRequest
}

// httpRequestResult HTTPリクエストの結果を表す構造体
type httpRequestResult struct {
	Body    []byte
	IsEmpty bool
}

// timeJSONElement targetTimes JSON要素の構造体
type timeJSONElement struct {
	BaseTime  string   `json:"basetime"`
	ValidTime string   `json:"validtime"`
	Elements  []string `json:"elements"`
}

// lightningPoint 落雷データを表す構造体
type lightningPoint struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Type int     `json:"type"`
}

const Version = "1.0"

// エラー定数
var (
	ErrGeocodingAPIError        = errors.New("geocoding API returned error status")
	ErrNoResultsFound           = errors.New("no results found for place")
	ErrInvalidCoordinatesFormat = errors.New("invalid coordinates format")
	ErrJSONUnmarshal            = errors.New("failed to json.Unmarshal")
)

// defaultHTTPClient はデフォルトのHTTPクライアント
var defaultHTTPClient = libHttp.DefaultHTTPClient

// CreateAmeshImageWithClient HTTPクライアントを指定してameshレーダー画像を作成する
func CreateAmeshImageWithClient(client libHttp.Client, req *CreateImageRequest) (*image.RGBA, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	// 最新のタイムスタンプを取得
	timestamps := getLatestTimestampsWithClient(client)

	hrpnsTimestamp := timestamps["hrpns_nd"]
	lidenTimestamp := timestamps["liden"]

	// 落雷データを取得
	lightningData, err := getLightningDataWithClient(client, lidenTimestamp)
	if err != nil {
		log.Printf("落雷データの取得に失敗: %v", err)
		lightningData = []lightningPoint{}
	}

	// ピクセル座標を計算
	centerX, centerY := getWebMercatorPixel(req)
	centerTileX, centerTileY := int(centerX/256), int(centerY/256)

	// ベース画像を作成
	imageSize := (2*req.AroundTiles + 1) * 256
	img := image.NewRGBA(image.Rect(0, 0, imageSize, imageSize))

	// 白い背景で塗りつぶし
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	// タイルをダウンロードして合成
	for dy := -req.AroundTiles; dy <= req.AroundTiles; dy++ {
		for dx := -req.AroundTiles; dx <= req.AroundTiles; dx++ {
			tileX := centerTileX + dx
			tileY := centerTileY + dy

			// ベースマップタイル（OpenStreetMap）をダウンロード
			baseURL := fmt.Sprintf("https://tile.openstreetmap.org/%d/%d/%d.png", req.Zoom, tileX, tileY)

			baseTile, err := downloadTileWithClient(client, baseURL)
			if err != nil {
				log.Printf("Failed to downloadTileWithClient: %v", err)
				continue
			}

			// ベースタイルを描画
			destRect := image.Rect(
				(dx+req.AroundTiles)*256,
				(dy+req.AroundTiles)*256,
				(dx+req.AroundTiles+1)*256,
				(dy+req.AroundTiles+1)*256,
			)
			draw.Draw(img, destRect, baseTile, image.Point{}, draw.Over)

			// レーダータイルをダウンロードしてオーバーレイ
			radarURL := fmt.Sprintf("https://www.jma.go.jp/bosai/jmatile/data/nowc/%s/none/%s/surf/hrpns/%d/%d/%d.png", hrpnsTimestamp, hrpnsTimestamp, req.Zoom, tileX, tileY)
			radarTile, err := downloadTileWithClient(client, radarURL)
			if err != nil {
				log.Printf("Failed to downloadTileWithClient: %v", err)
				continue
			}

			// レーダータイルを透明度付きで描画
			draw.DrawMask(img, destRect, radarTile, image.Point{}, &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 128}}, image.Point{}, draw.Over)
		}
	}

	// 距離円を描画
	for d := 10; d <= 50; d += 10 {
		drawDistanceCircle(
			&drawDistanceCircleParams{
				Img:                img,
				CreateImageRequest: req,
				RadiusKm:           float64(d),
				Col:                color.RGBA{R: 100, G: 100, B: 100, A: 255},
			})
	}

	// 落雷マーカーを描画
	for _, lightning := range lightningData {
		drawLightningMarker(&drawLightningMarkerParams{
			Img:                img,
			Lightning:          lightning,
			CreateImageRequest: req,
		})
	}

	return img, nil
}

// CreateAndSaveImageWithClient HTTPクライアントとファイルライターを指定してamesh画像を作成してファイルに保存する
func CreateAndSaveImageWithClient(req *CreateAndSaveImageRequest) (string, error) {
	if req == nil {
		return "", errors.New("req cannot be nil")
	}
	if req.Client == nil {
		return "", errors.New("client cannot be nil")
	}
	if req.Writer == nil {
		return "", errors.New("writer cannot be nil")
	}
	if req.Location == nil {
		return "", errors.New("location cannot be nil")
	}
	img, err := CreateAmeshImageWithClient(req.Client, &CreateImageRequest{
		Lat:         req.Location.Lat,
		Lng:         req.Location.Lng,
		Zoom:        10,
		AroundTiles: 2,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to CreateAmeshImageWithClient")
	}

	fileName := fmt.Sprintf(
		"amesh_%s_%d.png",
		strings.ReplaceAll(req.Location.PlaceName, " ", "_"),
		time.Now().Unix(),
	)
	filePath := filepath.Join(req.BasePath, fileName)

	file, err := req.Writer.Create(filePath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to writer.Create")
	}
	defer func(file io.WriteCloser) {
		if closeErr := file.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(file)

	if err := png.Encode(file, img); err != nil {
		return "", errors.Wrap(err, "Failed to png.Encode")
	}

	return filePath, nil
}

// CreateAndSaveImage amesh画像を作成してファイルに保存する
func CreateAndSaveImage(location *Location, basePath string) (string, error) {
	return CreateAndSaveImageWithClient(&CreateAndSaveImageRequest{
		Client:   defaultHTTPClient,
		Writer:   &OSFileWriter{},
		Location: location,
		BasePath: basePath,
	})
}

// GeocodeWithClient HTTPクライアントを指定して地名を座標に変換する
func GeocodeWithClient(client libHttp.Client, req *GeocodeRequest) (Location, error) {
	if req == nil {
		return Location{}, errors.New("req cannot be nil")
	}

	place := req.Place
	if place == "" {
		place = "東京"
	}

	requestURL := fmt.Sprintf("https://map.yahooapis.jp/geocode/V1/geoCoder?appid=%s&query=%s&output=json", req.APIKey, url.QueryEscape(place))

	resp, err := client.Get(requestURL)
	if err != nil {
		return Location{}, errors.Wrap(err, "Failed to Get")
	}

	if resp.StatusCode != 200 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return Location{}, errors.Wrap(closeErr, "Failed to Close")
		}
		return Location{}, errors.Wrapf(ErrGeocodingAPIError, "ステータス %d", resp.StatusCode)
	}

	body, err := handleHTTPResponse(resp)
	if err != nil {
		return Location{}, errors.Wrap(err, "Failed to handleHTTPResponse")
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
		return Location{}, errors.Wrap(ErrJSONUnmarshal, unmarshalErr.Error())
	}

	if len(result.Feature) == 0 {
		return Location{}, errors.Wrapf(ErrNoResultsFound, "%s", place)
	}

	feature := result.Feature[0]
	coords := strings.Split(feature.Geometry.Coordinates, ",")
	if len(coords) < 2 {
		return Location{}, ErrInvalidCoordinatesFormat
	}

	lng, err := strconv.ParseFloat(coords[0], 64)
	if err != nil {
		return Location{}, errors.Wrap(err, "Failed to strconv.ParseFloat")
	}

	lat, err := strconv.ParseFloat(coords[1], 64)
	if err != nil {
		return Location{}, errors.Wrap(err, "Failed to strconv.ParseFloat")
	}

	return Location{
		Lat:       lat,
		Lng:       lng,
		PlaceName: feature.Name,
	}, nil
}

// ParseLocationWithClient HTTPクライアントを指定して地名文字列から位置を解析し、Location構造体とエラーを返す
func ParseLocationWithClient(req *ParseLocationRequest) (*Location, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	if req.Client == nil {
		return nil, errors.New("client cannot be nil")
	}
	// 座標が直接提供されているかチェック
	parts := strings.Fields(req.GeocodeRequest.Place)
	if len(parts) == 2 {
		if parsedLat, err1 := parseFloat64(parts[0]); err1 == nil {
			if parsedLng, err2 := parseFloat64(parts[1]); err2 == nil {
				return &Location{
					Lat:       parsedLat,
					Lng:       parsedLng,
					PlaceName: fmt.Sprintf("%.2f,%.2f", parsedLat, parsedLng),
				}, nil
			}
		}
	}

	// 地名をジオコーディング
	result, geocodeErr := GeocodeWithClient(req.Client, &req.GeocodeRequest)
	if geocodeErr != nil {
		return nil, errors.Wrap(geocodeErr, "Failed to GeocodeWithClient")
	}
	return &result, nil
}

// ParseLocation 地名文字列から位置を解析し、Location構造体とエラーを返す
func ParseLocation(place, apiKey string) (*Location, error) {
	return ParseLocationWithClient(&ParseLocationRequest{
		Client: defaultHTTPClient,
		GeocodeRequest: GeocodeRequest{
			Place:  place,
			APIKey: apiKey,
		},
	})
}

// handleHTTPResponse HTTPレスポンスの共通処理を行う
func handleHTTPResponse(resp *http.Response) ([]byte, error) {
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to io.ReadAll")
	}
	return body, nil
}

// makeHTTPRequest HTTPリクエストを送信し、非200ステータスコードの場合は空を返す
func makeHTTPRequest(client libHttp.Client, url string) (*httpRequestResult, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Get")
	}

	if resp.StatusCode != 200 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, errors.Wrap(closeErr, "Failed to Close")
		}
		return &httpRequestResult{Body: nil, IsEmpty: true}, nil
	}

	body, err := handleHTTPResponse(resp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to handleHTTPResponse")
	}

	return &httpRequestResult{Body: body, IsEmpty: false}, nil
}

// fetchTimeDataFromURLWithClient HTTPクライアントを指定してタイムデータを取得する
func fetchTimeDataFromURLWithClient(client libHttp.Client, apiURL string) ([]timeJSONElement, error) {
	body, err := makeHTTPRequest(client, apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to makeHTTPRequest")
	}
	if body.Body == nil {
		return nil, errors.New("Body is nil")
	}

	var timeData []timeJSONElement
	if err := json.Unmarshal(body.Body, &timeData); err != nil {
		return nil, errors.Wrap(err, "Failed to json.Unmarshal")
	}

	return timeData, nil
}

// getLatestTimestampsWithClient HTTPクライアントを指定して最新のタイムスタンプを取得する
func getLatestTimestampsWithClient(client libHttp.Client) map[string]string {
	urls := []string{
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N1.json",
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N2.json",
		"https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N3.json",
	}

	var allTimeData []timeJSONElement

	for _, apiURL := range urls {
		timeData, err := fetchTimeDataFromURLWithClient(client, apiURL)
		if err != nil {
			log.Printf("Failed to fetchTimeDataFromURLWithClient: %v", err)
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
				if e == element && latestTime < td.BaseTime {
					latestTime = td.BaseTime
				}
			}
		}
		result[element] = latestTime
	}

	return result
}

// getLightningDataWithClient HTTPクライアントを指定して落雷データを取得する
func getLightningDataWithClient(client libHttp.Client, timestamp string) ([]lightningPoint, error) {
	apiURL := fmt.Sprintf("https://www.jma.go.jp/bosai/jmatile/data/nowc/%s/none/%s/surf/liden/data.geojson", timestamp, timestamp)

	result, err := makeHTTPRequest(client, apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to makeHTTPRequest")
	}
	if result.IsEmpty {
		return []lightningPoint{}, nil
	}

	var geoJSON struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Type int `json:"type"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(result.Body, &geoJSON); err != nil {
		return nil, errors.Wrap(err, "Failed to json.Unmarshal")
	}

	var lightningPoints []lightningPoint
	for _, feature := range geoJSON.Features {
		if len(feature.Geometry.Coordinates) < 2 {
			continue
		}
		lightningPoints = append(lightningPoints, lightningPoint{
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

// getWebMercatorPixel 地理座標をWebメルカトル投影でピクセル座標に変換
// - 地理座標（度数）をピクセル座標に変換
// - ズームレベルに応じたスケール調整
// - 地図タイルの標準的な座標系を使用
func getWebMercatorPixel(params *CreateImageRequest) (float64, float64) {
	if params.Zoom < 0 || 30 < params.Zoom {
		return 0, 0
	}
	zoomFactor := float64(int(1) << uint(params.Zoom))
	x := 256.0 * zoomFactor * (params.Lng + 180) / 360.0
	y := 256.0 * zoomFactor * (0.5 - math.Log(math.Tan(math.Pi/4+deg2rad(params.Lat)/2))/(2.0*math.Pi))
	return x, y
}

// downloadTileWithClient HTTPクライアントを指定してマップタイルをダウンロードする
func downloadTileWithClient(client libHttp.Client, tileURL string) (image.Image, error) {
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to libHttp.NewRequest")
	}
	req.Header.Set("User-Agent", "hato-bot-go/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
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
// 64個の線分で円を近似し、地球の曲率を考慮した地理的距離円を描画
func drawDistanceCircle(params *drawDistanceCircleParams) {
	// 線分で円を近似
	numSegments := 64
	earthRadius := 6371.0 // 地球半径（キロメートル）

	for i := 0; i < numSegments; i++ {
		angle1 := float64(i) * 2 * math.Pi / float64(numSegments)
		angle2 := float64(i+1) * 2 * math.Pi / float64(numSegments)

		// 円上の点を計算（地球の曲率を考慮）
		lat1 := params.CreateImageRequest.Lat + (params.RadiusKm/earthRadius)*math.Cos(angle1)*180/math.Pi
		lng1 := params.CreateImageRequest.Lng + (params.RadiusKm/earthRadius)*math.Sin(angle1)*180/math.Pi/math.Cos(deg2rad(params.CreateImageRequest.Lat))

		lat2 := params.CreateImageRequest.Lat + (params.RadiusKm/earthRadius)*math.Cos(angle2)*180/math.Pi
		lng2 := params.CreateImageRequest.Lng + (params.RadiusKm/earthRadius)*math.Sin(angle2)*180/math.Pi/math.Cos(deg2rad(params.CreateImageRequest.Lat))

		// ピクセル座標に変換
		x1, y1 := getWebMercatorPixel(&CreateImageRequest{
			Lat:  lat1,
			Lng:  lng1,
			Zoom: params.CreateImageRequest.Zoom,
		})
		x2, y2 := getWebMercatorPixel(&CreateImageRequest{
			Lat:  lat2,
			Lng:  lng2,
			Zoom: params.CreateImageRequest.Zoom,
		})

		// 画像座標に変換
		centerX, centerY := getWebMercatorPixel(params.CreateImageRequest)
		imageSize := (2*params.CreateImageRequest.AroundTiles + 1) * 256

		imgX1 := int(x1 - centerX + float64(imageSize/2))
		imgY1 := int(y1 - centerY + float64(imageSize/2))
		imgX2 := int(x2 - centerX + float64(imageSize/2))
		imgY2 := int(y2 - centerY + float64(imageSize/2))

		// 線分を描画
		drawLine(&drawLineParams{
			Img: params.Img,
			X1:  imgX1,
			Y1:  imgY1,
			X2:  imgX2,
			Y2:  imgY2,
			Col: params.Col,
		})
	}
}

// drawLine 二点間に直線を描画する
// ブレゼンハムアルゴリズム使用
func drawLine(params *drawLineParams) {
	// シンプルな直線描画アルゴリズム
	dx := abs(params.X2 - params.X1)
	dy := abs(params.Y2 - params.Y1)
	sx := 1
	sy := 1

	if params.X2 < params.X1 {
		sx = -1
	}
	if params.Y2 < params.Y1 {
		sy = -1
	}

	delta := dx - dy
	x, y := params.X1, params.Y1

	for {
		if 0 <= x && 0 <= y && x < params.Img.Bounds().Dx() && y < params.Img.Bounds().Dy() {
			params.Img.Set(x, y, params.Col)
		}

		if x == params.X2 && y == params.Y2 {
			break
		}

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
}

// abs 整数の絶対値を返す
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// drawLightningMarker 画像上に落雷マーカーを描画する
// 円形塗りつぶしアルゴリズム使用
func drawLightningMarker(params *drawLightningMarkerParams) {
	// ピクセル座標に変換
	x, y := getWebMercatorPixel(&CreateImageRequest{
		Lat:  params.Lightning.Lat,
		Lng:  params.Lightning.Lng,
		Zoom: params.CreateImageRequest.Zoom,
	})
	centerX, centerY := getWebMercatorPixel(params.CreateImageRequest)

	// 画像座標に変換
	imageSize := (2*params.CreateImageRequest.AroundTiles + 1) * 256
	imgX := int(x - centerX + float64(imageSize/2))
	imgY := int(y - centerY + float64(imageSize/2))

	// 落雷記号を描画（シンプルな円）
	radius := 7
	lightningColor := color.RGBA{G: 255, B: 255, A: 255}

	// ピタゴラスの定理による円内判定
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if radius*radius < dx*dx+dy*dy {
				continue
			}
			x := imgX + dx
			y := imgY + dy
			if 0 <= x && 0 <= y && x < params.Img.Bounds().Dx() && y < params.Img.Bounds().Dy() {
				params.Img.Set(x, y, lightningColor)
			}
		}
	}
}

// parseFloat64 文字列をfloat64に変換
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
