package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// ValidateRedirectURL validates that a redirect URL is safe to use.
// It checks that:
//   - The URL is properly formatted
//   - The scheme is either http or https
//   - The domain is in the trusted domains list (exact match or subdomain)
//
// Returns nil if the URL is valid and trusted, otherwise returns an error
// describing why the validation failed.
func ValidateRedirectURL(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err.Error())
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http and https are allowed")
	}

	domain := strings.ToLower(parsedURL.Hostname())

	for _, trustedDomain := range constant.TrustedRedirectDomains {
		if domain == trustedDomain || strings.HasSuffix(domain, "."+trustedDomain) {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in the trusted domains list", domain)
}

// ValidatePaymentRedirectURL accepts the normal trusted HTTP(S) redirects and
// exact custom-scheme callback bases registered for desktop clients.
func ValidatePaymentRedirectURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid payment redirect URL")
	}
	if parsedURL.User != nil {
		return fmt.Errorf("payment redirect URL must not contain credentials")
	}
	if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
		return ValidateRedirectURL(rawURL)
	}
	if parsedURL.Scheme == "blob" || parsedURL.Scheme == "data" || parsedURL.Scheme == "file" || parsedURL.Scheme == "ftp" || parsedURL.Scheme == "javascript" || parsedURL.Scheme == "mailto" {
		return fmt.Errorf("invalid payment redirect URL scheme")
	}
	base := parsedURL.Scheme + "://" + parsedURL.Host + strings.TrimRight(parsedURL.Path, "/")
	for _, trustedURI := range constant.TrustedPaymentRedirectURIs {
		if strings.TrimRight(trustedURI, "/") == base {
			return nil
		}
	}
	return fmt.Errorf("payment redirect URI is not registered")
}
