# gmaps2vcard

Convert Google Maps business links to vCard contacts - perfect for quickly saving business information to your iPhone, iCloud, or any contacts app.

## Goal

**Input:** `https://share.google/w4UZTre3NvPyC3b3Q`
**Output:** `.vcf` file with business contact (name, address, phone, website, photo, hours) → ready to import to iCloud contacts

## Quick Start

```bash
# Clone and build
git clone https://github.com/adriangalilea/gmaps2vcard
cd gmaps2vcard
go build

# Use with any Google Maps URL
./gmaps2vcard "https://share.google/w4UZTre3NvPyC3b3Q"
./gmaps2vcard "https://www.google.com/maps/place/Business+Name/@..."
```

**What you get:**
- ✅ Complete business data (name, address, phone, website, coordinates, hours, photo)
- ✅ Business photo embedded in vCard (works with Apple Contacts/Finder preview)
- ✅ Smart schedule parsing (Spanish/English, consolidated day ranges)
- ✅ Intelligent URL normalization with bot detection avoidance
- ✅ Single standalone binary
- ✅ No API keys required
- ✅ Uses chromedp for reliable scraping
- ✅ Works with share.google links AND full Maps URLs (direct URLs most reliable)

## How It Works

1. **URL Validation** (`main.go`) - Validates Google Maps/share.google URLs

2. **Unified Scraping** (`scraper/`) - ONE chromedp session does EVERYTHING:
   - Follows HTTP redirects with legitimate browser headers
   - Detects URL type (direct maps/place, search page, or unknown)
   - For search pages: extracts the maps/place link (same session)
   - Navigates to maps/place page
   - Extracts: name, address, phone, website, coordinates
   - Multi-strategy hours extraction: tries multiple button selectors → extracts from table structure → fallback to body text
   - Extracts image URL (tries multiple selectors with timeouts)
   - Cleans aria-label prefixes automatically

3. **Schedule Parsing** (`schedule/`) - Pure text parser:
   - Spanish/English → normalized English day names
   - Spanish time format ("De 8:00 a 13:00") → standard format ("08:00-13:00")
   - Consolidates consecutive days (Mon-Fri vs 5 separate entries)
   - Output: "Mon-Fri 08:00-13:00, 15:00-18:00; Sat-Sun Closed"

4. **Image Download** (`imageextractor/`) - Simple HTTP download + base64 encoding

5. **vCard Generation** (`main.go`) - Creates standard vCard 3.0 format

6. **File Output** (`main.go`) - Saves as `BusinessName.vcf`

## Architecture

Clean, minimal design:

**Modules:**
- **`scraper/`** - ONE chromedp session for URL normalization + ALL data extraction
- **`schedule/`** - Pure text parser (no dependencies)
- **`imageextractor/`** - Pure HTTP download function (no chromedp)
- **`main.go`** - Clean orchestration (70 lines, zero business logic)

**Key Design Principles:**
- ONE chromedp session total (efficient)
- No nested abstractions
- Code is self-documenting

## Supported URL Formats

- `https://share.google/<short_code>` (share links from mobile)
- `https://maps.google.com/...` (classic Google Maps)
- `https://www.google.com/maps/...` (modern Google Maps)
- `https://goo.gl/maps/...` (shortened links)

## Output

The tool generates a `.vcf` file with:
- Full name (FN)
- Organization (ORG)
- Address (ADR)
- Phone (TEL)
- Website (URL)
- Geographic coordinates (GEO)
- Business photo (PHOTO) - base64-encoded image
- Business hours (NOTE) - clean formatted schedule

Import this file to:
- iPhone Contacts app
- iCloud Contacts
- Google Contacts
- Outlook
- Any app that supports vCard format

## Examples

```bash
# From share.google link
./gmaps2vcard "https://share.google/w4UZTre3NvPyC3b3Q"

# From full Maps URL
./gmaps2vcard "https://www.google.com/maps/place/Eiffel+Tower/@48.8583701,2.2944813,17z/..."

# From shortened link
./gmaps2vcard "https://goo.gl/maps/xyz123"
```
