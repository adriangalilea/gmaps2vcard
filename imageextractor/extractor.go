package imageextractor

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
)

// DownloadAndEncode downloads an image from a URL and returns it as base64
// Pure function - no chromedp, just HTTP download and encoding
func DownloadAndEncode(imageURL string) (string, error) {
	log.Printf("[ImageExtractor] Downloading image from: %.80s...", imageURL)

	// Download image
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	log.Printf("[ImageExtractor] ✓ Image downloaded and encoded (%d bytes)", len(imageData))
	return base64Data, nil
}
