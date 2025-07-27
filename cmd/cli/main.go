package main

import (
	"fmt"
	"hato-bot-go/lib/amesh"
	"image/png"
	"os"
	"strconv"
	"strings"
)

// main スタンドアロンモードで実行
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <place_name> [yahoo_api_key]")
		fmt.Println("Example: go run main.go 東京 your_api_key")
		os.Exit(1)
	}

	place := os.Args[1]
	apiKey := os.Getenv("YAHOO_API_TOKEN")
	if len(os.Args) > 2 {
		apiKey = os.Args[2]
	}

	if apiKey == "" {
		fmt.Println("Please set YAHOO_API_TOKEN environment variable or provide it as argument")
		os.Exit(1)
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
		result, err := amesh.GeocodePlace(place, apiKey)
		if err != nil {
			fmt.Printf("Error geocoding place: %v\n", err)
			os.Exit(1)
		}
		lat, lng, placeName = result.Lat, result.Lng, result.Name
	}

	fmt.Printf("Generating amesh image for %s (%.4f, %.4f)\n", placeName, lat, lng)

	// amesh画像を作成
	img, err := amesh.CreateAmeshImage(lat, lng, 10, 2)
	if err != nil {
		fmt.Printf("Error creating amesh image: %v\n", err)
		os.Exit(1)
	}

	// 画像を保存
	filename := "amesh_" + strings.ReplaceAll(placeName, " ", "_") + ".png"
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}(file)

	if err = png.Encode(file, img); err != nil {
		fmt.Printf("Error encoding image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Amesh image saved to %s\n", filename)
}
