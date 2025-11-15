package main

import "fmt"

const (
	version  = "0.1.0"
	asciiArt = `
+--------------------------------------------------------------+
|   ____ _   _    _    _____                                   |
|  / ___| \ | |  / \  |_   _|   GNAT                           |
| | |  _|  \| | / _ \   | |     Load Testing                   |
| | |_| | |\  |/ ___ \  | |     Made Beautiful                 |
|  \____|_| \_/_/   \_\ |_|                                    |
+--------------------------------------------------------------+`
)

func printWelcome(addr string) {
	fmt.Println(asciiArt)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Server started on: %s\n", addr)

	baseURL := "http://localhost" + addr

	fmt.Println("\nEndpoints:")
	fmt.Printf("  POST   %s/api/setups           - Create test setup\n", baseURL)
	fmt.Printf("  GET    %s/api/setups           - List all setups\n", baseURL)
	fmt.Printf("  GET    %s/api/setups/{id}      - Get setup details\n", baseURL)
	fmt.Printf("  DELETE %s/api/setups/{id}      - Delete setup\n", baseURL)
	fmt.Printf("  POST   %s/api/runs             - Start a run\n", baseURL)
	fmt.Printf("  GET    %s/api/runs             - List all runs\n", baseURL)
	fmt.Printf("  GET    %s/api/runs/{id}        - Get run details\n", baseURL)
	fmt.Printf("  GET    %s/api/runs/{id}/stats  - Get run statistics\n", baseURL)
	fmt.Printf("  POST   %s/api/runs/{id}/cancel - Cancel active run\n", baseURL)
	fmt.Println("\nReady to accept requests...")
	fmt.Println()
}
