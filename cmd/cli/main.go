package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"hato-bot-go/lib/amesh"
)

// main スタンドアロンモードで実行
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <command> <params>")
		fmt.Println("Commands:")
		fmt.Println("	amesh: Displays amesh, which is rain cloud information")
		fmt.Println("	       Usage: go run main.go amesh <place name>")
		fmt.Println("	       Usage: go run main.go amesh <latitude>,<longitude>")
		fmt.Println("Note: YAHOO_API_TOKEN environment variable must be set")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "amesh":
		if len(os.Args) < 3 {
			fmt.Println("amesh: Displays amesh, which is rain cloud information")
			fmt.Println("Usage: go run main.go amesh <place name>")
			fmt.Println("Usage: go run main.go amesh <latitude>,<longitude>")
			fmt.Println("Note: YAHOO_API_TOKEN environment variable must be set")
			os.Exit(1)
		}

		place := os.Args[2]
		apiKey := os.Getenv("YAHOO_API_TOKEN")

		if apiKey == "" {
			panic(errors.Errorf("Please set YAHOO_API_TOKEN environment variable"))
		}

		ctx := context.Background()

		// 座標が直接提供された場合の解析
		location, err := amesh.ParseLocation(ctx, place, apiKey)
		if err != nil {
			panic(errors.Wrap(err, "Failed to amesh.ParseLocation"))
		}

		fmt.Printf(
			"Generating amesh image for %s (%.4f, %.4f)\n",
			location.PlaceName,
			location.Lat,
			location.Lng,
		)

		// amesh画像をメモリ上に作成
		imageReader, err := amesh.CreateImageReader(ctx, location)
		if err != nil {
			panic(errors.Wrap(err, "Failed to amesh.CreateImageReader"))
		}

		// ファイル名を生成
		fileName := amesh.GenerateFileName(location)
		filePath := filepath.Join(".", fileName)

		// ファイルに保存
		file, err := os.Create(filePath)
		if err != nil {
			panic(errors.Wrap(err, "Failed to os.Create"))
		}
		defer func(file *os.File) {
			if closeErr := file.Close(); closeErr != nil {
				panic(errors.Wrap(closeErr, "Failed to Close"))
			}
		}(file)

		if _, err := io.Copy(file, imageReader); err != nil {
			panic(errors.Wrap(err, "Failed to io.Copy"))
		}

		fmt.Printf("Amesh image saved to %s\n", filePath)
	default:
		panic(errors.Errorf("Unknown command: %s", command))
	}
}
