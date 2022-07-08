package strava

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/lildude/strautomagically/internal/client"
	gc "github.com/lildude/strautomagically/internal/client"
)

func TestGetActivity(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	resp := `{"id" : 12345678987654321, "name" : "Test Activity", "distance": 28099, "start_date" : "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time" : 4410, "external_id: "garmin_push_12345678987654321", "type": "Ride", "trainer": false, "commute": false, "private": false, "workout_type": 10, "hide_from_home": false, "gear_id": "b12345678987654321", "description": "Test activity description"}`

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, resp)
	})

	want := &Activity{}
	json.Unmarshal([]byte(resp), want) //nolint:errcheck

	got, err := GetActivity(client, 12345678987654321)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestGetActivityError(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(ioutil.Discard)

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := GetActivity(client, 12345678987654321)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestUpdateActivity(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	resp := `{"id" : 12345678987654321, "name" : "Test Activity - Updated", "distance": 28099, "start_date" : "2018-02-16T14:52:54Z", "start_date_local": "2018-02-16T06:52:54Z", "elapsed_time" : 4410, "type": "Run", "trainer": true, "commute": true, "private": false, "workout_type": 10, "hide_from_home": true, "gear_id": "b1234", "description": "Test activity description - Updated"}`
	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, resp)
	})

	want := &Activity{}
	json.Unmarshal([]byte(resp), want) //nolint:errcheck

	update := &UpdatableActivity{
		Name:         "Test Activity - Updated",
		Commute:      true,
		Trainer:      true,
		HideFromHome: true,
		Description:  "Test activity description - Updated",
		Type:         "Run",
		GearID:       "b1234",
	}

	got, err := UpdateActivity(client, 12345678987654321, update)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestUpdateActivityError(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(ioutil.Discard)

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := UpdateActivity(client, 12345678987654321, &UpdatableActivity{})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Setup establishes a test Server that can be used to provide mock responses during testing.
// It returns a pointer to a client, a mux, the server URL and a teardown function that
// must be called when testing is complete.
func setup() (client *client.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	url, _ := url.Parse(server.URL + "/")
	c := gc.NewClient(url, nil)

	return c, mux, server.URL, server.Close
}
