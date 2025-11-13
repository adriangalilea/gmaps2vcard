#!/usr/bin/env python3
import sys
import re
import json
import os
from urllib.parse import urlparse, unquote, parse_qs
from typing import Optional, Dict

import requests
import vobject


def validate_google_maps_url(url: str) -> str:
    """
    Validate and normalize Google Maps share URL.

    Accepts:
    - https://share.google/<short_code>
    - https://maps.google.com/...
    - https://www.google.com/maps/...
    - https://goo.gl/maps/...

    Returns the original URL if valid.
    Raises ValueError if invalid.
    """
    parsed = urlparse(url)

    assert parsed.scheme in ("http", "https"), "URL must use http or https"
    assert parsed.netloc, "URL must have a valid domain"

    valid_domains = (
        "share.google",
        "maps.google.com",
        "www.google.com",
        "google.com",
        "goo.gl",
    )

    assert any(parsed.netloc.endswith(domain) for domain in valid_domains), \
        f"URL must be from a Google Maps domain, got: {parsed.netloc}"

    return url


def follow_redirects(url: str) -> str:
    """
    Follow URL redirects and return the final destination URL.
    """
    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
        }
        response = requests.head(url, allow_redirects=True, timeout=10, headers=headers)
        return response.url
    except requests.RequestException:
        # If HEAD fails, try GET
        try:
            response = requests.get(url, allow_redirects=True, timeout=10, headers=headers)
            return response.url
        except requests.RequestException as e:
            raise RuntimeError(f"Failed to follow redirect: {e}")


def extract_place_id_from_url(url: str) -> Optional[str]:
    """
    Extract Google Place ID from Maps URL.
    Place IDs can be in various formats in the URL.
    """
    # Try to find place_id in query parameters
    parsed = urlparse(url)
    params = parse_qs(parsed.query)
    if 'place_id' in params:
        return params['place_id'][0]

    # Try to find in data parameter (for some share links)
    if 'data' in params:
        data_str = params['data'][0]
        match = re.search(r'!1s([^!]+)', data_str)
        if match:
            return match.group(1)

    # Try CID (Customer ID) format
    match = re.search(r'!1s0x[0-9a-f]+:0x[0-9a-f]+', url)
    if match:
        return match.group(0).replace('!1s', '')

    return None


def extract_place_name_from_url(url: str) -> Optional[str]:
    """
    Extract place name from Google Maps URL.
    Example: https://www.google.com/maps/place/Business+Name/@lat,lng...
    """
    match = re.search(r'/place/([^/@?]+)', url)
    if match:
        return unquote(match.group(1)).replace('+', ' ')
    return None


def extract_coordinates_from_url(url: str) -> tuple[Optional[str], Optional[str]]:
    """
    Extract latitude and longitude from Google Maps URL.
    """
    # Format: @lat,lng,zoom
    match = re.search(r'@(-?\d+\.\d+),(-?\d+\.\d+)', url)
    if match:
        return match.group(1), match.group(2)
    return None, None


def get_place_details_from_api(place_id: Optional[str], place_name: Optional[str],
                                 api_key: str) -> Optional[Dict]:
    """
    Get place details using Google Places API.
    """
    if not api_key:
        return None

    base_url = "https://maps.googleapis.com/maps/api/place"

    # If we have a place_id, use it directly
    if place_id:
        url = f"{base_url}/details/json"
        params = {
            'place_id': place_id,
            'fields': 'name,formatted_address,formatted_phone_number,website,geometry',
            'key': api_key
        }
    # Otherwise, search by name
    elif place_name:
        # First, find the place
        search_url = f"{base_url}/findplacefromtext/json"
        search_params = {
            'input': place_name,
            'inputtype': 'textquery',
            'fields': 'place_id',
            'key': api_key
        }

        try:
            response = requests.get(search_url, params=search_params, timeout=10)
            data = response.json()

            if data.get('status') != 'OK' or not data.get('candidates'):
                return None

            place_id = data['candidates'][0]['place_id']

            # Now get details
            url = f"{base_url}/details/json"
            params = {
                'place_id': place_id,
                'fields': 'name,formatted_address,formatted_phone_number,website,geometry',
                'key': api_key
            }
        except requests.RequestException:
            return None
    else:
        return None

    try:
        response = requests.get(url, params=params, timeout=10)
        data = response.json()

        if data.get('status') != 'OK':
            return None

        result = data.get('result', {})

        business_data = {
            'name': result.get('name'),
            'address': result.get('formatted_address'),
            'phone': result.get('formatted_phone_number'),
            'website': result.get('website'),
            'latitude': None,
            'longitude': None,
        }

        location = result.get('geometry', {}).get('location', {})
        if location:
            business_data['latitude'] = str(location.get('lat'))
            business_data['longitude'] = str(location.get('lng'))

        return business_data

    except requests.RequestException as e:
        print(f"Warning: API request failed: {e}", file=sys.stderr)
        return None


def extract_business_data_playwright(url: str) -> Dict:
    """
    Extract business data using Playwright browser automation.
    This is more reliable than simple scraping but requires browser dependencies.
    """
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("Error: Playwright not installed. Install with: uv pip install playwright && playwright install chromium", file=sys.stderr)
        return {'name': None, 'address': None, 'phone': None, 'website': None, 'latitude': None, 'longitude': None}

    business_data = {
        'name': None,
        'address': None,
        'phone': None,
        'website': None,
        'latitude': None,
        'longitude': None,
    }

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        try:
            page.goto(url, wait_until='networkidle', timeout=30000)
            page.wait_for_timeout(2000)  # Wait for dynamic content

            # Try to extract business name
            try:
                business_data['name'] = page.locator('h1').first.text_content()
            except:
                pass

            # Try to extract address
            try:
                address_button = page.locator('button[data-item-id="address"]').first
                business_data['address'] = address_button.get_attribute('aria-label')
            except:
                pass

            # Try to extract phone
            try:
                phone_button = page.locator('button[data-item-id*="phone"]').first
                business_data['phone'] = phone_button.get_attribute('aria-label')
            except:
                pass

            # Try to extract website
            try:
                website_link = page.locator('a[data-item-id="authority"]').first
                business_data['website'] = website_link.get_attribute('href')
            except:
                pass

            # Extract coordinates from URL
            final_url = page.url
            lat, lng = extract_coordinates_from_url(final_url)
            business_data['latitude'] = lat
            business_data['longitude'] = lng

        except Exception as e:
            print(f"Warning: Playwright extraction failed: {e}", file=sys.stderr)
        finally:
            browser.close()

    return business_data


def extract_business_data_basic(url: str) -> Dict:
    """
    Extract basic business information from URL structure.
    This is the simplest method but provides limited data.
    """
    business_data = {
        'name': extract_place_name_from_url(url),
        'address': None,
        'phone': None,
        'website': None,
        'latitude': None,
        'longitude': None,
    }

    lat, lng = extract_coordinates_from_url(url)
    business_data['latitude'] = lat
    business_data['longitude'] = lng

    return business_data


def generate_vcard(business_data: Dict) -> str:
    """
    Generate vCard from business data.
    """
    card = vobject.vCard()

    # Required fields
    name = business_data.get('name', 'Unknown Business')
    card.add('fn')
    card.fn.value = name

    card.add('n')
    # For organizations, put the name in the organization field of 'n'
    card.n.value = vobject.vcard.Name(family='', given='', additional='', prefix='', suffix='')

    # Add organization
    card.add('org')
    card.org.value = [name]

    # Add address if available
    address = business_data.get('address')
    if address:
        adr = card.add('adr')
        adr.type_param = 'WORK'
        # vCard ADR format: PO Box;Extended;Street;City;State;Postal;Country
        # Create Address with all fields as empty strings except street
        adr.value = vobject.vcard.Address()
        adr.value.box = ''
        adr.value.extended = ''
        adr.value.street = str(address)
        adr.value.city = ''
        adr.value.region = ''
        adr.value.code = ''
        adr.value.country = ''

    # Add phone if available
    phone = business_data.get('phone')
    if phone:
        tel = card.add('tel')
        tel.type_param = 'WORK'
        tel.value = str(phone)

    # Add website if available
    website = business_data.get('website')
    if website:
        url = card.add('url')
        url.value = str(website)
        url.type_param = 'WORK'

    # Add geo coordinates if available
    if business_data.get('latitude') and business_data.get('longitude'):
        geo = card.add('geo')
        geo.value = f"{business_data['latitude']};{business_data['longitude']}"

    return card.serialize()


def print_business_data(business_data: Dict):
    """Print extracted business data."""
    print(f"\nExtracted information:")
    print(f"  Name: {business_data['name'] or '(not found)'}")
    print(f"  Address: {business_data['address'] or '(not found)'}")
    print(f"  Phone: {business_data['phone'] or '(not found)'}")
    print(f"  Website: {business_data['website'] or '(not found)'}")
    if business_data['latitude'] and business_data['longitude']:
        print(f"  Location: {business_data['latitude']}, {business_data['longitude']}")


def main():
    if len(sys.argv) < 2:
        print("Usage: gmaps2vcard <google-maps-url> [--method api|playwright|basic]", file=sys.stderr)
        print("\nMethods:", file=sys.stderr)
        print("  api        - Use Google Places API (requires GOOGLE_PLACES_API_KEY env var) [RECOMMENDED]", file=sys.stderr)
        print("  playwright - Use browser automation (requires: uv pip install playwright)", file=sys.stderr)
        print("  basic      - Extract from URL only (limited data, no API key needed)", file=sys.stderr)
        print("\nExample:", file=sys.stderr)
        print("  export GOOGLE_PLACES_API_KEY='your-api-key'", file=sys.stderr)
        print("  gmaps2vcard 'https://share.google/w4UZTre3NvPyC3b3Q' --method api", file=sys.stderr)
        sys.exit(1)

    url = sys.argv[1]

    # Determine method
    method = 'basic'  # default
    if len(sys.argv) > 2:
        if sys.argv[2] == '--method' and len(sys.argv) > 3:
            method = sys.argv[3]

    # Check for API key in environment
    api_key = os.getenv('GOOGLE_PLACES_API_KEY')
    if method == 'api' and not api_key:
        print("Error: GOOGLE_PLACES_API_KEY environment variable not set", file=sys.stderr)
        print("Get an API key from: https://console.cloud.google.com/apis/credentials", file=sys.stderr)
        print("Or use: gmaps2vcard <url> --method playwright", file=sys.stderr)
        sys.exit(1)

    try:
        # Step 1: Validate URL
        validated_url = validate_google_maps_url(url)
        print(f"✓ Valid Google Maps URL")

        # Step 2: Follow redirects to get final URL
        print(f"→ Following redirects...")
        try:
            final_url = follow_redirects(validated_url)
            if final_url != validated_url:
                print(f"✓ Redirected to: {final_url[:80]}...")
            else:
                final_url = validated_url
        except RuntimeError as e:
            print(f"⚠ Could not follow redirects: {e}", file=sys.stderr)
            if 'share.google' in validated_url and method != 'playwright':
                print(f"⚠ share.google links block programmatic access!", file=sys.stderr)
                print(f"  Try: --method playwright", file=sys.stderr)
                print(f"  Or: Open the link in browser and copy the full Google Maps URL", file=sys.stderr)
            final_url = validated_url

        # Step 3: Extract business data using chosen method
        print(f"→ Extracting business data using method: {method}...")

        business_data = None

        if method == 'api':
            place_id = extract_place_id_from_url(final_url)
            place_name = extract_place_name_from_url(final_url)

            if place_id:
                print(f"  Found Place ID: {place_id}")
            if place_name:
                print(f"  Found Place Name: {place_name}")

            business_data = get_place_details_from_api(place_id, place_name, api_key)

            if not business_data or not business_data.get('name'):
                print("⚠ API method failed, falling back to basic extraction", file=sys.stderr)
                business_data = extract_business_data_basic(final_url)

        elif method == 'playwright':
            business_data = extract_business_data_playwright(final_url)

            if not business_data.get('name'):
                print("⚠ Playwright method failed, falling back to basic extraction", file=sys.stderr)
                business_data = extract_business_data_basic(final_url)

        else:  # basic
            business_data = extract_business_data_basic(final_url)

        print_business_data(business_data)

        if not business_data.get('name'):
            print("\n⚠ Warning: Could not extract business name. vCard may be incomplete.", file=sys.stderr)
            print("Try using: --method api (recommended) or --method playwright", file=sys.stderr)

        # Step 4: Generate vCard
        print(f"\n→ Generating vCard...")
        vcard_content = generate_vcard(business_data)

        # Step 5: Save to file
        safe_name = (business_data.get('name') or 'business').replace('/', '-')
        filename = f"{safe_name}.vcf"
        with open(filename, 'w', encoding='utf-8') as f:
            f.write(vcard_content)

        print(f"✓ vCard saved to: {filename}")
        print(f"\nYou can now import this file to your contacts app or iCloud.")

    except AssertionError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
