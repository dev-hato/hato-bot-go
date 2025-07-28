package main

import (
	"fmt"
	"hato-bot-go/lib/amesh"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// main スタンドアロンモードで実行
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <place_name>")
		fmt.Println("Example: go run main.go 東京")
		fmt.Println("Note: YAHOO_API_TOKEN environment variable must be set")
		os.Exit(1)
	}

	place := os.Args[1]
	apiKey := os.Getenv("YAHOO_API_TOKEN")

	if apiKey == "" {
		panic(errors.Errorf("Please set YAHOO_API_TOKEN environment variable"))
	}

	// 座標が直接提供された場合の解析
	parts := strings.Fields(place)
	var lat, lng float64
	var placeName string
	var err error

	if len(parts) == 2 {
		// 座標として解析を試行
		lat, err = strconv.ParseFloat(parts[0], 64)
		if err == nil {
			lng, err = strconv.ParseFloat(parts[1], 64)
			if err == nil {
				placeName = fmt.Sprintf("%.2f,%.2f", lat, lng)
			}
		}
	}

	if err != nil || placeName == "" {
		// 地名をジオコーディング
		result, geocodeErr := amesh.GeocodePlace(place, apiKey)
		if geocodeErr != nil {
			panic(errors.Wrap(geocodeErr, "Failed to amesh.GeocodePlace"))
		}
		lat, lng, placeName = result.Lat, result.Lng, result.Name
	}

	fmt.Printf("Generating amesh image for %s (%.4f, %.4f)\n", placeName, lat, lng)

	// amesh画像を作成・保存
	filename, err := amesh.CreateAndSaveImage(&amesh.Location{
		Lat:       lat,
		Lng:       lng,
		PlaceName: placeName,
	}, ".")
	if err != nil {
		panic(errors.Wrap(err, "Failed to amesh.CreateAndSaveImage"))
	}

	fmt.Printf("Amesh image saved to %s\n", filename)
}
