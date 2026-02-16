package auth

import (
	"context"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
)

func TestSubscribe(t *testing.T) {
	t.Setenv("STRAVA_CALLBACK_URI", "https://example.com/webhook")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name string
		mock string
		want bool
	}{
		{
			"successfully subscribed",
			"no_subscriptions.json",
			true,
		},
		{
			"failed to subscribe",
			"subscriptions.json",
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := os.ReadFile("testdata/" + tc.mock)
			httpmock.RegisterResponder("GET", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(200, string(resp)))

			httpmock.RegisterResponder("POST", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(204, ``))

			got, _ := Subscribe(context.Background())
			if tc.want != got {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestExistingSubscription(t *testing.T) {
	t.Setenv("STRAVA_CALLBACK_URI", "https://example.com/webhook")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name string
		mock string
		want bool
	}{
		{
			"subscription exists",
			"subscriptions.json",
			true,
		},
		{
			"subscription does not exist",
			"no_subscriptions.json",
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := os.ReadFile("testdata/" + tc.mock)
			httpmock.RegisterResponder("GET", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(200, string(resp)))

			got := existingSubscription(context.Background())
			if tc.want != got {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestUnsubscribe(t *testing.T) {
}
