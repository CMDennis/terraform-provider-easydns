package provider

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type domainNameValidator struct{}

func (domainNameValidator) Description(context.Context) string {
	return "must be a DNS domain name such as example.com"
}

func (domainNameValidator) MarkdownDescription(ctx context.Context) string {
	return domainNameValidator{}.Description(ctx)
}

func (domainNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := NormalizeDomain(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Domain Name",
			fmt.Sprintf("%q is not a usable domain name: %s", req.ConfigValue.ValueString(), err))
	}
}

// DomainNameValidator rejects a value the client could not turn into an IDNA
// ASCII domain, so the failure surfaces during planning.
func DomainNameValidator() validator.String {
	return domainNameValidator{}
}

type oneOfValidator struct {
	name    string
	allowed []string
}

func (v oneOfValidator) Description(context.Context) string {
	return fmt.Sprintf("must be one of %s", strings.Join(v.allowed, ", "))
}

func (v oneOfValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v oneOfValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	for _, allowed := range v.allowed {
		if value == allowed {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, fmt.Sprintf("Invalid %s", v.name),
		fmt.Sprintf("%s must be one of %s, got %q.", v.name, strings.Join(v.allowed, ", "), value))
}

func DomainServiceValidator() validator.String {
	return oneOfValidator{name: "service", allowed: []string{"lite", "dns", "pro", "enterprise"}}
}

func DomainCurrencyValidator() validator.String {
	return oneOfValidator{name: "currency", allowed: []string{"CAD", "USD"}}
}

func RenewalActionValidator() validator.String {
	return oneOfValidator{name: "renewal", allowed: []string{"remind", "renew", "expire"}}
}

type domainTermValidator struct{}

func (domainTermValidator) Description(context.Context) string {
	return "must be between 1 and 10 years"
}

func (domainTermValidator) MarkdownDescription(ctx context.Context) string {
	return domainTermValidator{}.Description(ctx)
}

func (domainTermValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if value := req.ConfigValue.ValueInt64(); value < 1 || value > 10 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Term",
			fmt.Sprintf("term must be between 1 and 10 years, got %d.", value))
	}
}

func DomainTermValidator() validator.Int64 {
	return domainTermValidator{}
}

// assertPremiumPriceWithinCeiling compares two decimal money strings exactly.
// Float arithmetic is avoided so a price is never accepted through rounding.
func assertPremiumPriceWithinCeiling(price, maximum string) error {
	parsedPrice, err := parseMoney(price)
	if err != nil {
		return fmt.Errorf("premium_price %q is not a decimal amount", price)
	}
	parsedMaximum, err := parseMoney(maximum)
	if err != nil {
		return fmt.Errorf("max_premium_price %q is not a decimal amount", maximum)
	}
	if parsedPrice.Cmp(parsedMaximum) > 0 {
		return fmt.Errorf("premium_price %s is above max_premium_price %s; confirm the registry price and raise the maximum deliberately", price, maximum)
	}
	return nil
}

func parseMoney(value string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("empty amount")
	}
	trimmed = strings.TrimPrefix(trimmed, "$")
	amount, ok := new(big.Rat).SetString(trimmed)
	if !ok || amount.Sign() < 0 {
		return nil, fmt.Errorf("invalid amount %q", value)
	}
	return amount, nil
}

type durationValidator struct {
	name string
}

func (v durationValidator) Description(context.Context) string {
	return "must be a positive Go duration such as 5s or 2m"
}

func (v durationValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v durationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := parsePositiveDuration(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, fmt.Sprintf("Invalid %s", v.name), err.Error())
	}
}

// DurationValidator rejects a value that is not a positive Go duration, so the
// failure surfaces during planning rather than at client construction.
func DurationValidator(name string) validator.String {
	return durationValidator{name: name}
}

func parsePositiveDuration(value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%q is not a Go duration; use a form such as 5s, 500ms, or 2m", value)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%q must be greater than zero", value)
	}
	return parsed, nil
}
