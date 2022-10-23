package update

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jarcoal/httpmock"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/strava"
)

func TestUpdateHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

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
		redis       []string // Used to seed Redis with the expected values for the tests
		wantStatus  int
	}{
		{
			"no webhook body",
			``,
			[]string{"", ""},
			400,
		},
		{
			"invalid JSON in webhook body",
			`{"foo: "bar"}`,
			[]string{"", ""},
			400,
		},
		{
			"non-create event",
			`{"aspect_type": "update"}`,
			[]string{"", ""},
			200,
		},
		{
			"unresponsive redis",
			`{"aspect_type": "create", "object_id": 123}`,
			[]string{"", ""},
			500,
		},
		{
			"repeat event",
			`{"aspect_type": "create", "object_id": 123}`,
			[]string{token, "123"},
			200,
		},
		{
			"create event",
			`{"aspect_type": "create", "object_id": 456}`,
			[]string{token, ""},
			200,
		},
	}

	r := miniredis.RunT(t)
	defer r.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Pre-populate Redis with the expected values, if set, and set REDIS_URL to use the miniredis instance
			if tc.redis[0] != "" {
				os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
				r.Set("strava_auth_token", tc.redis[0]) //nolint:errcheck
				r.Set("strava_activity", tc.redis[1])   //nolint:errcheck
			} else {
				os.Setenv("REDIS_URL", "foobar") // Forces a quick failure mimicking a non-existent Redis instance
			}

			req, err := http.NewRequest("GET", "/webhook", strings.NewReader(tc.webhookBody)) //nolint:noctx
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

func TestConstructUpdate(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

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
			"set gear and mute walks",
			&strava.UpdatableActivity{
				HideFromHome: true,
				GearID:       "g10043849",
			},
			"walks.json",
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
			"prefix and set get for TrainerRoad activities",
			&strava.UpdatableActivity{
				Name:    "TR: Test Activity",
				GearID:  "b9880609",
				Trainer: true,
			},
			"trainerroad.json",
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
			"set get to bike",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a strava.Activity
			activity, _ := os.ReadFile("testdata/" + tc.fixture)
			err := json.Unmarshal(activity, &a)
			if err != nil {
				t.Errorf("unexpected error parsing test input: %v", err)
			}

			got := constructUpdate(rc, &a)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
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
