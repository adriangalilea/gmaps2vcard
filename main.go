package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gmaps2vcard/imageextractor"
	"gmaps2vcard/schedule"
	"gmaps2vcard/urlnormalizer"

	"github.com/chromedp/chromedp"
	"github.com/emersion/go-vcard"
)

type BusinessData struct {
	Name         string
	Address      string
	Phone        string
	Website      string
	Hours        string // Raw hours text from scraping
	HoursClean   string // Formatted hours from schedule parser
	PhotoURL     string
	PhotoBase64  string
	Latitude     string
	Longitude    string
}

var debugSchedule bool

func main() {
	flag.BoolVar(&debugSchedule, "debug-schedule", false, "Enable debug logging for schedule parsing")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gmaps2vcard [options] <google-maps-url>")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fmt.Fprintln(os.Stderr, "  -debug-schedule  Enable debug logging for schedule parsing")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  gmaps2vcard 'https://share.google/w4UZTre3NvPyC3b3Q'")
		fmt.Fprintln(os.Stderr, "  gmaps2vcard -debug-schedule 'https://share.google/w4UZTre3NvPyC3b3Q'")
		os.Exit(1)
	}

	inputURL := flag.Arg(0)

	// Validate URL
	if !isValidGoogleMapsURL(inputURL) {
		log.Fatalf("Error: Not a valid Google Maps URL: %s", inputURL)
	}
	fmt.Println("✓ Valid Google Maps URL")

	// Normalize URL to maps/place format
	fmt.Println("→ Normalizing URL...")
	urlNormalizer := urlnormalizer.NewNormalizer(urlnormalizer.DefaultConfig())
	normResult := urlNormalizer.Normalize(inputURL)
	if !normResult.Success {
		log.Fatalf("Error normalizing URL: %v", normResult.Error)
	}
	fmt.Printf("✓ Normalized to maps/place URL: %.80s...\n", normResult.NormalizedURL)

	mapsURL := normResult.NormalizedURL

	// Extract business data
	fmt.Println("→ Extracting business data...")
	business, err := extractBusinessData(mapsURL)
	if err != nil {
		log.Fatalf("Error extracting data: %v", err)
	}

	// Parse and format schedule
	if business.Hours != "" {
		fmt.Println("→ Parsing schedule...")
		parsedSchedule, err := schedule.Parse(business.Hours, debugSchedule)
		if err != nil {
			log.Printf("⚠ Warning: schedule parsing failed: %v", err)
		} else {
			business.HoursClean = parsedSchedule.Format(debugSchedule)
			if debugSchedule {
				log.Printf("[DEBUG] Raw hours: %q", business.Hours)
				log.Printf("[DEBUG] Clean hours: %q", business.HoursClean)
			}
		}
	}

	// Extract business image
	fmt.Println("→ Extracting business image...")
	imageConfig := imageextractor.DefaultConfig()
	imageConfig.DebugLevel = imageextractor.DebugVerbose
	extractor := imageextractor.NewExtractor(imageConfig)
	imageResult := extractor.Extract(mapsURL)

	if imageResult.Found {
		business.PhotoURL = imageResult.ImageURL
		business.PhotoBase64 = imageResult.ImageBase64
		fmt.Printf("✓ Business image found: %.80s...\n", imageResult.ImageURL)
	} else {
		fmt.Fprintf(os.Stderr, "⚠ Warning: Business image not found: %v\n", imageResult.Error)
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

	var name, address, phone, website, hours string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content

		// Extract business name
		chromedp.Text(`h1`, &name, chromedp.NodeVisible, chromedp.ByQuery),

		// Extract address
		chromedp.AttributeValue(`button[data-item-id="address"]`, "aria-label", &address, nil, chromedp.ByQuery),

		// Extract phone
		chromedp.AttributeValue(`button[data-item-id*="phone"]`, "aria-label", &phone, nil, chromedp.ByQuery),

		// Extract website
		chromedp.AttributeValue(`a[data-item-id="authority"]`, "href", &website, nil, chromedp.ByQuery),
	)

	// Try to extract hours (best effort - don't fail if not found)
	if err == nil {
		// First try to click the hours section to expand full schedule
		chromedp.Run(ctx,
			chromedp.Click(`div.OqCZI.fontBodyMedium.WVXvdc`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)

		// Then get the hours text from the expanded section
		chromedp.Run(ctx,
			chromedp.Text(`div.OqCZI.fontBodyMedium.WVXvdc`, &hours, chromedp.NodeVisible, chromedp.ByQuery),
		)
	}

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
	if hours != "" {
		business.Hours = hours
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

	if business.HoursClean != "" {
		fmt.Printf("  Hours: %s\n", business.HoursClean)
	} else if business.Hours != "" {
		fmt.Printf("  Hours (raw): %s\n", business.Hours)
	} else {
		fmt.Printf("  Hours: (not found)\n")
	}

	fmt.Printf("  Photo: %s\n", orNotFound(business.PhotoURL))
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

	// Business photo (use base64-encoded data for Apple Contacts compatibility)
	if business.PhotoBase64 != "" {
		card.Add(vcard.FieldPhoto, &vcard.Field{
			Value: business.PhotoBase64,
			Params: vcard.Params{
				"ENCODING": []string{"b"},
				"TYPE":     []string{"JPEG"},
			},
		})
	}

	// Business hours in NOTE field (prefer clean format)
	hoursToUse := business.HoursClean
	if hoursToUse == "" {
		hoursToUse = business.Hours
	}
	if hoursToUse != "" {
		card.Set(vcard.FieldNote, &vcard.Field{
			Value: "Hours: " + hoursToUse,
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
