package main

import (
	"fmt"
	"hato-bot-go/lib/amesh"
	"os"

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
	location, err := amesh.ParseLocation(place, apiKey)
	if err != nil {
		panic(errors.Wrap(err, "Failed to amesh.ParseLocation"))
	}

	fmt.Printf("Generating amesh image for %s (%.4f, %.4f)\n", location.PlaceName, location.Lat, location.Lng)

	// amesh画像を作成・保存
	filename, err := amesh.CreateAndSaveImage(location, ".")
	if err != nil {
		panic(errors.Wrap(err, "Failed to amesh.CreateAndSaveImage"))
	}

	fmt.Printf("Amesh image saved to %s\n", filename)
}
