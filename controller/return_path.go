package controller

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func paymentReturnPath(suffix string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return base + suffix
}

func resolvePaymentReturnURL(rawURL string, fallback string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return fallback, nil
	}
	value := strings.TrimSpace(rawURL)
	if err := common.ValidatePaymentRedirectURL(value); err != nil {
		return "", fmt.Errorf("invalid payment return URL: %w", err)
	}
	return value, nil
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
