package main

import "fmt"

const (
	version  = "0.1.0"
	asciiArt = `
   _____ _   _       _______ 
  / ____| \ | |   /\|__   __|
 | |  __|  \| |  /  \  | |   
 | | |_ | . ' | / /\ \ | |   
 | |__| | |\  |/ ____ \| |   
  \_____|_| \_/_/    \_\_|   
                              
 Load Testing Made Beautiful`
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
