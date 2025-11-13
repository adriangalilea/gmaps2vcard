# URL Normalizer Module

Independent module for normalizing Google Maps URLs to the canonical `/maps/place/` format required for reliable data extraction.

## Features

- **High Debugability**: Multiple debug levels with detailed logging
- **Multi-Strategy Extraction**: Tries 2 non-invasive approaches to extract maps/place URLs from search pages
- **Legitimate Browser Fingerprint**: Uses realistic Chrome/macOS headers and flags for personal use
- **State Assertion**: Explicitly detects and reports URL type at each step
- **CAPTCHA Detection**: Detects Google's bot protection and provides helpful guidance
- **Independent Module**: Can be tested and debugged separately
- **No Click Automation**: Avoids clicking elements (triggers bot detection and doesn't work reliably)

## Usage

### Basic Usage

```go
import "gmaps2vcard/urlnormalizer"

// Create normalizer with default config
normalizer := urlnormalizer.NewNormalizer(nil)

// Normalize any Google Maps URL to /maps/place/ format
result := normalizer.Normalize("https://share.google/xyz123")

if result.Success {
    fmt.Println("Normalized URL:", result.NormalizedURL)
} else {
    fmt.Println("Error:", result.Error)
}
```

### Custom Configuration

```go
config := &urlnormalizer.Config{
    DebugLevel: urlnormalizer.DebugVeryVerbose,
    Timeout:    60 * time.Second,
    WaitTime:   5 * time.Second,
}

normalizer := urlnormalizer.NewNormalizer(config)
result := normalizer.Normalize(url)
```

### Debug Levels

- `DebugNone`: No logging
- `DebugBasic`: Essential logs only (success/failure)
- `DebugVerbose`: Detailed progress (default)
- `DebugVeryVerbose`: Every strategy attempt and page details

### Printing Debug Information

```go
result := normalizer.Normalize(url)
normalizer.PrintDebugInfo(result)
```

This prints:
```
=== URL Normalization Debug Info ===
Input URL: https://share.google/xyz123
Redirected URL: https://www.google.com/search?kgmid=/g/...
Detected Type: search page
Redirect Time: 234ms
Extraction Time: 3.2s
Total Time: 3.45s
Page Title: Business Name - Google Search
Page URL: https://www.google.com/search?...

Search Extraction Attempts:
  ✓ [1] click-address-link
      Value: https://www.google.com/maps/place//data=!4m2!3m1!1s0x...

✓ Result: https://www.google.com/maps/place//data=!4m2!3m1!1s0x...
URL Type: search->place
=====================================
```

## How It Works

### URL Type Detection

The module follows this flow:

1. **Follow Redirects**: Follow all HTTP redirects from share.google links
2. **Detect URL Type**: Parse the final URL and determine its type
3. **Process Based on Type**:
   - **Already `/maps/place/`**: Return immediately (fast path)
   - **Search page (`/search`)**: Extract maps/place link via chromedp
   - **Unknown type**: Fail with helpful error message

### Search Page Extraction Strategy

When a search page is detected, the module tries multiple strategies (in order):

1. **Extract href from address div** (most reliable): Gets `href` attribute from address link - avoids bot detection
2. **Extract data-url** (fallback): Gets the `data-url` attribute - gives minimal URL but works

**Note**: Click-based strategies are intentionally disabled as they trigger Google's bot detection.

### Legitimate Browser Fingerprint

To avoid Google's bot detection while maintaining legitimacy for personal use, the module uses:

**HTTP Headers:**
- User-Agent: Modern Chrome 120 on macOS
- Accept: Full browser accept string with image/avif, image/webp, etc.
- Accept-Language: en-US,en;q=0.9
- Accept-Encoding: gzip, deflate, br
- DNT: 1 (Do Not Track)
- Sec-Fetch-Dest/Mode/Site/User headers (modern browser security)
- Cache-Control: max-age=0

**Chrome Flags:**
- Disable automation indicators (`disable-blink-features: AutomationControlled`)
- Exclude automation switches (`exclude-switches: enable-automation`)
- Standard window size (1920x1080)
- Modern headless mode with realistic features enabled

### CAPTCHA Detection

If Google shows a `/sorry/` CAPTCHA page, the module:
1. Detects it immediately
2. Sets `CaptchaDetected: true` in debug info
3. Returns helpful error with manual workaround instructions

## Result Structure

```go
type Result struct {
    NormalizedURL string    // The final /maps/place/ URL
    URLType       string    // "direct", "search->place", "unknown"
    Success       bool      // Whether normalization succeeded
    Error         error     // Any error that occurred
    DebugInfo     *DebugInfo  // Detailed debugging information
}

type DebugInfo struct {
    InputURL        string
    RedirectedURL   string
    DetectedType    string
    SearchAttempts  []SearchAttempt  // All extraction attempts
    RedirectTime    time.Duration
    ExtractionTime  time.Duration
    TotalTime       time.Duration
    PageTitle       string
    PageURL         string
    CaptchaDetected bool
}

type SearchAttempt struct {
    Method  string  // "click-address-link", "extract-data-url", etc.
    Success bool    // Whether it succeeded
    Value   string  // The extracted URL
    Error   error   // Any error
}
```

## Supported URL Formats

### Input URLs (automatically normalized)

- `https://share.google/<short_code>` → Redirects to search page → Extracts maps/place
- `https://www.google.com/search?kgmid=...` → Extracts maps/place link
- `https://www.google.com/maps/place/...` → Returns immediately (already normalized)
- `https://maps.google.com/...` → Returns immediately

### Output URL Format

Always returns URLs in this format:
```
https://www.google.com/maps/place//data=!4m2!3m1!1s0x<hex_id>:<hex_id>?...
```

This is the canonical format that works reliably with chromedp scraping.

## Troubleshooting

### CAPTCHA/Bot Detection

If you get "Google blocked automated access":

1. This happens when Google detects too many automated requests
2. **Solution**: Open the share.google link in your browser manually
3. Copy the final `google.com/maps/place/` URL from the address bar
4. Use that URL directly with the tool

The module will detect this and provide these exact instructions.

### No maps/place Link Found

If all extraction strategies fail:

1. Use `DebugVeryVerbose` to see what each strategy attempted
2. Check `PrintDebugInfo()` output for the actual page structure
3. Google may have changed their HTML structure
4. The business might not have a Maps place page (rare)

### Timeout Errors

If you get timeout errors:

1. Increase `config.Timeout` (default: 45s)
2. Increase `config.WaitTime` for slower connections (default: 3s)
3. Check your internet connection
4. Try the URL manually to verify it works

## Dependencies

- `github.com/chromedp/chromedp`: For headless browser automation
- `net/http`: For following redirects
- `net/url`: For URL parsing

## License

Same as parent project.
