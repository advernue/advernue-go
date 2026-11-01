package advernue_test

import (
	"context"
	"fmt"
	"log"

	"github.com/advernue/advernue-go"
)

// Compile-checked versions of the README examples.

func Example_comparePlatforms() {
	client, err := advernue.NewClient() // ADVERNUE_API_KEY from the environment
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.Analytics.Compare(context.Background(), &advernue.AnalyticsCompareRequest{
		GroupBy: "platform", From: "2026-10-01", To: "2026-10-31",
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, g := range res.Groups {
		fmt.Println(g.Key)
	}
}

func Example_crossPlatformLaunch() {
	client, _ := advernue.NewClient()
	brief := &advernue.CampaignBrief{
		Name:           "Autumn sale",
		Objective:      "traffic",
		DestinationURL: "https://example.com/sale",
		Targets:        []string{"acc_google", "acc_meta"},
		TotalBudget:    advernue.Budget{Type: "daily", Amount: advernue.MustMoney("100", "USD")},
		Split:          []byte(`{"type":"equal"}`),
		Targeting:      advernue.CampaignBriefTargeting{Countries: []string{"US"}},
		Creatives:      []advernue.CampaignBriefCreativesItem{},
	}
	// Children are created paused and activated only if every platform succeeds.
	// Preview first with WithDryRun, then launch with a stable idempotency key.
	res, err := client.Crosspost.Launch(context.Background(), brief, advernue.WithIdempotencyKey("autumn-sale-v1"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", res)
}

func Example_pacing() {
	client, _ := advernue.NewClient()
	client_ := "cl_123"
	res, err := client.Analytics.Pacing(context.Background(), &advernue.AnalyticsPacingRequest{ClientID: &client_})
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range res.Campaigns {
		fmt.Println(c.CampaignID)
	}
}
