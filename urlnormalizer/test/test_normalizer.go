package main

import (
	"fmt"
	"os"

	"gmaps2vcard/urlnormalizer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run test_normalizer.go <google-maps-url>")
		os.Exit(1)
	}

	inputURL := os.Args[1]

	// Create normalizer with very verbose debugging
	config := urlnormalizer.DefaultConfig()
	config.DebugLevel = urlnormalizer.DebugVeryVerbose

	normalizer := urlnormalizer.NewNormalizer(config)
	result := normalizer.Normalize(inputURL)

	// Print detailed debug info
	normalizer.PrintDebugInfo(result)

	// Print final result
	if result.Success {
		fmt.Printf("\n✓ SUCCESS\n")
		fmt.Printf("Normalized URL: %s\n", result.NormalizedURL)
		fmt.Printf("URL Type: %s\n", result.URLType)
	} else {
		fmt.Printf("\n✗ FAILED\n")
		fmt.Printf("Error: %v\n", result.Error)
		os.Exit(1)
	}
}
