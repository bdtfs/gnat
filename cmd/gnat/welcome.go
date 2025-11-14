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
	fmt.Println("\nEndpoints:")
	fmt.Println("  POST   /api/setups          - Create test setup")
	fmt.Println("  GET    /api/setups          - List all setups")
	fmt.Println("  GET    /api/setups/{id}     - Get setup details")
	fmt.Println("  DELETE /api/setups/{id}     - Delete setup")
	fmt.Println("  POST   /api/runs            - Start a run")
	fmt.Println("  GET    /api/runs            - List all runs")
	fmt.Println("  GET    /api/runs/{id}       - Get run details")
	fmt.Println("  GET    /api/runs/{id}/stats - Get run statistics")
	fmt.Println("  POST   /api/runs/{id}/cancel - Cancel active run")
	fmt.Println("\nReady to accept requests...")
	fmt.Println()
}
