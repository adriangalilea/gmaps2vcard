package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// ResolveRedirect follows all redirects and returns the final destination URL
func ResolveRedirect(inputURL string) (string, error) {
	// Create a cookie jar to handle cookies across redirects
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create custom transport with TLS config that mimics Chrome
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: false,
			// Chrome-like cipher suites
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
		DisableCompression: false,
		IdleConnTimeout:    90 * time.Second,
	}

	// Create HTTP client with realistic browser configuration
	client := &http.Client{
		Jar:       jar,
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Copy headers to redirect requests
			for key, val := range via[0].Header {
				req.Header[key] = val
			}
			return nil
		},
	}

	// Create the request with comprehensive browser-like headers
	req, err := http.NewRequest("GET", inputURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set realistic browser headers that mimic Chrome on Windows
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Cache-Control", "max-age=0")

	// If this is a share.google link, add referer
	if strings.Contains(inputURL, "share.google") {
		req.Header.Set("Referer", "https://www.google.com/")
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got blocked
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return "", fmt.Errorf("access denied (HTTP %d) - Google blocked the request", resp.StatusCode)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 301 && resp.StatusCode != 302 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	finalURL := resp.Request.URL.String()

	// Check if the response contains a JavaScript redirect
	// Google sometimes uses meta refresh or JavaScript redirects
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			bodyStr := string(body)

			// Check for meta refresh tag
			metaRefreshRegex := regexp.MustCompile(`<meta[^>]*http-equiv=["']refresh["'][^>]*content=["'][^"']*url=([^"']+)["']`)
			if matches := metaRefreshRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
				redirectURL := matches[1]
				// Handle relative URLs
				if !strings.HasPrefix(redirectURL, "http") {
					base, _ := url.Parse(finalURL)
					if rel, err := url.Parse(redirectURL); err == nil {
						redirectURL = base.ResolveReference(rel).String()
					}
				}
				// Follow the meta refresh redirect
				return ResolveRedirect(redirectURL)
			}

			// Check for window.location JavaScript redirects
			jsRedirectRegex := regexp.MustCompile(`window\.location(?:\.href)?\s*=\s*["']([^"']+)["']`)
			if matches := jsRedirectRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
				redirectURL := matches[1]
				// Handle relative URLs
				if !strings.HasPrefix(redirectURL, "http") {
					base, _ := url.Parse(finalURL)
					if rel, err := url.Parse(redirectURL); err == nil {
						redirectURL = base.ResolveReference(rel).String()
					}
				}
				// Follow the JavaScript redirect
				return ResolveRedirect(redirectURL)
			}

			// Check for Google's specific redirect format
			googleRedirectRegex := regexp.MustCompile(`var\s+url\s*=\s*'([^']+)'`)
			if matches := googleRedirectRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
				redirectURL := matches[1]
				redirectURL = strings.ReplaceAll(redirectURL, `\x3d`, "=")
				redirectURL = strings.ReplaceAll(redirectURL, `\x26`, "&")
				if strings.HasPrefix(redirectURL, "http") {
					return ResolveRedirect(redirectURL)
				}
			}
		}
	}

	return finalURL, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <google-share-url>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s 'https://share.google/w4UZTre3NvPyC3b3Q'\n", os.Args[0])
		os.Exit(1)
	}

	inputURL := os.Args[1]

	// Validate it's a Google URL
	if !strings.Contains(inputURL, "google.com") && !strings.Contains(inputURL, "share.google") && !strings.Contains(inputURL, "goo.gl") {
		fmt.Fprintf(os.Stderr, "Error: URL must be a Google Maps or share.google link\n")
		os.Exit(1)
	}

	// Resolve the redirect
	finalURL, err := ResolveRedirect(inputURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving redirect: %v\n", err)
		os.Exit(1)
	}

	// Output just the final URL (for easy piping/parsing)
	fmt.Println(finalURL)
}
