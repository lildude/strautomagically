package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/jarcoal/httpmock"
	"github.com/lildude/strautomagically/internal/calendarevent"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Athlete{}, &model.Summit{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestUpdateHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.DiscardHandler))

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	ot, _ := os.ReadFile("testdata/oauth_token.json")
	token := string(ot)
	activity, _ := os.ReadFile("testdata/activity.json")
	weather, _ := os.ReadFile("testdata/weather.json")
	aqi, _ := os.ReadFile("testdata/aqi.json")

	httpmock.RegisterResponder("POST", "https://www.strava.com/oauth/token",
		httpmock.NewStringResponder(200, token))

	httpmock.RegisterResponder("GET", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, string(activity)))

	httpmock.RegisterResponder("PUT", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, string(activity)))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/3.0/onecall/timemachine",
		httpmock.NewStringResponder(200, string(weather)))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/2.5/air_pollution/history",
		httpmock.NewStringResponder(200, string(aqi)))

	t.Setenv("ENV", "test")

	tests := []struct {
		name        string
		webhookBody string
		wantStatus  int
	}{
		{
			"no webhook body",
			``,
			400,
		},
		{
			"invalid JSON in webhook body",
			`{"foo: "bar"}`,
			400,
		},
		{
			"non-create event",
			`{"aspect_type": "update"}`,
			200,
		},
		{
			"create event for unknown athlete",
			`{"owner_id": 999, "aspect_type": "create", "object_id": 789}`,
			500,
		},
		{
			"repeat event",
			`{"owner_id": 1, "aspect_type": "create", "object_id": 123}`,
			200,
		},
		{
			"create event",
			`{"owner_id": 1, "aspect_type": "create", "object_id": 456}`,
			200,
		},
	}

	db := setupTestDB(t)
	database.SetTestDB(db)
	t.Cleanup(func() { database.SetTestDB(nil) })

	tokenJSON := `{"access_token":"123456789"}`
	db.Create(&model.Athlete{
		StravaAthleteID:   1,
		StravaAthleteName: "test",
		StravaAuthToken:   tokenJSON,
		LastActivityID:    123,
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/webhook", strings.NewReader(tc.webhookBody))
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			// Fudging it as webhookHandler handles /webhook but calls updateHandler if it receives a POST request
			handler := http.HandlerFunc(UpdateHandler)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.wantStatus {
				t.Errorf("%s: handler returned wrong status code: got %d want %d", tc.name, status, tc.wantStatus)
			}
		})
	}
}

func TestUpdateHandlerStoresRefreshedTokens(t *testing.T) {
	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.DiscardHandler))

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tokenJSON, _ := os.ReadFile("testdata/oauth_token.json")
	activity, _ := os.ReadFile("testdata/activity.json")
	weather, _ := os.ReadFile("testdata/weather.json")
	aqi, _ := os.ReadFile("testdata/aqi.json")

	httpmock.RegisterResponder("POST", "https://www.strava.com/oauth/token",
		httpmock.NewStringResponder(200, string(tokenJSON)))

	httpmock.RegisterResponder("GET", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, string(activity)))

	httpmock.RegisterResponder("PUT", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, string(activity)))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/3.0/onecall/timemachine",
		httpmock.NewStringResponder(200, string(weather)))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/2.5/air_pollution/history",
		httpmock.NewStringResponder(200, string(aqi)))

	t.Setenv("ENV", "test")

	db := setupTestDB(t)
	database.SetTestDB(db)
	t.Cleanup(func() { database.SetTestDB(nil) })

	const athleteID = int64(42)
	db.Create(&model.Athlete{
		StravaAthleteID: athleteID,
		StravaAuthToken: `{"access_token":"old-access-token","refresh_token":"old-refresh-token","expiry":"2000-01-01T00:00:00Z"}`,
	})

	req, err := http.NewRequest(http.MethodGet, "/webhook", strings.NewReader(`{"owner_id": 42, "aspect_type": "create", "object_id": 456}`))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	http.HandlerFunc(UpdateHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %d want %d", rr.Code, http.StatusOK)
	}

	var athlete model.Athlete
	if err := db.First(&athlete, "strava_athlete_id = ?", athleteID).Error; err != nil {
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

type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestConstructUpdate(t *testing.T) {
	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.DiscardHandler))

	rc, mux, _ := setup()
	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		resp, _ := os.ReadFile("testdata/weather.json")
		fmt.Fprintln(w, string(resp))
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		resp, _ := os.ReadFile("testdata/aqi.json")
		fmt.Fprintln(w, string(resp))
	})

	tests := []struct {
		name    string
		want    *strava.UpdatableActivity
		fixture string
	}{
		{
			"no changes",
			&strava.UpdatableActivity{},
			"no_change.json",
		},
		{
			"unhandled activity type",
			&strava.UpdatableActivity{},
			"handcycle.json",
		},
		{
			"set dog walking title for early morning long walks",
			&strava.UpdatableActivity{
				Name:    "Emptying & Exercising the 🐶",
				Private: false,
				GearID:  "g10043849",
			},
			"walk_early_morning_long.json",
		},
		{
			"set gear and mute afternoon walks",
			&strava.UpdatableActivity{
				HideFromHome: true,
				GearID:       "g10043849",
			},
			"walk_afternoon.json",
		},
		{
			"set gear and mute short early morning walks",
			&strava.UpdatableActivity{
				HideFromHome: true,
				GearID:       "g10043849",
			},
			"walk_early_short.json",
		},
		{
			"set humane burpees title and mute",
			&strava.UpdatableActivity{
				Name:         "Humane Burpees",
				HideFromHome: true,
			},
			"humane_burpees.json",
		},
		{
			"prefix and set title from TrainerRoad calendar for TrainerRoad activities",
			&strava.UpdatableActivity{
				Name:    "TR: Capulin",
				GearID:  "b9880609",
				Trainer: true,
			},
			"trainerroad.json",
		},
		{
			"set gear to bike for non-TrainerRoad ride activities",
			&strava.UpdatableActivity{
				GearID: "b10013574",
			},
			"trainerroad_outside.json",
		},
		{
			"set gear to trainer for Zwift activities",
			&strava.UpdatableActivity{
				GearID:  "b9880609",
				Trainer: true,
			},
			"zwift.json",
		},
		{
			"set gear to bike",
			&strava.UpdatableActivity{
				GearID: "b10013574",
			},
			"ride.json",
		},
		{
			"set rowing title: speed pyramid",
			&strava.UpdatableActivity{
				Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work",
			},
			"row_speed_pyramid.json",
		},
		{
			"set rowing title: speed pyramid - the other one",
			&strava.UpdatableActivity{
				Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work",
			},
			"row_speed_pyramid_2.json",
		},
		{
			"set rowing title: 8x500",
			&strava.UpdatableActivity{
				Name: "8x 500m w/ 3.5' Active RI Row",
			},
			"row_8x500.json",
		},
		{
			"set rowing title: 8x500 - the other one",
			&strava.UpdatableActivity{
				Name: "8x 500m w/ 3.5' Active RI Row",
			},
			"row_8x500_2.json",
		},
		{
			"set rowing title: 5x1500",
			&strava.UpdatableActivity{
				Name: "5x 1500m w/ 5' RI Row",
			},
			"row_5x1500.json",
		},
		{
			"set rowing title: 4x2000",
			&strava.UpdatableActivity{
				Name: "4x 2000m w/5' Active RI Row",
			},
			"row_4x2000.json",
		},
		{
			"set rowing title: 4x2000 - the other one",
			&strava.UpdatableActivity{
				Name: "4x 2000m w/5' Active RI Row",
			},
			"row_4x2000_2.json",
		},
		{
			"set rowing title: 4x1000",
			&strava.UpdatableActivity{
				Name: "4x 1000m /5' RI Row",
			},
			"row_4x1000.json",
		},
		{
			"set rowing title: waterfall",
			&strava.UpdatableActivity{
				Name: "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row",
			},
			"row_waterfall.json",
		},
		{
			"set rowing title: waterfall - the other one",
			&strava.UpdatableActivity{
				Name: "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row",
			},
			"row_waterfall_2.json",
		},
		{
			"set rowing title: warmup",
			&strava.UpdatableActivity{
				Name:         "Warm-up Row",
				HideFromHome: true,
			},
			"row_warmup.json",
		},
		{
			"add weather to pop'd description",
			&strava.UpdatableActivity{
				Name:         "Warm-up Row",
				HideFromHome: true,
				Description:  "Test activity description\n\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n",
			},
			"row_add_weather.json",
		},
		{
			"set rowing title from first line of description",
			&strava.UpdatableActivity{
				Name:        "5x 1.5k w/ 5' Active RI",
				Description: "\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n",
			},
			"row_title_from_first_line.json",
		},
		{
			"add weather to outdoor activity",
			&strava.UpdatableActivity{
				GearID:      "b10013574",
				Description: "Outside ride description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n",
			},
			"outside_ride_add_weather.json",
		},
		{
			"adds weather for pain cave for virtual rides",
			&strava.UpdatableActivity{
				GearID:      "b9880609",
				Trainer:     true,
				Description: "Test virtualride description\n\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n",
			},
			"virtualride.json",
		},
		{
			"adds summit total for run",
			&strava.UpdatableActivity{
				Description: "Test run description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n\n🦶⬆️ 1000m\n",
			},
			"summit_add_for_run.json",
		},
		{
			"adds summit total for ride",
			&strava.UpdatableActivity{
				GearID:      "b10013574",
				Description: "Outside ride description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n\n🚴‍♂️⬆️ 1234m\n",
			},
			"summit_add_for_ride.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a strava.Activity
			var resp []byte
			db := setupTestDB(t)
			// TODO: Hacky AF - replace me
			if strings.HasPrefix(tc.fixture, "trainerroad") {
				resp, _ = os.ReadFile("testdata/trainerroad.ics")
			}

			mockClient := &MockClient{
				DoFunc: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(string(resp))),
					}, nil
				},
			}
			trcal := calendarevent.NewCalendarService(mockClient, "test", "test")
			activity, _ := os.ReadFile("testdata/" + tc.fixture)
			err := json.Unmarshal(activity, &a)
			if err != nil {
				t.Errorf("unexpected error parsing test input: %v", err)
			}

			if strings.HasPrefix(tc.fixture, "summit") {
				runGain := 0.0
				rideGain := 0.0
				if a.Type == "Run" {
					runGain = a.TotalElevationGain
				}
				if a.Type == "Ride" {
					rideGain = a.TotalElevationGain
				}
				db.Create(&model.Summit{
					AthleteID: int64(1),
					Year:      int64(a.StartDate.Year()),
					Run:       runGain,
					Ride:      rideGain,
				})
			}

			got, _ := constructUpdate(context.Background(), rc, &a, trcal, db)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %+v, got %+v", tc.want, got)
			}
		})
	}
}

// Setup establishes a test Server that can be used to provide mock responses during testing.
// It returns a pointer to a client, a mux, the server URL and a teardown function that
// must be called when testing is complete.
func setup() (rc *client.Client, mux *http.ServeMux, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	surl, _ := url.Parse(server.URL + "/")
	c := client.NewClient(surl, nil)

	return c, mux, server.Close
}
