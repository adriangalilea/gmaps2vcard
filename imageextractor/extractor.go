package imageextractor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// DebugLevel controls the verbosity of logging
type DebugLevel int

const (
	DebugNone DebugLevel = iota
	DebugBasic
	DebugVerbose
	DebugVeryVerbose
)

// Config holds configuration for the image extractor
type Config struct {
	DebugLevel DebugLevel
	Timeout    time.Duration
	WaitTime   time.Duration
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DebugLevel: DebugVerbose,
		Timeout:    30 * time.Second,
		WaitTime:   3 * time.Second,
	}
}

// Result contains the extracted image information
type Result struct {
	ImageURL    string
	ImageBase64 string
	Found       bool
	Error       error
	DebugInfo   *DebugInfo
}

// DebugInfo contains detailed debugging information
type DebugInfo struct {
	Selectors      []SelectorAttempt
	PageLoadTime   time.Duration
	ExtractionTime time.Duration
	TotalTime      time.Duration
	PageTitle      string
	PageURL        string
}

// SelectorAttempt tracks each selector attempt
type SelectorAttempt struct {
	Selector string
	Method   string
	Success  bool
	Value    string
	Error    error
}

// Extractor handles image extraction from Google Maps
type Extractor struct {
	config *Config
}

// NewExtractor creates a new image extractor
func NewExtractor(config *Config) *Extractor {
	if config == nil {
		config = DefaultConfig()
	}
	return &Extractor{config: config}
}

// Extract fetches the business image from a Google Maps URL
func (e *Extractor) Extract(pageURL string) *Result {
	startTime := time.Now()
	result := &Result{
		DebugInfo: &DebugInfo{
			Selectors: make([]SelectorAttempt, 0),
		},
	}

	e.logBasic("=== Starting Image Extraction ===")
	e.logBasic("URL: %s", pageURL)
	e.logBasic("Config: Timeout=%v, WaitTime=%v", e.config.Timeout, e.config.WaitTime)

	// Create chromedp context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Navigate and wait for page load
	pageLoadStart := time.Now()
	e.logVerbose("→ Navigating to page...")

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
	)

	if err != nil {
		result.Error = fmt.Errorf("page navigation failed: %w", err)
		e.logBasic("✗ Navigation failed: %v", err)
		return result
	}

	result.DebugInfo.PageLoadTime = time.Since(pageLoadStart)
	e.logVerbose("✓ Page loaded in %v", result.DebugInfo.PageLoadTime)

	// Get page info for debugging
	var pageTitle, currentURL string
	chromedp.Run(ctx,
		chromedp.Title(&pageTitle),
		chromedp.Location(&currentURL),
	)
	result.DebugInfo.PageTitle = pageTitle
	result.DebugInfo.PageURL = currentURL
	e.logVeryVerbose("Page title: %s", pageTitle)
	e.logVeryVerbose("Current URL: %s", currentURL)

	// Wait for dynamic content
	e.logVerbose("→ Waiting %v for dynamic content...", e.config.WaitTime)
	chromedp.Run(ctx, chromedp.Sleep(e.config.WaitTime))

	// Try multiple selectors in order of reliability
	extractionStart := time.Now()
	selectors := []struct {
		query  string
		method string
		desc   string
	}{
		{
			query:  `//button[contains(@class, 'aoRNLd')]//img`,
			method: "xpath-business-photo-button",
			desc:   "Business photo button (common structure)",
		},
		{
			query:  `//*[@id="QA0Szd"]//div[contains(@class, 'RZ66Rb')]//button//img`,
			method: "xpath-sidebar-button",
			desc:   "Sidebar photo button",
		},
		{
			query:  `//button[@data-photo-index]//img`,
			method: "xpath-photo-index",
			desc:   "Photo index button",
		},
		{
			query:  `button[aria-label*="Photo"] img`,
			method: "css-aria-photo",
			desc:   "Photo button by aria-label",
		},
		{
			query:  `.RZ66Rb button img`,
			method: "css-class-button",
			desc:   "Photo section button by class",
		},
		{
			query:  `//img[contains(@src, 'googleusercontent.com')]`,
			method: "xpath-any-gusercontent",
			desc:   "Any Googleusercontent image (fallback)",
		},
	}

	e.logVerbose("→ Trying %d selectors...", len(selectors))

	for i, sel := range selectors {
		e.logVeryVerbose("  [%d/%d] Trying: %s (%s)", i+1, len(selectors), sel.desc, sel.method)

		attempt := SelectorAttempt{
			Selector: sel.query,
			Method:   sel.method,
		}

		var imgSrc string
		var err error

		// Determine if XPath or CSS
		if strings.HasPrefix(sel.method, "xpath-") {
			err = chromedp.Run(ctx,
				chromedp.AttributeValue(sel.query, "src", &imgSrc, nil, chromedp.BySearch),
			)
		} else {
			err = chromedp.Run(ctx,
				chromedp.AttributeValue(sel.query, "src", &imgSrc, nil, chromedp.ByQuery),
			)
		}

		attempt.Value = imgSrc
		attempt.Error = err

		if err == nil && imgSrc != "" {
			// Validate that it's a real image URL
			if e.isValidImageURL(imgSrc) {
				attempt.Success = true
				result.ImageURL = imgSrc
				result.Found = true
				e.logVerbose("  ✓ Success! Found image: %.80s...", imgSrc)
				e.logVeryVerbose("  Full URL: %s", imgSrc)
				result.DebugInfo.Selectors = append(result.DebugInfo.Selectors, attempt)
				break
			} else {
				e.logVeryVerbose("  ⚠ Found URL but invalid format: %s", imgSrc)
			}
		} else {
			if err != nil {
				e.logVeryVerbose("  ✗ Error: %v", err)
			} else {
				e.logVeryVerbose("  ✗ No result")
			}
		}

		result.DebugInfo.Selectors = append(result.DebugInfo.Selectors, attempt)
	}

	result.DebugInfo.ExtractionTime = time.Since(extractionStart)

	if !result.Found {
		result.Error = fmt.Errorf("no image found after trying %d selectors", len(selectors))
		e.logBasic("✗ Image extraction failed: %v", result.Error)
	} else {
		e.logBasic("✓ Image extracted successfully in %v", result.DebugInfo.ExtractionTime)

		// Download and encode the image
		e.logVerbose("→ Downloading and encoding image...")
		base64Data, err := e.downloadAndEncode(result.ImageURL)
		if err != nil {
			e.logBasic("⚠ Warning: Failed to download/encode image: %v", err)
		} else {
			result.ImageBase64 = base64Data
			e.logVerbose("✓ Image downloaded and encoded (%d bytes)", len(base64Data))
		}
	}

	result.DebugInfo.TotalTime = time.Since(startTime)
	e.logBasic("=== Extraction Complete (total: %v) ===", result.DebugInfo.TotalTime)

	return result
}

// isValidImageURL checks if the URL looks like a valid Google image
func (e *Extractor) isValidImageURL(url string) bool {
	// Must be from googleusercontent.com
	if !strings.Contains(url, "googleusercontent.com") {
		return false
	}

	// Must be https
	if !strings.HasPrefix(url, "https://") {
		return false
	}

	// Should not be a 1x1 placeholder
	if strings.Contains(url, "=w1-h1") {
		return false
	}

	return true
}

// downloadAndEncode downloads the image and encodes it to base64
func (e *Extractor) downloadAndEncode(imageURL string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read data: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	return encoded, nil
}

// PrintDebugInfo prints detailed debugging information
func (e *Extractor) PrintDebugInfo(result *Result) {
	if result.DebugInfo == nil {
		return
	}

	info := result.DebugInfo

	fmt.Println("\n=== Image Extraction Debug Info ===")
	fmt.Printf("Page Title: %s\n", info.PageTitle)
	fmt.Printf("Page URL: %s\n", info.PageURL)
	fmt.Printf("Page Load Time: %v\n", info.PageLoadTime)
	fmt.Printf("Extraction Time: %v\n", info.ExtractionTime)
	fmt.Printf("Total Time: %v\n", info.TotalTime)
	fmt.Println("\nSelector Attempts:")

	for i, sel := range info.Selectors {
		status := "✗"
		if sel.Success {
			status = "✓"
		}
		fmt.Printf("  %s [%d] %s\n", status, i+1, sel.Method)
		fmt.Printf("      Selector: %s\n", sel.Selector)
		if sel.Error != nil {
			fmt.Printf("      Error: %v\n", sel.Error)
		}
		if sel.Value != "" {
			fmt.Printf("      Value: %.100s\n", sel.Value)
		}
	}

	if result.Found {
		fmt.Printf("\n✓ Result: %s\n", result.ImageURL)
	} else {
		fmt.Printf("\n✗ Result: No image found\n")
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}
	fmt.Println("===================================")
}

// Logging helpers
func (e *Extractor) logBasic(format string, args ...interface{}) {
	if e.config.DebugLevel >= DebugBasic {
		log.Printf("[ImageExtractor] "+format, args...)
	}
}

func (e *Extractor) logVerbose(format string, args ...interface{}) {
	if e.config.DebugLevel >= DebugVerbose {
		log.Printf("[ImageExtractor] "+format, args...)
	}
}

func (e *Extractor) logVeryVerbose(format string, args ...interface{}) {
	if e.config.DebugLevel >= DebugVeryVerbose {
		log.Printf("[ImageExtractor] "+format, args...)
	}
}
