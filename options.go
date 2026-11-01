package advernue

import (
	"net/http"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the bearer key (default: ADVERNUE_API_KEY).
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithBaseURL overrides the API origin (default: ADVERNUE_BASE_URL or https://api.advernue.com).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient sets the underlying *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithMaxRetries sets how many times a retryable request is retried (default 2).
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// WithRetryBackoff sets the base delay for exponential backoff (default 500ms).
func WithRetryBackoff(d time.Duration) Option { return func(c *Client) { c.backoff = d } }

// CallOption tweaks a single call.
type CallOption func(*callConfig)

type callConfig struct {
	idemKey string
	dryRun  bool
}

// WithIdempotencyKey sets the Idempotency-Key header. Without it, write calls
// get a random key so retries are safe.
func WithIdempotencyKey(key string) CallOption { return func(c *callConfig) { c.idemKey = key } }

// WithDryRun validates and previews a write without sending it to the platform.
func WithDryRun() CallOption { return func(c *callConfig) { c.dryRun = true } }
