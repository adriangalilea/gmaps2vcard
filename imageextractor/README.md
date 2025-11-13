# Image Extractor Module

Independent module for extracting business images from Google Maps URLs and adding them to vCards.

## Features

- **High Debugability**: Multiple debug levels with detailed logging
- **Multiple Selector Strategies**: Tries 6+ different selectors to maximize success rate
- **Detailed Result Tracking**: Returns comprehensive debug information for troubleshooting
- **Independent Module**: Can be merged separately without conflicts

## Usage

### Basic Usage

```go
import "gmaps2vcard/imageextractor"

// Create extractor with default config
extractor := imageextractor.NewExtractor(nil)

// Extract image from Google Maps URL
result := extractor.Extract("https://www.google.com/maps/place/...")

if result.Found {
    fmt.Println("Image URL:", result.ImageURL)
} else {
    fmt.Println("Error:", result.Error)
}
```

### Custom Configuration

```go
config := &imageextractor.Config{
    DebugLevel: imageextractor.DebugVeryVerbose,
    Timeout:    45 * time.Second,
    WaitTime:   5 * time.Second,
}

extractor := imageextractor.NewExtractor(config)
result := extractor.Extract(url)
```

### Debug Levels

- `DebugNone`: No logging
- `DebugBasic`: Essential logs only (success/failure)
- `DebugVerbose`: Detailed progress (default)
- `DebugVeryVerbose`: Every selector attempt and page details

### Printing Debug Information

```go
result := extractor.Extract(url)
extractor.PrintDebugInfo(result)
```

This prints:
```
=== Image Extraction Debug Info ===
Page Title: Business Name - Google Maps
Page URL: https://www.google.com/maps/place/...
Page Load Time: 1.23s
Extraction Time: 456ms
Total Time: 1.69s

Selector Attempts:
  ✓ [1] xpath-business-photo-button
      Selector: //button[contains(@class, 'aoRNLd')]//img
      Value: https://lh3.googleusercontent.com/p/AF1Qip...
  ✗ [2] xpath-sidebar-button
      Selector: //*[@id="QA0Szd"]//div[contains(@class, 'RZ66Rb')]//button//img
      Error: context deadline exceeded

✓ Result: https://lh3.googleusercontent.com/p/AF1Qip...
===================================
```

## How It Works

### Selector Strategy

The module tries multiple selectors in order of reliability:

1. **Business photo button** (most common): `//button[contains(@class, 'aoRNLd')]//img`
2. **Sidebar photo button**: `//*[@id="QA0Szd"]//div[contains(@class, 'RZ66Rb')]//button//img`
3. **Photo index button**: `//button[@data-photo-index]//img`
4. **Photo by aria-label** (CSS): `button[aria-label*="Photo"] img`
5. **Photo section by class** (CSS): `.RZ66Rb button img`
6. **Any Googleusercontent image** (fallback): `//img[contains(@src, 'googleusercontent.com')]`

### Image Validation

Extracted URLs are validated to ensure:
- They're from `googleusercontent.com`
- They use HTTPS
- They're not 1x1 placeholders

### Chromedp Flow

1. Navigate to Google Maps URL
2. Wait for page to load (`body` element)
3. Wait additional time for dynamic content (default: 3s)
4. Try each selector sequentially until success
5. Validate the extracted URL
6. Return result with debug information

## Integration with vCard

The extracted image URL is added to vCards using the `PHOTO` field:

```
PHOTO;VALUE=URI;TYPE=JPEG:https://lh3.googleusercontent.com/p/...
```

This follows the [RFC 6350 vCard specification](https://datatracker.ietf.org/doc/html/rfc6350#section-6.2.4) for the PHOTO field.

## Result Structure

```go
type Result struct {
    ImageURL  string      // The extracted image URL
    Found     bool        // Whether an image was found
    Error     error       // Any error that occurred
    DebugInfo *DebugInfo  // Detailed debugging information
}

type DebugInfo struct {
    Selectors      []SelectorAttempt  // All selector attempts
    PageLoadTime   time.Duration      // Time to load page
    ExtractionTime time.Duration      // Time to extract image
    TotalTime      time.Duration      // Total time
    PageTitle      string             // Page title
    PageURL        string             // Final page URL
}

type SelectorAttempt struct {
    Selector string  // The selector used
    Method   string  // Method name (e.g., "xpath-business-photo-button")
    Success  bool    // Whether it succeeded
    Value    string  // The extracted value
    Error    error   // Any error
}
```

## Troubleshooting

### No image found

If `result.Found` is `false`:

1. Check `result.Error` for the error message
2. Use `extractor.PrintDebugInfo(result)` to see all selector attempts
3. Try increasing the wait time in config
4. Try `DebugVeryVerbose` level to see what's happening

### Wrong image extracted

If the wrong image is extracted (e.g., a thumbnail):

1. Check the `result.ImageURL` format
2. The module filters out 1x1 placeholders automatically
3. Try adjusting the selector order in `extractor.go`

### Timeout errors

If you get timeout errors:

1. Increase `config.Timeout` (default: 30s)
2. Increase `config.WaitTime` for slower connections (default: 3s)
3. Check your internet connection

## Dependencies

- `github.com/chromedp/chromedp`: For headless browser automation

## License

Same as parent project.
