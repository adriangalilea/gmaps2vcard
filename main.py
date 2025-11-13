import sys
from urllib.parse import urlparse


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


def main():
    if len(sys.argv) != 2:
        print("Usage: gmaps2vcard <google-maps-url>", file=sys.stderr)
        sys.exit(1)

    url = sys.argv[1]

    try:
        validated_url = validate_google_maps_url(url)
        print(f"Valid URL: {validated_url}")
        # TODO: Extract business data
        # TODO: Generate vCard
        # TODO: Import to iCloud
    except AssertionError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
