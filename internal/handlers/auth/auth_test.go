package auth

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jarcoal/httpmock"
)

func TestAuthHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	oat := `{
		"access_token":"123456789",
		"token_type":"Bearer",
		"refresh_token":"987654321",
		"expiry":"2022-07-12T18:30:36.917400827Z",
		"athlete":{
			"id":1,
			"username":"test"
			}
		}`

	httpmock.RegisterResponder("POST", "https://www.strava.com/oauth/token",
		httpmock.NewStringResponder(200, oat))

	httpmock.RegisterResponder("GET", "https://www.strava.com/api/v3/push_subscriptions",
		httpmock.NewStringResponder(200, `[{}]`))

	httpmock.RegisterResponder("POST", "https://www.strava.com/api/v3/push_subscriptions",
		httpmock.NewStringResponder(200, `{"id":1}`))

	r := miniredis.RunT(t)
	defer r.Close()
	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
	os.Setenv("STATE_TOKEN", "test-state-token")

	tests := []struct {
		name  string
		query string
		body  string
		want  int
	}{
		{
			"no state redirects to strava",
			"",
			"",
			http.StatusFound,
		},
		{
			"invalid state",
			"?state=invalid-state",
			"",
			http.StatusBadRequest,
		},
		{
			"valid state but no code",
			"?state=test-state-token",
			"",
			http.StatusBadRequest,
		},
		{
			"valid state and code",
			"?state=test-state-token&code=test-code",
			"",
			http.StatusFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", fmt.Sprintf("/auth%s", tc.query), strings.NewReader(tc.body)) //nolint:noctx
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(AuthHandler)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.want {
				t.Errorf("%s: handler returned wrong status code: got %d want %d", tc.name, status, tc.want)
			}
		})
	}
}
