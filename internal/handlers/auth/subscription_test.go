package auth

import (
	"io"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/sirupsen/logrus"
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
			name: "Successfully subscribed",
			mock: "no_subscriptions.json",
			want: true,
		},
		{
			name: "Failed to subscribe",
			mock: "subscriptions.json",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := os.ReadFile("testdata/" + tc.mock)
			httpmock.RegisterResponder("GET", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(200, string(resp)))

			httpmock.RegisterResponder("POST", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(204, ``))

			got, _ := Subscribe()
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
			name: "Subscription exists",
			mock: "subscriptions.json",
			want: true,
		},
		{
			name: "Subscription does not exist",
			mock: "no_subscriptions.json",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := os.ReadFile("testdata/" + tc.mock)
			httpmock.RegisterResponder("GET", "https://www.strava.com/api/v3/push_subscriptions",
				httpmock.NewStringResponder(200, string(resp)))

			got := existingSubscription()
			if tc.want != got {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestUnsubscribe(t *testing.T) {
}

func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard)
	os.Exit(m.Run())
}
