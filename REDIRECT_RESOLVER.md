# Google Maps Redirect Resolver (Go)

This is a focused Go tool designed to solve **ONLY** the redirect resolution problem for Google Maps share links.

## Problem

`share.google` links block Python's requests library with 403 errors. Google uses:
- Bot detection on share links
- JavaScript redirects
- Meta refresh tags
- Cookie tracking

## Solution

This Go program implements a legitimate browser-like HTTP client that:

1. **Proper Headers**: Sets comprehensive Chrome-like headers
   - User-Agent, Accept, Accept-Language, Accept-Encoding
   - Sec-Fetch-* headers for modern browser behavior
   - Referer header for share.google links

2. **Cookie Handling**: Uses `cookiejar` with public suffix list
   - Maintains cookies across redirects
   - Proper domain matching

3. **Redirect Following**: Custom redirect handler
   - Follows up to 10 redirects
   - Preserves headers across redirects
   - Handles both HTTP 301/302 and meta refresh

4. **JavaScript Redirect Detection**: Parses HTML for:
   - `<meta http-equiv="refresh">` tags
   - `window.location = "..."` patterns
   - Google's specific redirect formats

## Usage

### Build

```bash
go build -o redirect_resolver redirect_resolver.go
```

### Run

```bash
./redirect_resolver "https://share.google/w4UZTre3NvPyC3b3Q"
```

Output: The final Google Maps URL

### Use from Python

```python
import subprocess

def follow_redirects_go(url: str) -> str:
    """Use Go redirect resolver instead of Python requests."""
    result = subprocess.run(
        ['./redirect_resolver', url],
        capture_output=True,
        text=True,
        timeout=30
    )
    if result.returncode != 0:
        raise RuntimeError(f"Redirect failed: {result.stderr}")
    return result.stdout.strip()
```

## Why Go?

1. **Better HTTP Client**: Go's `net/http` client is more sophisticated
2. **No Bot Detection**: Go binaries are less likely to be fingerprinted
3. **Proper Cookie Handling**: Built-in cookie jar with PSL support
4. **Performance**: Compiled binary is faster than Python
5. **Legitimate Requests**: All measures to appear as a real browser

## Testing

```bash
# Test with a real share.google link
./redirect_resolver "https://share.google/your_link_here"

# Should output something like:
# https://www.google.com/maps/place/Business+Name/@lat,lng,zoom...
```

## Dependencies

- Go 1.21+
- `golang.org/x/net` (for publicsuffix list)

## Integration

This tool is designed to be called from the main Python script when redirect resolution fails:

1. Python tries normal requests
2. If 403/429/timeout, fall back to Go resolver
3. Go resolver returns final URL
4. Python continues with extraction

## Limitations

- Still requires internet access
- Google may eventually block Go user agents too
- JavaScript-heavy redirects may need browser automation
- No JavaScript execution (only regex parsing)

If this fails, the only remaining option is full browser automation (Playwright).
