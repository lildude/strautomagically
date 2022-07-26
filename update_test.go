package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	ic "github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/strava"
)

func TestUpdateHandler(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(ioutil.Discard)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	oat := `{"access_token":"123456789","token_type":"Bearer","refresh_token":"987654321","expiry":"2022-07-12T18:30:36.917400827Z"}`

	httpmock.RegisterResponder("POST", "https://www.strava.com/oauth/token",
		httpmock.NewStringResponder(200, oat))

	httpmock.RegisterResponder("GET", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, `{"id": 123, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "garmin_push_12345678987654321", "type": "Ride", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description"}`))

	httpmock.RegisterResponder("PUT", `=~^https://www\.strava\.com/api/v3/activities/\d+\z`,
		httpmock.NewStringResponder(200, `{"id": 123, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "garmin_push_12345678987654321", "type": "Ride", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description"}`))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/3.0/onecall/timemachine",
		httpmock.NewStringResponder(200, `{"data":[{"temp":19.13,"feels_like":16.44,"humidity":64,"clouds":0,"wind_speed":3.6,"wind_deg":340,"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}]}]}`))

	httpmock.RegisterResponder("GET", "https://api.openweathermap.org/data/2.5/air_pollution/history",
		httpmock.NewStringResponder(200, `{"list":[{"dt":1605182400,"main":{"aqi":1}}]}`))

	tests := []struct {
		name  string
		body  string
		redis []string // Used to seed Redis with the expected values for the tests
		want  int
	}{
		{
			"no body",
			``,
			[]string{"", ""},
			400,
		},
		{
			"invalid JSON in body",
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
			[]string{oat, "123"},
			200,
		},
		{
			"create event",
			`{"aspect_type": "create", "object_id": 456}`,
			[]string{oat, ""},
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

			req, err := http.NewRequest("GET", "/webhook", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(updateHandler) // Fudging it as webhookHandler handles /webhook but calls updateHandler if it receives a POST request
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.want {
				t.Errorf("%s: handler returned wrong status code: got %d want %d", tc.name, status, tc.want)
			}
		})
	}
}

func TestConstructUpdate(t *testing.T) {
	client, mux, _, _ := setup()
	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"data":[{"temp":19.13,"feels_like":16.44,"humidity":64,"clouds":0,"wind_speed":3.6,"wind_deg":340,"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}]}]}`
		fmt.Fprintln(w, resp)
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"list":[{"dt":1605182400,"main":{"aqi":5}}]}`
		fmt.Fprintln(w, resp)
	})

	tests := []struct {
		name     string
		want     *strava.UpdatableActivity
		wantLog  string
		activity []byte
	}{
		{
			"no changes",
			&strava.UpdatableActivity{},
			"nothing to do\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "garmin_push_12345678987654321", "type": "Run", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set gear and mute walks",
			&strava.UpdatableActivity{
				HideFromHome: true,
				GearID:       "g10043849",
			},
			"muted walk\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "garmin_push_12345678987654321", "type": "Walk", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set humane burpees title and mute",
			&strava.UpdatableActivity{
				Name:         "Humane Burpees",
				HideFromHome: true,
			},
			"set humane burpees title\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 200, "external_id": "garmin_push_12345678987654321", "type": "WeightTraining", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"prefix and set get for TrainerRoad activities",
			&strava.UpdatableActivity{
				Name:    "TR: Test Activity",
				GearID:  "b9880609",
				Trainer: true,
			},
			"prefixed name of ride with TR and set gear to trainer\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "trainerroad_12345678987654321", "type": "Ride", "trainer": true, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set gear to trainer for Zwift activities",
			&strava.UpdatableActivity{
				GearID:  "b9880609",
				Trainer: true,
			},
			"set gear to trainer\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "VirtualRide", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set get to bike",
			&strava.UpdatableActivity{
				GearID: "b10013574",
			},
			"set gear to bike\n",
			[]byte(`{"id": 12345678987654321, "name": "Test Activity", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Ride", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: speed pyramid",
			&strava.UpdatableActivity{
				Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work",
			},
			"set title to Speed Pyramid Row w/ 1.5' Active RI per 250m work\n",
			[]byte(`{"id": 12345678987654321, "name": "v250m/1:30r...7 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: speed pyramid - the other one",
			&strava.UpdatableActivity{
				Name: "Speed Pyramid Row w/ 1.5' Active RI per 250m work",
			},
			"set title to Speed Pyramid Row w/ 1.5' Active RI per 250m work\n",
			[]byte(`{"id": 12345678987654321, "name": "v5:00/1:00r...15 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 8x500",
			&strava.UpdatableActivity{
				Name: "8x 500m w/ 3.5' Active RI Row",
			},
			"set title to 8x 500m w/ 3.5' Active RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "8x500m/3:30r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 8x500 - the other one",
			&strava.UpdatableActivity{
				Name: "8x 500m w/ 3.5' Active RI Row",
			},
			"set title to 8x 500m w/ 3.5' Active RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "v5:00/1:00r...17 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 5x1500",
			&strava.UpdatableActivity{
				Name: "5x 1500m w/ 5' RI Row",
			},
			"set title to 5x 1500m w/ 5' RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "5x1500m/5:00r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 4x2000",
			&strava.UpdatableActivity{
				Name: "4x 2000m w/5' Active RI Row",
			},
			"set title to 4x 2000m w/5' Active RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "4x2000m/5:00r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 4x2000 - the other one",
			&strava.UpdatableActivity{
				Name: "4x 2000m w/5' Active RI Row",
			},
			"set title to 4x 2000m w/5' Active RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "v5:00/1:00r...9 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 4x1000",
			&strava.UpdatableActivity{
				Name: "4x 1000m /5' RI Row",
			},
			"set title to 4x 1000m /5' RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "4x1000m/5:00r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: waterfall",
			&strava.UpdatableActivity{
				Name: "Waterfall of 3k, 2.5k, 2k w/ 5' RI Row",
			},
			"set title to Waterfall of 3k, 2.5k, 2k w/ 5' RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "v3000m/5:00r...3 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: waterfall",
			&strava.UpdatableActivity{
				Name: "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row",
			},
			"set title to Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "v5:00/1:00r...7 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: warmup",
			&strava.UpdatableActivity{
				Name:         "Warm-up Row",
				HideFromHome: true,
			},
			"set title to Warm-up Row\n",
			[]byte(`{"id": 12345678987654321, "name": "5:00 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"add weather to pop'd description",
			&strava.UpdatableActivity{
				Name:         "Warm-up Row",
				HideFromHome: true,
				Description:  "Test activity description\n\n☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | 💨 14-14km/h ↓ | AQI 🖤\n",
			},
			"set title to Warm-up Row & added weather\n",
			[]byte(`{"id": 12345678987654321, "name": "5:00 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description"}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use a faux logger so we can parse the content to find our debug messages to confirm our tests
			var fauxLog bytes.Buffer
			log.SetFlags(0)
			log.SetOutput(&fauxLog)

			var a strava.Activity
			err := json.Unmarshal(tc.activity, &a)
			if err != nil {
				t.Errorf("unexpected error parsing test input: %v", err)
			}

			got := constructUpdate(client, &a)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
			if fauxLog.String() != tc.wantLog {
				t.Errorf("expected %q, got %q", tc.wantLog, fauxLog.String())
			}
		})
	}
}

// Setup establishes a test Server that can be used to provide mock responses during testing.
// It returns a pointer to a client, a mux, the server URL and a teardown function that
// must be called when testing is complete.
func setup() (client *client.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	url, _ := url.Parse(server.URL + "/")
	c := ic.NewClient(url, nil)

	return c, mux, server.URL, server.Close
}
