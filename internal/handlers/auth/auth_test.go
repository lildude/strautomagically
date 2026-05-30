package auth

import (
	"io"
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

	if err := db.AutoMigrate(&model.Athlete{}, &model.Summit{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestAuthHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

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

	t.Setenv("STATE_TOKEN", "test-state-token")

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
			req, err := http.NewRequest(http.MethodPost, "/auth"+tc.query, strings.NewReader(tc.body))
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
