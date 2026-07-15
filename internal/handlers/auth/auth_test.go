package auth

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jarcoal/httpmock"
)

func TestAuthHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.DiscardHandler))

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
	t.Setenv("REDIS_URL", "redis://"+r.Addr())

	const testValidState = "abc123def456ghi7"

	tests := []struct {
		name        string
		query       string
		body        string
		stateCookie string
		want        int
	}{
		{
			name:  "no state redirects to strava",
			query: "",
			body:  "",
			want:  http.StatusFound,
		},
		{
			name:        "invalid state",
			query:       "?state=invalid-state",
			stateCookie: testValidState,
			want:        http.StatusBadRequest,
		},
		{
			name:        "valid state but no code",
			query:       "?state=" + testValidState,
			stateCookie: testValidState,
			want:        http.StatusBadRequest,
		},
		{
			name:        "valid state and code",
			query:       "?state=" + testValidState + "&code=test-code",
			stateCookie: testValidState,
			want:        http.StatusFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/auth"+tc.query, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if tc.stateCookie != "" {
				req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: tc.stateCookie})
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
