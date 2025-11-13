package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"gmaps2vcard/imageextractor"
	"gmaps2vcard/schedule"
	"gmaps2vcard/scraper"

	"github.com/emersion/go-vcard"
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gmaps2vcard <google-maps-url>")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  gmaps2vcard 'https://share.google/w4UZTre3NvPyC3b3Q'")
		os.Exit(1)
	}

	inputURL := flag.Arg(0)

	// Validate URL
	if !isValidGoogleMapsURL(inputURL) {
		log.Fatalf("Error: Not a valid Google Maps URL: %s", inputURL)
	}
	fmt.Println("✓ Valid Google Maps URL")

	// Extract ALL data in ONE chromedp session (handles URL normalization too)
	fmt.Println("→ Extracting business data...")
	business, err := scraper.Extract(inputURL, nil)
	if err != nil {
		log.Fatalf("Error scraping data: %v", err)
	}

	// Parse schedule (if hours found)
	var hoursClean string
	if business.Hours != "" {
		parsedSchedule, err := schedule.Parse(business.Hours, false)
		if err != nil {
			log.Printf("⚠ Warning: schedule parsing failed: %v", err)
		} else {
			hoursClean = parsedSchedule.Format(false)
		}
	}

	// Download and encode image (if URL found)
	var photoBase64 string
	if business.PhotoURL != "" {
		photoBase64, err = imageextractor.DownloadAndEncode(business.PhotoURL)
		if err != nil {
			log.Printf("⚠ Warning: image download failed: %v", err)
		}
	}

	// Print extracted data
	printBusinessData(business, hoursClean)

	// Validate we have at least a name
	if business.Name == "" {
		log.Fatal("Error: Could not extract business name")
	}

	// Generate vCard
	fmt.Println("\n→ Generating vCard...")
	vcardData := generateVCard(business, hoursClean, photoBase64)

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

func printBusinessData(business *scraper.BusinessData, hoursClean string) {
	fmt.Println("\nExtracted information:")
	fmt.Printf("  Name: %s\n", orNotFound(business.Name))
	fmt.Printf("  Address: %s\n", orNotFound(business.Address))
	fmt.Printf("  Phone: %s\n", orNotFound(business.Phone))
	fmt.Printf("  Website: %s\n", orNotFound(business.Website))

	if hoursClean != "" {
		fmt.Printf("  Hours: %s\n", hoursClean)
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

func generateVCard(business *scraper.BusinessData, hoursClean, photoBase64 string) string {
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
			Value: ";;" + business.Address + ";;;;",
			Params: vcard.Params{
				vcard.ParamType: []string{"WORK"},
			},
		})
	}

	// Phone
	if business.Phone != "" {
		card.Add(vcard.FieldTelephone, &vcard.Field{
			Value:  business.Phone,
			Params: vcard.Params{vcard.ParamType: []string{"WORK"}},
		})
	}

	// Website
	if business.Website != "" {
		card.Add(vcard.FieldURL, &vcard.Field{
			Value:  business.Website,
			Params: vcard.Params{vcard.ParamType: []string{"WORK"}},
		})
	}

	// Geo coordinates
	if business.Latitude != "" && business.Longitude != "" {
		geoValue := fmt.Sprintf("%s;%s", business.Latitude, business.Longitude)
		card.Set("GEO", &vcard.Field{Value: geoValue})
	}

	// Business photo
	if photoBase64 != "" {
		card.Add(vcard.FieldPhoto, &vcard.Field{
			Value: photoBase64,
			Params: vcard.Params{
				"ENCODING": []string{"b"},
				"TYPE":     []string{"JPEG"},
			},
		})
	}

	// Business hours
	hoursToUse := hoursClean
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
