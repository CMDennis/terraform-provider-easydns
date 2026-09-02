package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Known DNS record types supported by EasyDNS
var knownRecordTypes = map[string]bool{
	"A":         true,
	"AAAA":      true,
	"AFSDB":     true,
	"ANAME":     true,
	"CAA":       true,
	"CERT":      true,
	"CNAME":     true,
	"DYN":       true,
	"MX":        true,
	"NAPTR":     true,
	"NS":        true,
	"PTR":       true,
	"SECONDARY": true,
	"SOA":       true,
	"SPF":       true,
	"SRV":       true,
	"SSHFP":     true,
	"STEALTH":   true,
	"TLSA":      true,
	"TXT":       true,
	"URL":       true,
	"URLHTTPS":  true,
}

// Record types that use priority
var priorityRecordTypes = map[string]bool{
	"MX":  true,
	"SRV": true,
}

// hostnameRegex validates DNS hostname characters
// Allows: letters, digits, hyphens, underscores, dots, and @ for root
// Also allows underscore-prefixed labels for DMARC (_dmarc), DKIM (x._domainkey), etc.
var hostnameRegex = regexp.MustCompile(`^(@|\*|(\*\.)?[a-zA-Z0-9_]([a-zA-Z0-9\-_]*)?(\.[a-zA-Z0-9_]([a-zA-Z0-9\-_]*)?)*)$`)

// ============================================================================
// Record Type Validator (Warning only)
// ============================================================================

type recordTypeValidator struct{}

func (v recordTypeValidator) Description(ctx context.Context) string {
	return "validates DNS record type"
}

func (v recordTypeValidator) MarkdownDescription(ctx context.Context) string {
	return "validates DNS record type is a known EasyDNS type"
}

func (v recordTypeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := strings.ToUpper(req.ConfigValue.ValueString())
	if !knownRecordTypes[value] {
		resp.Diagnostics.AddWarning(
			"Unknown Record Type",
			fmt.Sprintf("Record type '%s' is not a known EasyDNS type. The API may reject this request. Known types: A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, PTR, SPF, etc.", req.ConfigValue.ValueString()),
		)
	}
}

func RecordTypeValidator() validator.String {
	return recordTypeValidator{}
}

// ============================================================================
// Hostname Validator
// ============================================================================

type hostnameValidator struct{}

func (v hostnameValidator) Description(ctx context.Context) string {
	return "validates DNS hostname format"
}

func (v hostnameValidator) MarkdownDescription(ctx context.Context) string {
	return "validates that the hostname contains only valid DNS characters"
}

func (v hostnameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !hostnameRegex.MatchString(value) {
		resp.Diagnostics.AddError(
			"Invalid Hostname",
			fmt.Sprintf("Hostname '%s' contains invalid characters. Use only letters, digits, hyphens, underscores, and dots. Use '@' for the root domain or '*' for wildcards.", value),
		)
	}
}

func HostnameValidator() validator.String {
	return hostnameValidator{}
}

// ============================================================================
// IPv4 Validator (for A records)
// ============================================================================

type ipv4Validator struct{}

func (v ipv4Validator) Description(ctx context.Context) string {
	return "validates IPv4 address format"
}

func (v ipv4Validator) MarkdownDescription(ctx context.Context) string {
	return "validates that the value is a valid IPv4 address"
}

func (v ipv4Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		resp.Diagnostics.AddError(
			"Invalid IPv4 Address",
			fmt.Sprintf("'%s' is not a valid IPv4 address for an A record.", value),
		)
	}
}

func IPv4Validator() validator.String {
	return ipv4Validator{}
}

// ============================================================================
// IPv6 Validator (for AAAA records)
// ============================================================================

type ipv6Validator struct{}

func (v ipv6Validator) Description(ctx context.Context) string {
	return "validates IPv6 address format"
}

func (v ipv6Validator) MarkdownDescription(ctx context.Context) string {
	return "validates that the value is a valid IPv6 address"
}

func (v ipv6Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() != nil { // To4() returns non-nil for IPv4, nil for IPv6
		resp.Diagnostics.AddError(
			"Invalid IPv6 Address",
			fmt.Sprintf("'%s' is not a valid IPv6 address for an AAAA record.", value),
		)
	}
}

func IPv6Validator() validator.String {
	return ipv6Validator{}
}

// ============================================================================
// Priority Validator (for MX/SRV records)
// ============================================================================

type priorityValidator struct{}

func (v priorityValidator) Description(ctx context.Context) string {
	return "validates priority is between 0 and 100"
}

func (v priorityValidator) MarkdownDescription(ctx context.Context) string {
	return "validates that priority is within the valid range (0-100)"
}

func (v priorityValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueInt64()
	if value < 0 || value > 100 {
		resp.Diagnostics.AddError(
			"Invalid Priority",
			fmt.Sprintf("Priority must be between 0 and 100, got: %d", value),
		)
	}
}

func PriorityValidator() validator.Int64 {
	return priorityValidator{}
}
