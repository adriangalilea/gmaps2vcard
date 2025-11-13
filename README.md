# gmaps2vcard

Convert Google Maps business links to vCard contacts - perfect for quickly saving business information to your iPhone, iCloud, or any contacts app.

## Goal

**Input:** `https://share.google/w4UZTre3NvPyC3b3Q`
**Output:** `.vcf` file with business contact (name, address, phone, website) → ready to import to iCloud contacts

## Installation

```bash
git clone <repository>
cd gmaps2vcard
```

Dependencies are managed via `uv` and will be automatically installed on first run.

## Usage

gmaps2vcard supports three extraction methods, each with different trade-offs:

### Method 1: Google Places API (Recommended)

**Best for:** Most reliable and complete data
**Requires:** Free Google Places API key (includes $200/month credit)

```bash
# Get your free API key from: https://console.cloud.google.com/apis/credentials
export GOOGLE_PLACES_API_KEY='your-api-key-here'

uv run main.py "https://share.google/w4UZTre3NvPyC3b3Q" --method api
```

**Pros:**
- ✅ Most reliable and accurate
- ✅ Complete business info (name, address, phone, website)
- ✅ Official Google API
- ✅ Free for personal use (<$200/month usage)

**Cons:**
- ❌ Requires API key setup (5 minutes)

### Method 2: Browser Automation (Playwright)

**Best for:** When you don't want to set up an API key
**Requires:** Playwright installation

```bash
# First-time setup
uv pip install playwright
playwright install chromium

# Usage
uv run main.py "https://share.google/w4UZTre3NvPyC3b3Q" --method playwright
```

**Pros:**
- ✅ No API key needed
- ✅ Can extract most business data
- ✅ Free

**Cons:**
- ❌ Requires browser installation
- ❌ Slower than API
- ❌ May break if Google changes their layout

### Method 3: Basic (URL Parsing)

**Best for:** Quick extraction with minimal setup
**Requires:** Nothing extra

```bash
uv run main.py "https://share.google/w4UZTre3NvPyC3b3Q" --method basic
```

**Pros:**
- ✅ No setup required
- ✅ Fast
- ✅ Works out of the box

**Cons:**
- ❌ Limited data (only name and coordinates from URL)
- ❌ May not get phone/address/website

## Supported URL Formats

All methods support these Google Maps URL formats:

- `https://share.google/<short_code>` (share links from mobile)
- `https://maps.google.com/...` (classic Google Maps)
- `https://www.google.com/maps/...` (modern Google Maps)
- `https://goo.gl/maps/...` (shortened links)

## How to Get a Google Places API Key

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create a new project (or select existing)
3. Enable "Places API"
4. Create credentials → API Key
5. Copy the API key
6. Set it as environment variable: `export GOOGLE_PLACES_API_KEY='your-key'`

**Cost:** Free for personal use. Google provides $200/month free credit, which covers thousands of requests.

## Output

The tool generates a `.vcf` file with:
- Full name (FN)
- Organization (ORG)
- Address (ADR) - if available
- Phone (TEL) - if available
- Website (URL) - if available
- Geographic coordinates (GEO) - if available

Import this file to:
- iPhone Contacts app
- iCloud Contacts
- Google Contacts
- Outlook
- Any app that supports vCard format

## Comparison of Methods

| Feature | API | Playwright | Basic |
|---------|-----|------------|-------|
| Name | ✅ | ✅ | ✅ |
| Address | ✅ | ✅ | ❌ |
| Phone | ✅ | ✅ | ❌ |
| Website | ✅ | ✅ | ❌ |
| Coordinates | ✅ | ✅ | ✅ |
| Setup Time | 5 min | 2 min | 0 min |
| Reliability | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ |
| Speed | Fast | Slow | Fast |

## Troubleshooting

**Q: I get "Could not extract business name"**
A: The basic method has limitations. Try `--method api` or `--method playwright` instead.

**Q: The share.google link doesn't redirect properly**
A: Some share links require JavaScript to redirect. Use `--method playwright` to handle these.

**Q: API returns "REQUEST_DENIED"**
A: Make sure you've enabled the Places API in your Google Cloud Console and your API key is valid.

**Q: Playwright fails to extract data**
A: Google Maps may have changed their layout. Open an issue and we'll update the selectors.

## Examples

```bash
# Using API (recommended)
export GOOGLE_PLACES_API_KEY='AIza...'
uv run main.py "https://maps.google.com/maps?q=eiffel+tower" --method api

# Using Playwright
uv run main.py "https://goo.gl/maps/xyz123" --method playwright

# Using basic (default)
uv run main.py "https://share.google/abc123"
```

## Research & Implementation Details

For a detailed analysis of the different extraction methods and why these approaches were chosen, see the commit history and initial research in this repository.
