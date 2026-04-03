package main

import (
	"fmt"
	"os"

	"github.com/hsqbyte/qbot/src"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <command>")
		fmt.Println("Commands:")
		fmt.Println("  dev    - 启动开发模式")
		fmt.Println("  prod   - 启动生产模式")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "dev":
		os.Setenv("APP_ENV", "dev")
	case "prod":
		os.Setenv("APP_ENV", "prod")
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}

	src.Run()
}
