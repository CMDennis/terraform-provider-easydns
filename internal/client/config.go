package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultHTTPTimeout            = 30 * time.Second
	DefaultRequestInterval        = time.Second
	DefaultMaxResponseBytes       = int64(8 << 20)
	DefaultRecordPollInterval     = 2 * time.Second
	DefaultRecordReconcileTimeout = 2 * time.Minute
)

type RecordWriteMode string

const (
	RecordWriteModeSynchronous  RecordWriteMode = "synchronous"
	RecordWriteModeAsynchronous RecordWriteMode = "asynchronous"
)

type Clock interface {
	Now() time.Time
}

type Waiter interface {
	Wait(context.Context, time.Duration) error
}

type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Jitter       func(time.Duration) time.Duration
}

type Config struct {
	BaseURL         string
	Token           string
	Key             string
	RecordWriteMode RecordWriteMode
	// EnableDomainRegistration is the provider-level opt-in from ADR-0003. It
	// is enforced here as well as in the Terraform layer so a registration can
	// never be billed through a client that was not explicitly allowed to.
	EnableDomainRegistration bool
	HTTPClient               *http.Client
	HTTPTimeout              time.Duration
	RequestInterval          time.Duration
	DisableRateLimiting      bool
	MaxResponseBodyBytes     int64
	RecordPollInterval       time.Duration
	RecordReconcileTimeout   time.Duration
	RetryPolicy              RetryPolicy
	Clock                    Clock
	Waiter                   Waiter
	UserAgent                string
}

type Client struct {
	baseURL                  *url.URL
	token                    string
	key                      string
	httpClient               *http.Client
	recordWriteMode          RecordWriteMode
	enableDomainRegistration bool
	requestInterval          time.Duration
	maxResponseBodyBytes     int64
	recordPollInterval       time.Duration
	recordReconcileTimeout   time.Duration
	retryPolicy              RetryPolicy
	clock                    Clock
	waiter                   Waiter
	userAgent                string

	rateMu      sync.Mutex
	nextRequest time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type realWaiter struct{}

func (realWaiter) Wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func New(config Config) (*Client, error) {
	baseURL, err := parseBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}

	mode := config.RecordWriteMode
	if mode == "" {
		mode = RecordWriteModeSynchronous
	}
	if mode != RecordWriteModeSynchronous && mode != RecordWriteModeAsynchronous {
		return nil, fmt.Errorf("invalid record write mode %q", mode)
	}

	httpTimeout := config.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = DefaultHTTPTimeout
	}
	if httpTimeout < 0 {
		return nil, fmt.Errorf("HTTP timeout cannot be negative")
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	} else {
		clone := *httpClient
		httpClient = &clone
	}
	if httpClient.Timeout == 0 {
		httpClient.Timeout = httpTimeout
	}

	requestInterval := config.RequestInterval
	if requestInterval == 0 {
		requestInterval = DefaultRequestInterval
	}
	if config.DisableRateLimiting {
		requestInterval = 0
	}
	if requestInterval < 0 {
		return nil, fmt.Errorf("request interval cannot be negative")
	}

	maxResponseBodyBytes := config.MaxResponseBodyBytes
	if maxResponseBodyBytes == 0 {
		maxResponseBodyBytes = DefaultMaxResponseBytes
	}
	if maxResponseBodyBytes < 1 {
		return nil, fmt.Errorf("maximum response body size must be positive")
	}

	recordPollInterval := config.RecordPollInterval
	if recordPollInterval == 0 {
		recordPollInterval = DefaultRecordPollInterval
	}
	if recordPollInterval < 0 {
		return nil, fmt.Errorf("record poll interval cannot be negative")
	}
	recordReconcileTimeout := config.RecordReconcileTimeout
	if recordReconcileTimeout == 0 {
		recordReconcileTimeout = DefaultRecordReconcileTimeout
	}
	if recordReconcileTimeout < 0 {
		return nil, fmt.Errorf("record reconciliation timeout cannot be negative")
	}

	retryPolicy := config.RetryPolicy
	if retryPolicy.MaxAttempts == 0 {
		retryPolicy.MaxAttempts = 4
	}
	if retryPolicy.InitialDelay == 0 {
		retryPolicy.InitialDelay = time.Second
	}
	if retryPolicy.MaxDelay == 0 {
		retryPolicy.MaxDelay = 8 * time.Second
	}
	if retryPolicy.MaxAttempts < 1 {
		return nil, fmt.Errorf("retry maximum attempts must be at least one")
	}
	if retryPolicy.InitialDelay < 0 || retryPolicy.MaxDelay < 0 {
		return nil, fmt.Errorf("retry delays cannot be negative")
	}
	if retryPolicy.MaxDelay < retryPolicy.InitialDelay {
		return nil, fmt.Errorf("retry maximum delay cannot be less than initial delay")
	}
	if retryPolicy.Jitter == nil {
		retryPolicy.Jitter = defaultJitter
	}

	clock := config.Clock
	if clock == nil {
		clock = realClock{}
	}
	waiter := config.Waiter
	if waiter == nil {
		waiter = realWaiter{}
	}

	userAgent := strings.TrimSpace(config.UserAgent)
	if userAgent == "" {
		userAgent = "terraform-provider-easydns"
	}

	return &Client{
		baseURL:                  baseURL,
		token:                    config.Token,
		key:                      config.Key,
		httpClient:               httpClient,
		recordWriteMode:          mode,
		enableDomainRegistration: config.EnableDomainRegistration,
		requestInterval:          requestInterval,
		maxResponseBodyBytes:     maxResponseBodyBytes,
		recordPollInterval:       recordPollInterval,
		recordReconcileTimeout:   recordReconcileTimeout,
		retryPolicy:              retryPolicy,
		clock:                    clock,
		waiter:                   waiter,
		userAgent:                userAgent,
	}, nil
}

func (c *Client) RecordWriteMode() RecordWriteMode {
	return c.recordWriteMode
}

// DomainRegistrationEnabled reports the provider-level registration opt-in.
func (c *Client) DomainRegistrationEnabled() bool {
	return c.enableDomainRegistration
}

func parseBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid EasyDNS API URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("EasyDNS API URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("EasyDNS API URL must include a host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("EasyDNS API URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("EasyDNS API URL must not include a query or fragment")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.EscapedPath(), "/")
	return parsed, nil
}

func (c *Client) endpoint(segments ...string) *url.URL {
	endpoint := *c.baseURL
	decodedPath := strings.TrimRight(endpoint.Path, "/")
	escapedPath := strings.TrimRight(endpoint.EscapedPath(), "/")

	for _, segment := range segments {
		decodedPath += "/" + segment
		escapedPath += "/" + url.PathEscape(segment)
	}

	endpoint.Path = decodedPath
	endpoint.RawPath = escapedPath
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return &endpoint
}

func (c *Client) reserveRequest(ctx context.Context) error {
	if c.requestInterval == 0 {
		return nil
	}

	now := c.clock.Now()
	c.rateMu.Lock()
	scheduled := now
	if c.nextRequest.After(scheduled) {
		scheduled = c.nextRequest
	}
	c.nextRequest = scheduled.Add(c.requestInterval)
	c.rateMu.Unlock()

	return c.waiter.Wait(ctx, scheduled.Sub(now))
}
