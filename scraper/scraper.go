package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BusinessData contains all extracted business information
type BusinessData struct {
	Name      string
	Address   string
	Phone     string
	Website   string
	Hours     string // Raw hours text for schedule parser
	PhotoURL  string
	Latitude  string
	Longitude string
}

// Config holds configuration for the scraper
type Config struct {
	Timeout  time.Duration
	WaitTime time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Timeout:  45 * time.Second,
		WaitTime: 3 * time.Second,
	}
}

// Extract extracts all business data from ANY Google Maps URL in ONE chromedp session
// Handles URL normalization, search page extraction, and business data scraping
func Extract(inputURL string, config *Config) (*BusinessData, error) {
	if config == nil {
		config = DefaultConfig()
	}

	business := &BusinessData{}
	log.Printf("[Scraper] Starting extraction from: %.80s...", inputURL)

	// Step 1: Follow HTTP redirects (no chromedp needed)
	log.Printf("[Scraper] Following redirects...")
	finalURL, err := followRedirects(inputURL)
	if err != nil {
		return nil, fmt.Errorf("failed to follow redirects: %w", err)
	}
	log.Printf("[Scraper] Redirected to: %.80s...", finalURL)

	// Step 2: Parse URL and check type
	u, err := url.Parse(finalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract coordinates from URL (works for any URL format)
	coordsRe := regexp.MustCompile(`@(-?\d+\.\d+),(-?\d+\.\d+)`)
	if matches := coordsRe.FindStringSubmatch(finalURL); len(matches) == 3 {
		business.Latitude = matches[1]
		business.Longitude = matches[2]
	}

	// Step 3: Set up chromedp - ONE session for EVERYTHING
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("headless", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	defer ctxCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, config.Timeout)
	defer timeoutCancel()

	// Step 4: Handle URL type and navigate
	var mapsPlaceURL string

	if strings.Contains(u.Path, "/maps/place/") {
		// Already a maps/place URL - perfect!
		log.Printf("[Scraper] Already a maps/place URL")
		mapsPlaceURL = finalURL

	} else if strings.Contains(u.Path, "/search") {
		// Search page - need to extract maps/place link
		log.Printf("[Scraper] Detected search page, extracting maps/place link...")
		extractedURL, err := extractMapsPlaceFromSearch(timeoutCtx, finalURL, config)
		if err != nil {
			return nil, fmt.Errorf("failed to extract from search page: %w", err)
		}
		mapsPlaceURL = extractedURL
		log.Printf("[Scraper] Extracted maps/place URL: %.80s...", mapsPlaceURL)

	} else {
		return nil, fmt.Errorf("unknown Google Maps URL type: %s", u.Path)
	}

	// Step 5: Navigate to maps/place URL and extract ALL business data (same chromedp session)
	log.Printf("[Scraper] Extracting business data...")
	err = extractBusinessData(timeoutCtx, mapsPlaceURL, business, config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract business data: %w", err)
	}

	log.Printf("[Scraper] ✓ Extraction complete")
	return business, nil
}

// followRedirects follows all HTTP redirects and returns the final URL
func followRedirects(inputURL string) (string, error) {
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

	// Legitimate browser headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// Handle Google consent page
	if strings.Contains(finalURL, "consent.google.com") {
		u, err := url.Parse(finalURL)
		if err != nil {
			return finalURL, nil
		}
		if continueURL := u.Query().Get("continue"); continueURL != "" {
			return continueURL, nil
		}
	}

	return finalURL, nil
}

// extractMapsPlaceFromSearch extracts the maps/place link from a Google search page
func extractMapsPlaceFromSearch(ctx context.Context, searchURL string, config *Config) (string, error) {
	var pageURL string

	err := chromedp.Run(ctx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(config.WaitTime),
		chromedp.Location(&pageURL),
	)

	if err != nil {
		return "", fmt.Errorf("failed to navigate to search page: %w", err)
	}

	// Check for CAPTCHA
	if strings.Contains(pageURL, "/sorry/") {
		return "", fmt.Errorf("Google CAPTCHA detected - use direct maps/place URL instead")
	}

	// Try to extract maps/place link from page
	var hrefURL string
	err = chromedp.Run(ctx,
		chromedp.AttributeValue(`div[data-attrid="kc:/location/location:address"] a[href*="/maps/place/"]`, "href", &hrefURL, nil, chromedp.ByQuery),
	)

	if err == nil && hrefURL != "" {
		if strings.HasPrefix(hrefURL, "/") {
			return "https://www.google.com" + hrefURL, nil
		}
		return hrefURL, nil
	}

	// Fallback: try data-url attribute
	var dataURL string
	err = chromedp.Run(ctx,
		chromedp.AttributeValue(`a[data-url*="/maps/place/"]`, "data-url", &dataURL, nil, chromedp.ByQuery),
	)

	if err == nil && dataURL != "" {
		if strings.HasPrefix(dataURL, "/") {
			return "https://www.google.com" + dataURL, nil
		}
		return dataURL, nil
	}

	return "", fmt.Errorf("no maps/place link found on search page")
}

// extractBusinessData extracts all business data from a maps/place page (in existing chromedp session)
func extractBusinessData(ctx context.Context, pageURL string, business *BusinessData, config *Config) error {
	var name, address, phone, website string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(config.WaitTime),

		// Extract basic business info
		chromedp.Text(`h1`, &name, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.AttributeValue(`button[data-item-id="address"]`, "aria-label", &address, nil, chromedp.ByQuery),
		chromedp.AttributeValue(`button[data-item-id*="phone"]`, "aria-label", &phone, nil, chromedp.ByQuery),
		chromedp.AttributeValue(`a[data-item-id="authority"]`, "href", &website, nil, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to extract basic data: %w", err)
	}

	business.Name = name
	business.Address = cleanAriaLabel(address)
	business.Phone = cleanAriaLabel(phone)
	business.Website = website

	// Extract image URL FIRST (before clicking anything that might open modals)
	photoURL, err := extractImageURL(ctx)
	if err != nil {
		log.Printf("[Scraper] ⚠ Image extraction failed: %v", err)
	} else {
		business.PhotoURL = photoURL
	}

	// Extract hours (click to expand, then scrape)
	hours, err := extractHours(ctx, config)
	if err != nil {
		log.Printf("[Scraper] ⚠ Hours extraction failed: %v", err)
	} else {
		business.Hours = hours
	}

	return nil
}

// extractHours clicks the hours button and extracts the full schedule text
func extractHours(ctx context.Context, config *Config) (string, error) {
	// Click hours button to expand full schedule
	err := chromedp.Run(ctx,
		chromedp.Click(`button[data-item-id="oh"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		return "", fmt.Errorf("failed to click hours button: %w", err)
	}

	// Extract all body text (schedule appears in the modal/panel)
	var bodyText string
	err = chromedp.Run(ctx,
		chromedp.Text("body", &bodyText, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("failed to extract body text: %w", err)
	}

	// Parse out hours section from body text
	hoursMarkers := []string{"Horas\n", "Hours\n", "Horario\n"}
	endMarkers := []string{"Sugerir nuevo horario", "Suggest new hours", "Ocultar el panel", "Hide"}

	var hoursSection string
	for _, marker := range hoursMarkers {
		if idx := strings.Index(bodyText, marker); idx != -1 {
			hoursSection = bodyText[idx+len(marker):]
			break
		}
	}

	if hoursSection == "" {
		return "", fmt.Errorf("hours section not found in page")
	}

	// Trim to end of hours section
	for _, endMarker := range endMarkers {
		if endIdx := strings.Index(hoursSection, endMarker); endIdx != -1 {
			hoursSection = hoursSection[:endIdx]
			break
		}
	}

	return strings.TrimSpace(hoursSection), nil
}

// extractImageURL tries multiple selectors to find the business image
func extractImageURL(ctx context.Context) (string, error) {
	selectors := []string{
		`button[jsaction*="pane.heroHeaderImage"] img`,
		`div.ZKCDEc img`,
		`img[src*="googleusercontent.com"]`,
		`img[src*="gstatic.com/images"]`,
		`button.aoRNLd img`,
		`div[role="img"]`,
	}

	var imageURL string
	for _, selector := range selectors {
		// Use a short timeout for each selector (don't waste time on non-matching selectors)
		selectorCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := chromedp.Run(selectorCtx,
			chromedp.AttributeValue(selector, "src", &imageURL, nil, chromedp.ByQuery),
		)
		cancel()

		if err == nil && imageURL != "" {
			return imageURL, nil
		}
	}

	return "", fmt.Errorf("no image found with any selector")
}

// cleanAriaLabel removes common aria-label prefixes
func cleanAriaLabel(s string) string {
	prefixes := []string{
		"Dirección: ", "Address: ",
		"Teléfono: ", "Phone: ", "Telephone: ",
		"Sitio web: ", "Website: ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}
	return s
}
