// Package advernue is the Go client for the Advernue unified ad-platform API.
package advernue

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.advernue.com"

// Client talks to the Advernue API. Each tag of the API is a service field.
type Client struct {
	services

	apiKey     string
	baseURL    string
	http       *http.Client
	maxRetries int
	backoff    time.Duration
}

// NewClient builds a Client. The API key comes from WithAPIKey or ADVERNUE_API_KEY.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{maxRetries: 2, backoff: 500 * time.Millisecond}
	for _, o := range opts {
		o(c)
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv("ADVERNUE_API_KEY")
	}
	if c.baseURL == "" {
		c.baseURL = os.Getenv("ADVERNUE_BASE_URL")
	}
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	if c.http == nil {
		c.http = &http.Client{Timeout: 60 * time.Second}
	}
	if c.apiKey == "" {
		return nil, errors.New("advernue: no API key (use WithAPIKey or set ADVERNUE_API_KEY)")
	}
	c.initServices()
	return c, nil
}

type op struct {
	method, path string
	query        url.Values
	idem, dry    bool // operation accepts Idempotency-Key / dry_run
	noAuth       bool
}

// expand fills {name} placeholders in a path template.
func expand(tmpl string, kv ...string) string {
	for i := 0; i+1 < len(kv); i += 2 {
		tmpl = strings.ReplaceAll(tmpl, "{"+kv[i]+"}", url.PathEscape(kv[i+1]))
	}
	return tmpl
}

func (c *Client) do(ctx context.Context, o op, body any, opts []CallOption, out any) error {
	b, err := c.doRaw(ctx, o, body, opts)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("advernue: decode response: %w", err)
	}
	return nil
}

func newKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return hex.EncodeToString(b[:])
}

func (c *Client) doRaw(ctx context.Context, o op, body any, opts []CallOption) ([]byte, error) {
	var cfg callConfig
	for _, f := range opts {
		f(&cfg)
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("advernue: encode body: %w", err)
		}
	}
	q := url.Values{}
	for k, v := range o.query {
		q[k] = v
	}
	if cfg.dryRun && o.dry {
		q.Set("dry_run", "true")
	}
	u := c.baseURL + o.path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	idem := cfg.idemKey
	if idem == "" && o.idem {
		idem = newKey()
	}
	// GET is always safe to retry; writes only when they carry an idempotency key.
	retryable := o.method == http.MethodGet || idem != ""

	for attempt := 0; ; attempt++ {
		var rd io.Reader
		if payload != nil {
			rd = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, o.method, u, rd)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "advernue-go")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if !o.noAuth {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		if idem != "" && o.idem {
			req.Header.Set("Idempotency-Key", idem)
		}
		resp, err := c.http.Do(req)
		var data []byte
		status := 0
		var retryAfter time.Duration
		if err == nil {
			status = resp.StatusCode
			data, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		}
		if err == nil && status < 400 {
			return data, nil
		}
		if err == nil && !(status == 429 || status >= 500) || ctx.Err() != nil {
			if err == nil {
				return nil, parseError(status, data)
			}
			return nil, err
		}
		if !retryable || attempt >= c.maxRetries {
			if err != nil {
				return nil, err
			}
			return nil, parseError(status, data)
		}
		d := retryAfter
		if d == 0 {
			d = c.backoff << attempt
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

func parseRetryAfter(v string) time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}
