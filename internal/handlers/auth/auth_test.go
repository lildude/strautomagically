package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/jarcoal/httpmock"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Athlete{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

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

	db := setupTestDB(t)
	database.SetTestDB(db)
	t.Cleanup(func() { database.SetTestDB(nil) })

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

func TestAuthHandlerStoresTokens(t *testing.T) {
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

	db := setupTestDB(t)
	database.SetTestDB(db)
	t.Cleanup(func() { database.SetTestDB(nil) })

	const testValidState = "abc123def456ghi7"

	req, err := http.NewRequest(
		http.MethodPost,
		"/auth?state="+testValidState+"&code=test-code",
		strings.NewReader(""),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: testValidState})
	rr := httptest.NewRecorder()
	http.HandlerFunc(AuthHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("handler returned wrong status code: got %d want %d", rr.Code, http.StatusFound)
	}

	var athlete model.Athlete
	if err := db.First(&athlete).Error; err != nil {
		t.Fatalf("failed to load athlete: %v", err)
	}

	if athlete.StravaAccessToken != "123456789" {
		t.Errorf("unexpected access token: got %q want %q", athlete.StravaAccessToken, "123456789")
	}
	if athlete.StravaRefreshToken != "987654321" {
		t.Errorf("unexpected refresh token: got %q want %q", athlete.StravaRefreshToken, "987654321")
	}

	var storedToken map[string]any
	if err := json.Unmarshal([]byte(athlete.StravaAuthToken), &storedToken); err != nil {
		t.Fatalf("expected valid token JSON to be stored: %v", err)
	}
	if storedToken["access_token"] != "123456789" {
		t.Errorf("unexpected auth token payload access token: got %#v want %q", storedToken["access_token"], "123456789")
	}
	if storedToken["refresh_token"] != "987654321" {
		t.Errorf("unexpected auth token payload refresh token: got %#v want %q", storedToken["refresh_token"], "987654321")
	}
}
