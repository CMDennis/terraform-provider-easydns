package client

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

func NormalizeDomain(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}
	if strings.TrimSpace(value) != value {
		return "", fmt.Errorf("domain must not contain surrounding whitespace")
	}
	value = strings.TrimSuffix(value, ".")
	if value == "" || strings.HasSuffix(value, ".") {
		return "", fmt.Errorf("domain may contain at most one trailing dot")
	}
	ascii, err := idna.Lookup.ToASCII(value)
	if err != nil {
		return "", fmt.Errorf("convert domain to IDNA ASCII: %w", err)
	}
	ascii = strings.ToLower(ascii)
	if len(ascii) > 253 {
		return "", fmt.Errorf("domain exceeds 253 characters")
	}
	for _, label := range strings.Split(ascii, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("domain contains an invalid DNS label")
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return "", fmt.Errorf("domain contains an invalid DNS label")
			}
		}
	}
	return ascii, nil
}

func NormalizeHost(value string) string {
	return strings.ToLower(strings.TrimSuffix(value, "."))
}

func NormalizeRecordType(value string) string {
	return strings.ToUpper(value)
}

func NormalizeRecordRdata(recordType, value string) (string, error) {
	switch NormalizeRecordType(recordType) {
	case "A":
		if value == "PARK" {
			return value, nil
		}
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is4() {
			return "", fmt.Errorf("invalid IPv4 address %q", value)
		}
		return address.String(), nil
	case "AAAA":
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is6() || address.Is4In6() {
			return "", fmt.Errorf("invalid IPv6 address %q", value)
		}
		return address.String(), nil
	case "CNAME", "MX", "NS", "PTR", "ANAME", "SECONDARY":
		return normalizeDomainTarget(value), nil
	case "AFSDB", "SRV":
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return value, nil
		}
		fields[len(fields)-1] = normalizeDomainTarget(fields[len(fields)-1])
		return strings.Join(fields, " "), nil
	default:
		return value, nil
	}
}

func normalizeDomainTarget(value string) string {
	if value == "." {
		return value
	}
	return strings.ToLower(strings.TrimSuffix(value, "."))
}

func normalizeCreateRecordRequest(request CreateRecordRequest) (CreateRecordRequest, error) {
	domain, err := NormalizeDomain(request.Domain)
	if err != nil {
		return CreateRecordRequest{}, err
	}
	rdata, err := NormalizeRecordRdata(request.Type, request.Rdata)
	if err != nil {
		return CreateRecordRequest{}, err
	}
	request.Domain = domain
	request.Host = NormalizeHost(request.Host)
	request.Type = NormalizeRecordType(request.Type)
	request.Rdata = rdata
	return request, nil
}

func RecordsEquivalent(record Record, desired CreateRecordRequest) bool {
	normalizedDesired, err := normalizeCreateRecordRequest(desired)
	if err != nil {
		return false
	}
	normalizedDomain, err := NormalizeDomain(record.Domain)
	if err != nil {
		return false
	}
	normalizedRdata, err := NormalizeRecordRdata(record.Type, record.Rdata)
	if err != nil {
		return false
	}
	return normalizedDomain == normalizedDesired.Domain &&
		NormalizeHost(record.Host) == normalizedDesired.Host &&
		NormalizeRecordType(record.Type) == normalizedDesired.Type &&
		normalizedRdata == normalizedDesired.Rdata &&
		record.TTL == normalizedDesired.TTL &&
		record.Prio == normalizedDesired.Prio &&
		record.GeozoneID == normalizedDesired.GeozoneID
}

func lessRecordID(left, right string) bool {
	leftID, leftErr := strconv.ParseInt(left, 10, 64)
	rightID, rightErr := strconv.ParseInt(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		return leftID < rightID
	}
	return left < right
}
