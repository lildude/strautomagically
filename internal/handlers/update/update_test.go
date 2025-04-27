package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgtype"
	"github.com/jarcoal/httpmock"
	"github.com/lildude/strautomagically/internal/calendarevent"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(&model.Athlete{}, &model.Summit{})
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestUpdateHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	logrus.SetOutput(io.Discard)

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

	tests := []struct {
		name        string
		webhookBody string
		wantStatus  int
	}{
		{
			name:        "No webhook body",
			webhookBody: ``,
			wantStatus:  400,
		},
		{
			name:        "Invalid JSON in webhook body",
			webhookBody: `{"foo: "bar"}`,
			wantStatus:  400,
		},
		{
			name:        "Non-create event",
			webhookBody: `{"aspect_type": "update"}`,
			wantStatus:  200,
		},
		{
			name:        "Repeat event",
			webhookBody: `{"owner_id": 1, "aspect_type": "create", "object_id": 123}`,
			wantStatus:  200,
		},
		{
			name:        "Create event",
			webhookBody: `{"owner_id": 1, "aspect_type": "create", "object_id": 456}`,
			wantStatus:  200,
		},
	}

	db := setupTestDB(t)
	database.SetTestDB(db)

	tokenJSON := pgtype.JSONB{}
	tokenJSON.Set(map[string]string{"access_token": "123456789"})
	db.Create(&model.Athlete{
		StravaAthleteID:   1,
		StravaAthleteName: "test",
		StravaAuthToken:   tokenJSON,
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

type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestConstructUpdate(t *testing.T) {
	// Discard logs to avoid polluting test output
	logrus.SetOutput(io.Discard)

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
			name:    "No changes",
			want:    &strava.UpdatableActivity{},
			fixture: "no_change.json",
		},
		{
			name:    "Unhandled activity type",
			want:    &strava.UpdatableActivity{},
			fixture: "handcycle.json",
		},
		{
			name:    "Set gear and mute walks",
			want:    &strava.UpdatableActivity{HideFromHome: true, GearID: "g10043849"},
			fixture: "walks.json",
		},
		{
			name:    "Set humane burpees title and mute",
			want:    &strava.UpdatableActivity{Name: "Humane Burpees", HideFromHome: true},
			fixture: "humane_burpees.json",
		},
		{
			name:    "Prefix and set title from TrainerRoad calendar for TrainerRoad activities",
			want:    &strava.UpdatableActivity{Name: "TR: Capulin", GearID: "b9880609", Trainer: true},
			fixture: "trainerroad.json",
		},
		{
			name:    "Prefix and set title from TrainerRoad calendar for outside ride activities",
			want:    &strava.UpdatableActivity{Name: "TR: Capulin - Outside", GearID: "b10013574", Trainer: false},
			fixture: "trainerroad_outside.json",
		},
		{
			name:    "Set gear to trainer for Zwift activities",
			want:    &strava.UpdatableActivity{GearID: "b9880609", Trainer: true},
			fixture: "zwift.json",
		},
		{
			name:    "Set gear to Ride",
			want:    &strava.UpdatableActivity{GearID: "b10013574"},
			fixture: "ride.json",
		},
		{
			name:    "Set rowing title: speed pyramid",
			want:    &strava.UpdatableActivity{Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work"},
			fixture: "row_speed_pyramid.json",
		},
		{
			name:    "Set rowing title: speed pyramid - the other one",
			want:    &strava.UpdatableActivity{Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work"},
			fixture: "row_speed_pyramid_2.json",
		},
		{
			name:    "Set rowing title: 8x500",
			want:    &strava.UpdatableActivity{Name: "8x 500m w/ 3.5' Active RI Row"},
			fixture: "row_8x500.json",
		},
		{
			name:    "Set rowing title: 8x500 - the other one",
			want:    &strava.UpdatableActivity{Name: "8x 500m w/ 3.5' Active RI Row"},
			fixture: "row_8x500_2.json",
		},
		{
			name:    "Set rowing title: 5x1500",
			want:    &strava.UpdatableActivity{Name: "5x 1500m w/ 5' RI Row"},
			fixture: "row_5x1500.json",
		},
		{
			name:    "Set rowing title: 4x2000",
			want:    &strava.UpdatableActivity{Name: "4x 2000m w/5' Active RI Row"},
			fixture: "row_4x2000.json",
		},
		{
			name:    "Set rowing title: 4x2000 - the other one",
			want:    &strava.UpdatableActivity{Name: "4x 2000m w/5' Active RI Row"},
			fixture: "row_4x2000_2.json",
		},
		{
			name:    "Set rowing title: 4x1000",
			want:    &strava.UpdatableActivity{Name: "4x 1000m /5' RI Row"},
			fixture: "row_4x1000.json",
		},
		{
			name:    "Set rowing title: waterfall",
			want:    &strava.UpdatableActivity{Name: "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row"},
			fixture: "row_waterfall.json",
		},
		{
			name:    "Set rowing title: waterfall - the other one",
			want:    &strava.UpdatableActivity{Name: "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row"},
			fixture: "row_waterfall_2.json",
		},
		{
			name:    "Set rowing title: warmup",
			want:    &strava.UpdatableActivity{Name: "Warm-up Row", HideFromHome: true},
			fixture: "row_warmup.json",
		},
		{
			name:    "Add weather to pop'd description",
			want:    &strava.UpdatableActivity{Name: "Warm-up Row", HideFromHome: true, Description: "Test activity description\n\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n"},
			fixture: "row_add_weather.json",
		},
		{
			name:    "Set rowing title from first line of description",
			want:    &strava.UpdatableActivity{Name: "5x 1.5k w/ 5' Active RI", Description: "\n\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n"},
			fixture: "row_title_from_first_line.json",
		},
		{
			name:    "Add weather to outdoor activity",
			want:    &strava.UpdatableActivity{GearID: "b10013574", Description: "Outside ride description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n"},
			fixture: "outside_ride_add_weather.json",
		},
		{
			name:    "Adds weather for pain cave for virtual rides",
			want:    &strava.UpdatableActivity{GearID: "b9880609", Trainer: true, Description: "Test virtualride description\n\nThe Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n"},
			fixture: "virtualride.json",
		},
		{
			name:    "Adds summit total for run",
			want:    &strava.UpdatableActivity{Description: "Test run description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n\n🦶⬆️ 1000m\n"},
			fixture: "summit_add_for_run.json",
		},
		{
			name:    "Adds summit total for ride",
			want:    &strava.UpdatableActivity{GearID: "b10013574", Description: "Outside ride description\n\nOn the road: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n\n🚴‍♂️⬆️ 1234m\n"},
			fixture: "summit_add_for_ride.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a strava.Activity
			var resp []byte
			db := setupTestDB(t)
			database.SetTestDB(db)

			// TODO: Hacky AF - replace me
			if strings.HasPrefix(tc.fixture, "trainerroad") {
				resp, _ = os.ReadFile("testdata/trainerroad.ics")
			}

			if strings.HasPrefix(tc.fixture, "summit") {
				// Read in the fixture file and unmarshal the JSON
				resp, _ = os.ReadFile("testdata/" + tc.fixture)
				err := json.Unmarshal(resp, &a)
				if err != nil {
					t.Fatalf("unexpected error parsing test input: %v", err)
				}
				runGain := 0.0
				rideGain := 0.0
				if a.Type == "Ride" {
					rideGain = a.TotalElevationGain
				}
				if a.Type == "Run" {
					runGain = a.TotalElevationGain
				}

				// Create a summit record for the test using the total_elevation_gain from the activity
				summit := &model.Summit{
					AthleteID: int64(1),
					Year:      int64(a.StartDate.Year()),
					Run:       runGain,
					Ride:      rideGain,
				}

				db.Create(summit)
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

			got, _ := constructUpdate(rc, &a, trcal, db)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("\nexpected %+v, \ngot %+v", tc.want, got)
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
