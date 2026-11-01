# advernue-go

Go client for [Advernue](https://advernue.com): one REST API, one request shape, for every ad platform.
Standard library only, Go 1.22+. Services and types are generated from [`spec/openapi.json`](spec/openapi.json).

```sh
go get github.com/advernue/advernue-go
```

## Quickstart

```go
client, err := advernue.NewClient() // reads ADVERNUE_API_KEY (and optional ADVERNUE_BASE_URL)
if err != nil {
	log.Fatal(err)
}
campaigns, err := client.Campaigns.List(ctx, &advernue.CampaignsListParams{})
```

Options: `advernue.WithAPIKey`, `WithBaseURL` (default `https://api.advernue.com`), `WithHTTPClient`, `WithMaxRetries`, `WithRetryBackoff`.

Every tag in the API is a service on the client, and `operationId = <tag>.<operation>` maps to `client.<Tag>.<Operation>`:
`campaigns.list` is `client.Campaigns.List(ctx, params)`, `analytics.compare` is `client.Analytics.Compare(ctx, body)`.
Path parameters are positional (`client.Campaigns.Get(ctx, id)`), query parameters go in a `*Params` struct, and the JSON body is a typed struct.
Free-form fields (such as `platformNative`) are `json.RawMessage`.

## Examples

### Compare platforms

```go
res, err := client.Analytics.Compare(ctx, &advernue.AnalyticsCompareRequest{
	GroupBy: "platform", From: "2026-10-01", To: "2026-10-31",
})
for _, g := range res.Groups {
	fmt.Println(g.Key)
}
```

### Create a cross-platform campaign (children paused until all succeed)

```go
brief := &advernue.CampaignBrief{
	Name:           "Autumn sale",
	Objective:      "traffic",
	DestinationURL: "https://example.com/sale",
	Targets:        []string{"acc_google", "acc_meta"},
	TotalBudget:    advernue.Budget{Type: "daily", Amount: advernue.MustMoney("100", "USD")},
	Split:          []byte(`{"type":"equal"}`),
	Targeting:      advernue.CampaignBriefTargeting{Countries: []string{"US"}},
	Creatives:      []advernue.CampaignBriefCreativesItem{ /* ... */ },
}
// Preview with advernue.WithDryRun(), then launch with a stable idempotency key.
res, err := client.Crosspost.Launch(ctx, brief, advernue.WithIdempotencyKey("autumn-sale-v1"))
```

### Pacing

```go
cl := "cl_123"
pacing, err := client.Analytics.Pacing(ctx, &advernue.AnalyticsPacingRequest{ClientID: &cl})
for _, c := range pacing.Campaigns {
	fmt.Println(c.CampaignID)
}
```

## Behaviour

- **Writes**: every write sends an `Idempotency-Key`; if you do not pass `advernue.WithIdempotencyKey(k)`, a random one is generated (crypto/rand). `advernue.WithDryRun()` validates and previews without touching the platform. Writes return the `Action`; read its `Status` (`succeeded`, `pending_approval`, `dry_run`, `denied`, `failed`).
- **Errors**: non-2xx responses become `*advernue.Error` (`Status`, `Code`, `Message`, `Details`):

  ```go
  var apiErr *advernue.Error
  if errors.As(err, &apiErr) && apiErr.Code == "fx_rate_missing" { /* ... */ }
  ```

- **Retries**: 429 and 5xx are retried with exponential backoff (honouring `Retry-After`). GET always; writes only when they carry an idempotency key. The context cancels waits and requests.
- **Money**: `advernue.Money{Micros, Currency}` is integer micros. `ParseMoney("12.5", "USD")` and `m.Decimal()` convert to and from decimal strings without floats.

## Development

```sh
go generate ./...   # regenerate types_gen.go, services_gen.go and CONTRIBUTING.md from spec/openapi.json
go vet ./... && go test -race ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md). MIT licensed.
