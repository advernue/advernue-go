package advernue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTest(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	opts = append([]Option{WithAPIKey("k_test"), WithBaseURL(srv.URL), WithRetryBackoff(time.Millisecond)}, opts...)
	c, err := NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEnvConfig(t *testing.T) {
	t.Setenv("ADVERNUE_API_KEY", "envkey")
	t.Setenv("ADVERNUE_BASE_URL", "http://example.test/")
	c, err := NewClient()
	if err != nil || c.apiKey != "envkey" || c.baseURL != "http://example.test" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("ADVERNUE_API_KEY", "")
	t.Setenv("ADVERNUE_BASE_URL", "")
	if _, err := NewClient(); err == nil {
		t.Fatal("expected missing key error")
	}
	c, _ = NewClient(WithAPIKey("x"))
	if c.baseURL != "https://api.advernue.com" {
		t.Fatal(c.baseURL)
	}
}

func TestListQueryAndAuth(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k_test" || r.URL.Path != "/v1/campaigns" ||
			r.URL.Query().Get("clientId") != "cl_1" || r.URL.Query().Get("status") != "paused" {
			t.Errorf("bad request: %s %v", r.URL, r.Header)
		}
		io.WriteString(w, `[{"id":"c1","name":"n","adAccountId":"a","externalId":"e","objective":"traffic","platform":"meta"}]`)
	})
	cl, st := "cl_1", EntityStatus("paused")
	got, err := c.Campaigns.List(context.Background(), &CampaignsListParams{ClientID: &cl, Status: &st})
	if err != nil || len(got) != 1 || got[0].ID != "c1" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestWriteIdempotencyDryRunAndBody(t *testing.T) {
	var keys []string
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		if r.Method != "POST" || r.URL.Path != "/v1/campaigns" || b["name"] != "Spring" || r.URL.Query().Get("dry_run") != "true" {
			t.Errorf("bad request %s %s %v", r.Method, r.URL, b)
		}
		w.WriteHeader(200)
		io.WriteString(w, `{"id":"act_1","status":"dry_run"}`)
	})
	body := &CampaignsCreateRequest{AdAccountID: "a", Name: "Spring", Objective: "traffic"}
	if _, err := c.Campaigns.Create(context.Background(), body, WithDryRun()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Campaigns.Create(context.Background(), body, WithDryRun(), WithIdempotencyKey("mine")); err != nil {
		t.Fatal(err)
	}
	if len(keys[0]) != 32 || keys[1] != "mine" {
		t.Fatalf("keys %v", keys)
	}
}

func TestErrorEnvelope(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		io.WriteString(w, `{"error":{"code":"fx_rate_missing","message":"no rate","details":{"pair":"EURUSD"}}}`)
	})
	_, err := c.Campaigns.Get(context.Background(), "x")
	var ae *Error
	if !errors.As(err, &ae) || ae.Status != 422 || ae.Code != "fx_rate_missing" || ae.Message != "no rate" || string(ae.Details) != `{"pair":"EURUSD"}` {
		t.Fatalf("%#v", err)
	}
}

func TestNonEnvelopeError(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404); io.WriteString(w, "nope") })
	_, err := c.Campaigns.Get(context.Background(), "x")
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != "http_404" {
		t.Fatalf("%v", err)
	}
}

func TestRetryGetThen200(t *testing.T) {
	var n int32
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"id":"c1"}`)
	})
	if _, err := c.Campaigns.Get(context.Background(), "c1"); err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestRetryWriteKeepsKeyAndNoRetryOn4xx(t *testing.T) {
	var keys []string
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) == 1 {
			w.WriteHeader(429)
			w.Header().Set("Retry-After", "0")
			return
		}
		w.WriteHeader(201)
		io.WriteString(w, `{"id":"act","status":"succeeded"}`)
	})
	a, err := c.Campaigns.Create(context.Background(), &CampaignsCreateRequest{Name: "n"})
	if err != nil || a.ID != "act" || len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("%v %v %v", a, err, keys)
	}
	var n int32
	c2 := newTest(t, func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&n, 1); w.WriteHeader(400) })
	if _, err := c2.Campaigns.Get(context.Background(), "x"); err == nil || n != 1 {
		t.Fatalf("n=%d", n)
	}
}

func TestWriteWithoutKeyNotRetried(t *testing.T) {
	var n int32
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&n, 1); w.WriteHeader(500) })
	// Fabricate an op that does not accept Idempotency-Key.
	err := c.do(context.Background(), op{method: "POST", path: "/x"}, nil, nil, nil)
	if err == nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestContextCancelStopsRetry(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, WithRetryBackoff(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Campaigns.Get(ctx, "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
}

func TestPathEscapeAndAnalyticsCompare(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/analytics/compare" {
			io.WriteString(w, `{"groups":[{"key":"meta"}]}`)
			return
		}
		if r.URL.EscapedPath() != "/v1/campaigns/a%2Fb" {
			t.Errorf("path %s", r.URL.EscapedPath())
		}
		io.WriteString(w, `{}`)
	})
	c.Campaigns.Get(context.Background(), "a/b")
	res, err := c.Analytics.Compare(context.Background(), &AnalyticsCompareRequest{GroupBy: "platform", From: "2026-01-01", To: "2026-01-31"})
	if err != nil || len(res.Groups) != 1 || res.Groups[0].Key != "meta" {
		t.Fatalf("%v %v", res, err)
	}
}
