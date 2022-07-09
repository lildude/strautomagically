package main

import (
	"bytes"
	"encoding/json"
	"log"
	"reflect"
	"testing"

	"github.com/lildude/strautomagically/internal/strava"
)

func TestConstructUpdate(t *testing.T) {
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
				Name: "Speed Pyramid Row w/ 1.5' RI per 250m work",
			},
			"set title to Speed Pyramid Row w/ 1.5' RI per 250m work\n",
			[]byte(`{"id": 12345678987654321, "name": "v250m/1:30r...7 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		{
			"set rowing title: 8x500",
			&strava.UpdatableActivity{
				Name: "8x 500m w/ 3.5' RI Row",
			},
			"set title to 8x 500m w/ 3.5' RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "8x500m/3:30r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
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
			"set rowing title: 4x200",
			&strava.UpdatableActivity{
				Name: "4x 2000m w/5' RI Row",
			},
			"set title to 4x 2000m w/5' RI Row\n",
			[]byte(`{"id": 12345678987654321, "name": "4x2000m/5:00r row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
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
			"set rowing title: warmup",
			&strava.UpdatableActivity{
				Name:         "Warm-up Row",
				HideFromHome: true,
			},
			"set title to Warm-up Row\n",
			[]byte(`{"id": 12345678987654321, "name": "5:00 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description\n AQI: ?\n"}`),
		},
		// {
		// 	"add weather to pop'd description",
		// 	&strava.UpdatableActivity{
		// 		Name:         "Warm-up Row",
		// 		HideFromHome: true,
		// 		Description:  "Test activity description\n\n☀️ Clear Sky | 🌡 0-0°C | 👌 -3°C | 💦 96-97% | 💨 10-10km/h ↗️ | AQI 🖤\n",
		// 	},
		// 	"set title to Warm-up Row & added weather\n",
		// 	[]byte(`{"id": 12345678987654321, "name": "5:00 row", "distance": 28099, "start_date": "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time": 4410, "external_id": "zwift_12345678987654321", "type": "Rowing", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description"}`),
		// },
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

			got := constructUpdate(&a)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
			if fauxLog.String() != tc.wantLog {
				t.Errorf("expected %q, got %q", tc.wantLog, fauxLog.String())
			}
		})
	}
}

func TestUpdateHandler(t *testing.T) {
	// skip until we've refactored
	t.SkipNow()
}
