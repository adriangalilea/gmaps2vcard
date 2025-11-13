# gmaps2vcard

Convert Google Maps business links to vCard contacts.

## Goal

Input: `https://share.google/w4UZTre3NvPyC3b3Q`
Output: `.vcf` file with business contact (name, address, phone, website) → automatically imported to iCloud contacts

## Usage

```bash
uv run gmaps2vcard <google-maps-share-link>
```

## Supported URL Formats

- `https://share.google/<short_code>`
- `https://maps.google.com/...`
- `https://www.google.com/maps/...`
- `https://goo.gl/maps/...`
