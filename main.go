package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/emersion/go-vcard"
)

type BusinessData struct {
	Name      string
	Address   string
	Phone     string
	Website   string
	Latitude  string
	Longitude string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gmaps2vcard <google-maps-url>")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  gmaps2vcard 'https://share.google/w4UZTre3NvPyC3b3Q'")
		os.Exit(1)
	}

	inputURL := os.Args[1]

	// Validate URL
	if !isValidGoogleMapsURL(inputURL) {
		log.Fatalf("Error: Not a valid Google Maps URL: %s", inputURL)
	}
	fmt.Println("✓ Valid Google Maps URL")

	// Follow redirects
	fmt.Println("→ Following redirects...")
	finalURL, err := followRedirects(inputURL)
	if err != nil {
		log.Fatalf("Error following redirects: %v", err)
	}
	if finalURL != inputURL {
		fmt.Printf("✓ Redirected to: %.80s...\n", finalURL)
	}

	// Extract business data
	fmt.Println("→ Extracting business data...")
	business, err := extractBusinessData(finalURL)
	if err != nil {
		log.Fatalf("Error extracting data: %v", err)
	}

	// Print extracted data
	printBusinessData(business)

	// Validate we have at least a name
	if business.Name == "" {
		log.Fatal("Error: Could not extract business name")
	}

	// Generate vCard
	fmt.Println("\n→ Generating vCard...")
	vcardData := generateVCard(business)

	// Save to file
	filename := strings.ReplaceAll(business.Name, "/", "-") + ".vcf"
	if err := os.WriteFile(filename, []byte(vcardData), 0644); err != nil {
		log.Fatalf("Error writing vCard: %v", err)
	}

	fmt.Printf("✓ vCard saved to: %s\n", filename)
	fmt.Println("\nYou can now import this file to your contacts app or iCloud.")
}

func isValidGoogleMapsURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	validDomains := []string{
		"share.google",
		"maps.google.com",
		"www.google.com",
		"google.com",
		"goo.gl",
	}

	for _, domain := range validDomains {
		if strings.HasSuffix(u.Host, domain) {
			return true
		}
	}

	return false
}

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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

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

func extractBusinessData(pageURL string) (*BusinessData, error) {
	business := &BusinessData{}

	// Extract coordinates from URL
	coordsRe := regexp.MustCompile(`@(-?\d+\.\d+),(-?\d+\.\d+)`)
	if matches := coordsRe.FindStringSubmatch(pageURL); len(matches) == 3 {
		business.Latitude = matches[1]
		business.Longitude = matches[2]
	}

	// Extract name from URL first (fallback)
	nameRe := regexp.MustCompile(`/place/([^/@?]+)`)
	if matches := nameRe.FindStringSubmatch(pageURL); len(matches) > 1 {
		business.Name = strings.ReplaceAll(url.QueryEscape(matches[1]), "+", " ")
		business.Name, _ = url.QueryUnescape(business.Name)
	}

	// If it's a search URL, extract from q= parameter
	if u, err := url.Parse(pageURL); err == nil {
		if q := u.Query().Get("q"); q != "" {
			business.Name = q
		}
	}

	// Use chromedp to scrape full details if we have a Maps URL
	if strings.Contains(pageURL, "/maps/place/") {
		if err := scrapeWithChromedp(pageURL, business); err != nil {
			// Chromedp failed, but we still have basic data from URL
			fmt.Fprintf(os.Stderr, "⚠ Warning: chromedp scraping failed: %v\n", err)
		}
	}

	return business, nil
}

func scrapeWithChromedp(pageURL string, business *BusinessData) error {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var name, address, phone, website string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second), // Wait for dynamic content

		// Extract business name
		chromedp.Text(`h1`, &name, chromedp.NodeVisible, chromedp.ByQuery),

		// Extract address
		chromedp.AttributeValue(`button[data-item-id="address"]`, "aria-label", &address, nil, chromedp.ByQuery),

		// Extract phone
		chromedp.AttributeValue(`button[data-item-id*="phone"]`, "aria-label", &phone, nil, chromedp.ByQuery),

		// Extract website
		chromedp.AttributeValue(`a[data-item-id="authority"]`, "href", &website, nil, chromedp.ByQuery),
	)

	if err != nil {
		return err
	}

	// Update business data with scraped info (clean aria-label prefixes)
	if name != "" {
		business.Name = name
	}
	if address != "" {
		business.Address = cleanAriaLabel(address)
	}
	if phone != "" {
		business.Phone = cleanAriaLabel(phone)
	}
	if website != "" {
		business.Website = website
	}

	return nil
}

func cleanAriaLabel(s string) string {
	// Remove common aria-label prefixes like "Dirección: ", "Teléfono: ", etc.
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

func printBusinessData(business *BusinessData) {
	fmt.Println("\nExtracted information:")
	fmt.Printf("  Name: %s\n", orNotFound(business.Name))
	fmt.Printf("  Address: %s\n", orNotFound(business.Address))
	fmt.Printf("  Phone: %s\n", orNotFound(business.Phone))
	fmt.Printf("  Website: %s\n", orNotFound(business.Website))
	if business.Latitude != "" && business.Longitude != "" {
		fmt.Printf("  Location: %s, %s\n", business.Latitude, business.Longitude)
	}
}

func orNotFound(s string) string {
	if s == "" {
		return "(not found)"
	}
	return s
}

func generateVCard(business *BusinessData) string {
	card := make(vcard.Card)

	// Version (required)
	card.SetValue(vcard.FieldVersion, "3.0")

	// Required: Full name
	card.SetValue(vcard.FieldFormattedName, business.Name)

	// Name structure (empty for organizations)
	card.Set(vcard.FieldName, &vcard.Field{
		Value: ";;;;",
	})

	// Organization
	card.SetValue(vcard.FieldOrganization, business.Name)

	// Address
	if business.Address != "" {
		card.Set(vcard.FieldAddress, &vcard.Field{
			Value: ";;"+business.Address+";;;;",
			Params: vcard.Params{
				vcard.ParamType: []string{"WORK"},
			},
		})
	}

	// Phone
	if business.Phone != "" {
		card.Add(vcard.FieldTelephone, &vcard.Field{
			Value: business.Phone,
			Params: vcard.Params{
				vcard.ParamType: []string{"WORK"},
			},
		})
	}

	// Website
	if business.Website != "" {
		card.Add(vcard.FieldURL, &vcard.Field{
			Value: business.Website,
			Params: vcard.Params{
				vcard.ParamType: []string{"WORK"},
			},
		})
	}

	// Geo coordinates
	if business.Latitude != "" && business.Longitude != "" {
		geoValue := fmt.Sprintf("%s;%s", business.Latitude, business.Longitude)
		card.Set("GEO", &vcard.Field{
			Value: geoValue,
		})
	}

	// Encode to string
	var buf strings.Builder
	enc := vcard.NewEncoder(&buf)
	if err := enc.Encode(card); err != nil {
		log.Printf("Warning: vCard encoding error: %v", err)
	}

	return buf.String()
}
