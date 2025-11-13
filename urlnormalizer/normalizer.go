package urlnormalizer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
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

// Config holds configuration for the URL normalizer
type Config struct {
	DebugLevel DebugLevel
	Timeout    time.Duration
	WaitTime   time.Duration
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DebugLevel: DebugVerbose,
		Timeout:    45 * time.Second,
		WaitTime:   3 * time.Second,
	}
}

// Result contains the normalized URL and debug information
type Result struct {
	NormalizedURL string
	URLType       string // "direct", "search->place", "unknown"
	Success       bool
	Error         error
	DebugInfo     *DebugInfo
}

// DebugInfo contains detailed debugging information
type DebugInfo struct {
	InputURL        string
	RedirectedURL   string
	DetectedType    string
	SearchAttempts  []SearchAttempt
	RedirectTime    time.Duration
	ExtractionTime  time.Duration
	TotalTime       time.Duration
	PageTitle       string
	PageURL         string
	CaptchaDetected bool
}

// SearchAttempt tracks each search page extraction attempt
type SearchAttempt struct {
	Method  string
	Success bool
	Value   string
	Error   error
}

// Normalizer handles URL normalization to Google Maps place URLs
type Normalizer struct {
	config *Config
}

// NewNormalizer creates a new URL normalizer
func NewNormalizer(config *Config) *Normalizer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Normalizer{config: config}
}

// Normalize takes any Google Maps URL and normalizes it to a /maps/place/ URL
func (n *Normalizer) Normalize(inputURL string) *Result {
	startTime := time.Now()
	result := &Result{
		DebugInfo: &DebugInfo{
			InputURL:       inputURL,
			SearchAttempts: make([]SearchAttempt, 0),
		},
	}

	n.logBasic("=== Starting URL Normalization ===")
	n.logBasic("Input URL: %s", inputURL)

	// Step 1: Follow all redirects
	redirectStart := time.Now()
	n.logVerbose("→ Following redirects...")
	finalURL, err := n.followRedirects(inputURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to follow redirects: %w", err)
		n.logBasic("✗ Redirect failed: %v", err)
		return result
	}
	result.DebugInfo.RedirectedURL = finalURL
	result.DebugInfo.RedirectTime = time.Since(redirectStart)
	n.logVerbose("✓ Redirected to: %s", finalURL)

	// Step 2: Parse and detect URL type
	u, err := url.Parse(finalURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse URL: %w", err)
		return result
	}

	// Case 1: Already a /maps/place/ URL - perfect!
	if strings.Contains(u.Path, "/maps/place/") {
		n.logBasic("✓ Already a maps/place URL")
		result.NormalizedURL = finalURL
		result.URLType = "direct"
		result.Success = true
		result.DebugInfo.DetectedType = "maps/place (direct)"
		result.DebugInfo.TotalTime = time.Since(startTime)
		return result
	}

	// Case 2: It's a /search URL - need to extract the maps/place link
	if strings.Contains(u.Path, "/search") {
		n.logBasic("→ Detected search page, extracting maps/place link...")
		result.DebugInfo.DetectedType = "search page"

		extractionStart := time.Now()
		mapsURL, err := n.extractFromSearchPage(finalURL, result.DebugInfo)
		result.DebugInfo.ExtractionTime = time.Since(extractionStart)

		if err != nil {
			result.Error = err
			n.logBasic("✗ Extraction failed: %v", err)
			return result
		}

		result.NormalizedURL = mapsURL
		result.URLType = "search->place"
		result.Success = true
		n.logBasic("✓ Normalized to: %s", mapsURL)
	} else {
		// Case 3: Unknown URL type
		result.Error = fmt.Errorf("unknown Google Maps URL type: %s\nPlease provide either:\n  - A share.google link\n  - A direct maps/place URL\n  - Or check if Google changed their URL structure", finalURL)
		result.DebugInfo.DetectedType = "unknown"
		n.logBasic("✗ Unknown URL type: %s", u.Path)
		return result
	}

	result.DebugInfo.TotalTime = time.Since(startTime)
	n.logBasic("=== Normalization Complete (total: %v) ===", result.DebugInfo.TotalTime)
	return result
}

// followRedirects follows all HTTP redirects and returns the final URL
func (n *Normalizer) followRedirects(inputURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow all redirects
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", inputURL, nil)
	if err != nil {
		return "", err
	}

	// Legitimate browser headers for personal use
	// Mimicking real Chrome on macOS to avoid triggering bot detection
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// Check for consent page
	if strings.Contains(finalURL, "consent.google.com") {
		u, err := url.Parse(finalURL)
		if err != nil {
			return finalURL, nil
		}
		continueURL := u.Query().Get("continue")
		if continueURL != "" {
			return continueURL, nil
		}
	}

	return finalURL, nil
}

// extractFromSearchPage navigates a Google search page and extracts the maps/place link
func (n *Normalizer) extractFromSearchPage(searchURL string, debugInfo *DebugInfo) (string, error) {
	// Set up chromedp with legitimate browser fingerprint for personal use
	// Using realistic macOS Safari/Chrome headers to avoid triggering bot detection
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// Modern Chrome on macOS
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),

		// Disable automation indicators
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),

		// Enable features that real browsers have
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),

		// Standard window size (not headless indicator sizes)
		chromedp.WindowSize(1920, 1080),

		// Accept language for personal browsing
		chromedp.Flag("lang", "en-US,en"),

		// Run headless but with modern mode
		chromedp.Flag("headless", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	defer ctxCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, n.config.Timeout)
	defer timeoutCancel()

	ctx = timeoutCtx

	n.logVerbose("→ Navigating to search page...")
	n.logVeryVerbose("Timeout set to: %v", n.config.Timeout)
	n.logVeryVerbose("Wait time set to: %v", n.config.WaitTime)

	var pageTitle, pageURL string

	// Navigate and get page info
	n.logVeryVerbose("Starting chromedp.Run...")
	err := chromedp.Run(ctx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(n.config.WaitTime),
		chromedp.Title(&pageTitle),
		chromedp.Location(&pageURL),
	)

	if err != nil {
		n.logVeryVerbose("chromedp.Run failed with error: %v", err)
		return "", fmt.Errorf("failed to navigate: %w", err)
	}
	n.logVeryVerbose("chromedp.Run completed successfully")

	debugInfo.PageTitle = pageTitle
	debugInfo.PageURL = pageURL
	n.logVerbose("✓ Page loaded: %s", pageTitle)
	n.logVeryVerbose("Current URL: %s", pageURL)

	// Check for CAPTCHA/bot detection
	if strings.Contains(pageURL, "/sorry/") {
		debugInfo.CaptchaDetected = true
		return "", fmt.Errorf("Google blocked automated access (CAPTCHA/bot detection)\n\nThis happens with share.google links that redirect to search pages.\nPlease use the direct Maps URL instead:\n\n1. Open the share.google link in your browser\n2. Copy the final google.com/maps/place/ URL from the address bar\n3. Use that URL with this tool\n\nExample: gmaps2vcard \"https://www.google.com/maps/place/...\"")
	}

	// Strategy 1: Click on the address link
	// COMMENTED OUT: This strategy triggers bot detection and rarely works
	// Clicking elements on search pages often leads to CAPTCHA or stays on same page
	// Keeping code for reference but disabled for production use
	/*
		n.logVerbose("→ Strategy 1: Clicking on address link to navigate to maps/place...")
		attempt1 := SearchAttempt{Method: "click-address-link"}

		var locationAfterClick string
		err = chromedp.Run(ctx,
			chromedp.Click(`a[data-url*="/maps/place/"]`, chromedp.ByQuery),
			chromedp.Sleep(3*time.Second),
			chromedp.Location(&locationAfterClick),
		)

		attempt1.Value = locationAfterClick
		if err == nil && strings.Contains(locationAfterClick, "/maps/place/") {
			if strings.Contains(locationAfterClick, "/sorry/") {
				attempt1.Error = fmt.Errorf("clicked but redirected to CAPTCHA page")
				debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt1)
				debugInfo.CaptchaDetected = true
				n.logVeryVerbose("✗ Click led to CAPTCHA page: %s", locationAfterClick)
			} else {
				attempt1.Success = true
				debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt1)
				n.logVerbose("✓ Successfully clicked and navigated to: %s", locationAfterClick)
				return n.makeAbsoluteURL(locationAfterClick), nil
			}
		} else {
			attempt1.Error = err
			debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt1)
			n.logVeryVerbose("✗ Click strategy failed: %v (URL: %s)", err, locationAfterClick)
		}
	*/

	// Strategy 1: Extract href from address link (most reliable, avoids bot detection)
	n.logVerbose("→ Strategy 1: Extracting href from address link...")
	attempt1 := SearchAttempt{Method: "extract-href-address"}

	var hrefFull string
	err = chromedp.Run(ctx,
		chromedp.AttributeValue(`div[data-attrid="kc:/location/location:address"] a[href*="/maps/place/"]`, "href", &hrefFull, nil, chromedp.ByQuery),
	)

	if err == nil && hrefFull != "" {
		attempt1.Success = true
		attempt1.Value = hrefFull
		debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt1)
		n.logVerbose("✓ Extracted href from address: %s", hrefFull)
		return n.makeAbsoluteURL(hrefFull), nil
	}
	attempt1.Error = err
	debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt1)
	n.logVeryVerbose("✗ href from address extraction failed: %v", err)

	// Strategy 2: Extract data-url attribute (fallback, gives minimal URL)
	n.logVerbose("→ Strategy 2: Extracting data-url attribute...")
	attempt2 := SearchAttempt{Method: "extract-data-url"}

	var dataURL string
	err = chromedp.Run(ctx,
		chromedp.AttributeValue(`a[data-url*="/maps/place/"]`, "data-url", &dataURL, nil, chromedp.ByQuery),
	)

	if err == nil && dataURL != "" {
		attempt2.Success = true
		attempt2.Value = dataURL
		debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt2)
		n.logVerbose("✓ Extracted data-url: %s", dataURL)
		return n.makeAbsoluteURL(dataURL), nil
	}
	attempt2.Error = err
	debugInfo.SearchAttempts = append(debugInfo.SearchAttempts, attempt2)
	n.logVeryVerbose("✗ data-url extraction failed: %v", err)

	// All strategies failed
	return "", fmt.Errorf("failed to extract maps/place link after trying %d strategies\nGoogle may have changed their page structure", len(debugInfo.SearchAttempts))
}

// makeAbsoluteURL converts relative URLs to absolute
func (n *Normalizer) makeAbsoluteURL(urlStr string) string {
	if strings.HasPrefix(urlStr, "/") {
		return "https://www.google.com" + urlStr
	}
	return urlStr
}

// PrintDebugInfo prints detailed debugging information
func (n *Normalizer) PrintDebugInfo(result *Result) {
	if result.DebugInfo == nil {
		return
	}

	info := result.DebugInfo

	fmt.Println("\n=== URL Normalization Debug Info ===")
	fmt.Printf("Input URL: %s\n", info.InputURL)
	fmt.Printf("Redirected URL: %s\n", info.RedirectedURL)
	fmt.Printf("Detected Type: %s\n", info.DetectedType)
	fmt.Printf("Redirect Time: %v\n", info.RedirectTime)
	fmt.Printf("Extraction Time: %v\n", info.ExtractionTime)
	fmt.Printf("Total Time: %v\n", info.TotalTime)

	if info.PageTitle != "" {
		fmt.Printf("Page Title: %s\n", info.PageTitle)
	}
	if info.PageURL != "" {
		fmt.Printf("Page URL: %s\n", info.PageURL)
	}
	if info.CaptchaDetected {
		fmt.Printf("CAPTCHA Detected: YES\n")
	}

	if len(info.SearchAttempts) > 0 {
		fmt.Println("\nSearch Extraction Attempts:")
		for i, attempt := range info.SearchAttempts {
			status := "✗"
			if attempt.Success {
				status = "✓"
			}
			fmt.Printf("  %s [%d] %s\n", status, i+1, attempt.Method)
			if attempt.Error != nil {
				fmt.Printf("      Error: %v\n", attempt.Error)
			}
			if attempt.Value != "" {
				fmt.Printf("      Value: %s\n", attempt.Value)
			}
		}
	}

	if result.Success {
		fmt.Printf("\n✓ Result: %s\n", result.NormalizedURL)
		fmt.Printf("URL Type: %s\n", result.URLType)
	} else {
		fmt.Printf("\n✗ Failed to normalize URL\n")
		if result.Error != nil {
			fmt.Printf("Error: %v\n", result.Error)
		}
	}
	fmt.Println("=====================================")
}

// Logging helpers
func (n *Normalizer) logBasic(format string, args ...interface{}) {
	if n.config.DebugLevel >= DebugBasic {
		log.Printf("[URLNormalizer] "+format, args...)
	}
}

func (n *Normalizer) logVerbose(format string, args ...interface{}) {
	if n.config.DebugLevel >= DebugVerbose {
		log.Printf("[URLNormalizer] "+format, args...)
	}
}

func (n *Normalizer) logVeryVerbose(format string, args ...interface{}) {
	if n.config.DebugLevel >= DebugVeryVerbose {
		log.Printf("[URLNormalizer] "+format, args...)
	}
}
