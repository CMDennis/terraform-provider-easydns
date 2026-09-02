package provider

import (
	"fmt"
	"net/url"
	"strings"
)

const sandboxHostname = "sandbox.rest.easydns.net"

func validateAcceptanceBaseURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("parse acceptance-test API URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("acceptance tests require an HTTPS EasyDNS sandbox URL")
	}
	if !strings.EqualFold(parsed.Hostname(), sandboxHostname) {
		return "", fmt.Errorf("acceptance tests may only use %s", sandboxHostname)
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return "", fmt.Errorf("acceptance tests may only use the default HTTPS port")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("acceptance-test API URL must not include credentials, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("acceptance-test API URL must not include a path")
	}
	parsed.Path = ""
	return parsed.String(), nil
}
