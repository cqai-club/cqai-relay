package controller

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func paymentReturnPath(suffix string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return base + suffix
}

func resolvePaymentReturnURL(c *gin.Context, rawURL string, fallback string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return fallback, nil
	}
	value := strings.TrimSpace(rawURL)
	if c != nil && c.GetBool(internalPaymentRequestContextKey) {
		if err := validatePaymentRedirectURLSyntax(value); err != nil {
			return "", fmt.Errorf("invalid payment return URL: %w", err)
		}
		return value, nil
	}
	if err := common.ValidatePaymentRedirectURL(value); err != nil {
		return "", fmt.Errorf("invalid payment return URL: %w", err)
	}
	return value, nil
}

func validatePaymentRedirectURLSyntax(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("payment redirect URL is malformed")
	}
	if parsedURL.User != nil {
		return fmt.Errorf("payment redirect URL must not contain credentials")
	}
	switch strings.ToLower(parsedURL.Scheme) {
	case "blob", "data", "file", "ftp", "javascript", "mailto":
		return fmt.Errorf("payment redirect URL uses a forbidden scheme")
	}
	return nil
}

func addPaymentReturnParam(rawURL string, key string, value string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return rawURL, nil
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set(key, value)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}
