package duva

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "https://api.duva.ca"
	defaultTimeout      = 10 * time.Second
	defaultMaxRetries   = 2
	defaultMaxRetryWait = 30 * time.Second
	packageVersion      = "0.1.0"
)

// Doer is the minimal HTTP client interface this package needs: *http.Client satisfies it, and
// so does any test double or wrapper (proxying, logging, mTLS). See TestTransport for testing
// your own application without a real network call.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type config struct {
	apiKey       string
	domain       string
	baseURL      string
	timeout      time.Duration
	maxRetries   int
	maxRetryWait time.Duration
	language     string
	userAgent    string
	httpClient   Doer
}

// Option configures a Client created by New.
type Option func(*config)

// WithBaseURL overrides the default https://api.duva.ca (http://localhost is allowed, for tests).
func WithBaseURL(baseURL string) Option { return func(c *config) { c.baseURL = baseURL } }

// WithTimeout sets the per-attempt request timeout (10s by default). Only takes effect when no
// WithHTTPClient is given: your own *http.Client keeps its own timeout.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// WithMaxRetries sets how many times a safe-to-retry call is retried after a transient failure
// (2 by default; 0 disables retries).
func WithMaxRetries(n int) Option { return func(c *config) { c.maxRetries = n } }

// WithMaxRetryWait sets the longest Retry-After a 429 rate_limited is retried for (30s by
// default); a longer wait raises *RateLimitError instead.
func WithMaxRetryWait(d time.Duration) Option { return func(c *config) { c.maxRetryWait = d } }

// WithLanguage sets Accept-Language ("en" or "fr"): the language of error.message.
func WithLanguage(lang string) Option { return func(c *config) { c.language = lang } }

// WithUserAgent appends text to the User-Agent header (never replaces it), for diagnostics.
func WithUserAgent(ua string) Option { return func(c *config) { c.userAgent = ua } }

// WithHTTPClient injects your own Doer (proxy, mTLS, logging, or TestTransport for tests)
// instead of the default *http.Client.
func WithHTTPClient(client Doer) Option { return func(c *config) { c.httpClient = client } }

func resolveConfig(apiKey, domain string, opts []Option) (*config, error) {
	cfg := &config{
		baseURL:      defaultBaseURL,
		timeout:      defaultTimeout,
		maxRetries:   defaultMaxRetries,
		maxRetryWait: defaultMaxRetryWait,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if apiKey == "" {
		apiKey = os.Getenv("DUVA_API_KEY")
	}
	if domain == "" {
		domain = os.Getenv("DUVA_DOMAIN")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("duva: an API key is required (New's apiKey argument or DUVA_API_KEY)")
	}
	if domain == "" {
		return nil, fmt.Errorf("duva: a domain is required (New's domain argument or DUVA_DOMAIN)")
	}
	cfg.apiKey = apiKey
	cfg.domain = domain
	cfg.baseURL = strings.TrimRight(cfg.baseURL, "/")
	if !strings.HasPrefix(cfg.baseURL, "https://") && !strings.Contains(cfg.baseURL, "localhost") {
		return nil, fmt.Errorf("duva: base URL must be https:// (http://localhost is allowed for tests)")
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	}
	return cfg, nil
}

func (c *config) domainPath() string {
	return "/v1/" + pathEscape(c.domain)
}

func (c *config) userAgentHeader() string {
	if c.userAgent != "" {
		return fmt.Sprintf("duva-go/%s %s", packageVersion, c.userAgent)
	}
	return fmt.Sprintf("duva-go/%s", packageVersion)
}
