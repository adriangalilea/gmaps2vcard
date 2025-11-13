# Research: Google Maps to vCard Implementation Options

## Problem Statement

Convert Google Maps share links to vCard files containing business contact information (name, address, phone, website) for import into contacts apps (iPhone, iCloud, etc.).

## Research Findings

### Option 1: Google Places API ⭐ RECOMMENDED

**How it works:**
- Use official Google Places API
- Extract place ID or name from URL
- Call API to get structured business data
- Cost: FREE for personal use ($200/month credit)

**Implementation:**
```python
# Extract place_id from URL or search by name
# Call: https://maps.googleapis.com/maps/api/place/details/json
# Returns: name, address, phone, website, coordinates
```

**Pros:**
- ✅ Official, stable, documented API
- ✅ Reliable structured data
- ✅ Legal and ToS-compliant
- ✅ Free for <$200/month usage (thousands of requests)

**Cons:**
- ❌ Requires API key setup (~5 minutes)
- ❌ Requires credit card for Google Cloud account

**Cost Analysis:**
- Basic data (name, address): ~$17 per 1,000 requests
- Contact data (phone, website): +$3-4 per 1,000 requests
- Monthly free credit: $200 (covers ~10,000 requests)
- For personal use: Effectively FREE

---

### Option 2: Browser Automation (Playwright/Selenium)

**How it works:**
- Launch headless browser
- Navigate to Google Maps URL
- Wait for JavaScript to render
- Extract data from DOM elements

**Implementation:**
```python
from playwright.sync_api import sync_playwright
# Launch browser, navigate, extract h1 for name
# Find data-item-id attributes for address, phone
# Parse website from links
```

**Pros:**
- ✅ No API key needed
- ✅ Can access all visible data
- ✅ Free

**Cons:**
- ❌ Requires browser installation (~170MB for Chromium)
- ❌ Fragile (breaks when Google changes DOM)
- ❌ Slower (2-5 seconds per extraction)
- ❌ Against Google ToS (technically scraping)
- ❌ Anti-bot protection (CAPTCHA risk)

**Risk Level:** Medium - may break with Google Updates

---

### Option 3: Lightweight Scraping (requests + BeautifulSoup)

**How it works:**
- Make HTTP request to Maps URL
- Parse HTML/JSON-LD for business data
- Extract from meta tags and structured data

**Implementation:**
```python
import requests
from bs4 import BeautifulSoup
# Parse OpenGraph tags, JSON-LD schema
# Extract from <script type="application/ld+json">
```

**Pros:**
- ✅ Simple, lightweight
- ✅ Fast (<1 second)
- ✅ No browser dependencies

**Cons:**
- ❌ Google returns 403 for most requests
- ❌ Missing JavaScript-rendered content
- ❌ Against Google ToS
- ❌ Very fragile

**Status:** NOT VIABLE - Google blocks programmatic access

---

### Option 4: URL Parsing Only

**How it works:**
- Extract information directly from URL structure
- No HTTP requests needed

**Implementation:**
```python
# Extract from /place/Business+Name/@lat,lng,zoom
# Regex: /place/([^/@?]+)  → business name
# Regex: @(-?\d+\.\d+),(-?\d+\.\d+) → coordinates
```

**Pros:**
- ✅ Zero setup, works instantly
- ✅ Fast, reliable
- ✅ No API key or browser needed

**Cons:**
- ❌ Limited data (name + coordinates only)
- ❌ No address, phone, or website

**Use Case:** Quick extraction when only basic info needed

---

## Critical Discovery: share.google Links

**Problem:** `https://share.google/` short links **block programmatic access**

```bash
$ curl https://share.google/w4UZTre3NvPyC3b3Q
Access denied (403)
```

**Why:** Google uses JavaScript redirects and bot detection on share links

**Solutions:**
1. Use Playwright to handle as real browser
2. Manually open link in browser, copy full URL
3. Use Google's URL shortener API (deprecated)

**Impact:** share.google links REQUIRE browser automation or manual intervention

---

## Recommended Implementation Strategy

### Hybrid Multi-Method Approach

Implement all three viable methods with automatic fallback:

1. **Primary: Google Places API**
   - Best reliability and data quality
   - User sets `GOOGLE_PLACES_API_KEY` env var
   - Extract place_id or name from URL → API call

2. **Secondary: Playwright**
   - Fallback for users without API key
   - Required for share.google links
   - Browser automation extracts full data

3. **Tertiary: Basic (URL parsing)**
   - Last resort, default method
   - Works instantly but limited data
   - Good for quick name + coordinates

### Decision Matrix

| Scenario | Method | Reason |
|----------|--------|--------|
| User has API key | API | Best quality, fast |
| No API key | Playwright | Full data without API |
| Quick extraction | Basic | Instant, no setup |
| share.google link | Playwright | Only method that works |

---

## vCard Generation

**Library:** `vobject` (Python)

**Why vobject:**
- ✅ Official vCard 3.0/4.0 support
- ✅ Active maintenance
- ✅ Simple API
- ✅ Handles encoding automatically

**Alternative Considered:** Manual string formatting
- ❌ Error-prone
- ❌ Encoding issues
- ❌ vCard spec complexity

**Implementation:**
```python
import vobject
card = vobject.vCard()
card.add('fn').value = name
card.add('org').value = [name]
card.add('tel').value = phone  # if available
card.add('adr')  # structured address
card.add('geo').value = f"{lat};{lng}"
```

---

## Testing Results

### Test Case 1: Direct Google Maps URL
```
Input: https://www.google.com/maps/place/Eiffel+Tower/@48.8583701,2.2944813,17z
Method: Basic
Result: ✅ SUCCESS
Output:
  Name: Eiffel Tower
  Coordinates: 48.8583701, 2.2944813
  vCard: Valid
```

### Test Case 2: share.google Link
```
Input: https://share.google/w4UZTre3NvPyC3b3Q
Method: Basic/HTTP
Result: ❌ FAILED - 403 Access Denied
Method: Playwright
Result: ⏳ REQUIRES BROWSER (to be tested)
```

---

## Security & Legal Considerations

### Google ToS Compliance
- ✅ Places API: Fully compliant
- ⚠️ Playwright: Gray area (scraping)
- ❌ requests scraping: Violates ToS

**Recommendation:** Prefer API method, clearly document Playwright risks

### Rate Limiting
- API: 500 requests/sec limit
- Scraping: Risk of IP ban
- Basic: No limits (offline parsing)

### Privacy
- API: Google logs requests
- Playwright: Appears as normal user
- Basic: No external requests

---

## Cost Analysis (Monthly)

### Scenario: Personal Use (50 lookups/month)

| Method | Setup Cost | Running Cost | Total |
|--------|-----------|--------------|-------|
| API | $0 (free tier) | $0.85 (within free credit) | $0 |
| Playwright | 5 min time | $0 | $0 |
| Basic | 0 min | $0 | $0 |

### Scenario: Power User (1,000 lookups/month)

| Method | Setup Cost | Running Cost | Total |
|--------|-----------|--------------|-------|
| API | $0 | $17 (within $200 credit) | $0 |
| Playwright | 5 min + compute | $0 | $0 |
| Basic | 0 min | $0 | $0 |

**Conclusion:** All methods are free for reasonable personal use

---

## Final Recommendation

**Implement all three methods** with this priority:

1. **Basic** (default) - Works instantly for simple cases
2. **API** (opt-in with env var) - Best quality when configured
3. **Playwright** (opt-in) - Handles edge cases and share links

**User Experience:**
```bash
# Just works (limited data)
gmaps2vcard <url>

# Best quality (one-time setup)
export GOOGLE_PLACES_API_KEY='...'
gmaps2vcard <url> --method api

# For share.google links
gmaps2vcard <url> --method playwright
```

This balances:
- ✅ Ease of use (works out of box)
- ✅ Quality (API available when needed)
- ✅ Flexibility (handles all URL types)
- ✅ Transparency (user chooses trade-offs)
