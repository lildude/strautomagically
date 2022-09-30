package strava

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
	"testing"

	"github.com/lildude/strautomagically/internal/client"
)

func TestGetActivity(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	resp, _ := os.ReadFile("testdata/activity.json")
	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, string(resp))
	})

	want := &Activity{}
	json.Unmarshal(resp, want) //nolint:errcheck

	got, err := GetActivity(rc, 12345678987654321)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestGetActivityError(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := GetActivity(rc, 12345678987654321)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestUpdateActivity(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	resp, _ := os.ReadFile("testdata/updated_activity.json")
	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, string(resp))
	})

	want := &Activity{}
	json.Unmarshal(resp, want) //nolint:errcheck

	update := &UpdatableActivity{
		Name:         "Test Activity - Updated",
		Commute:      true,
		Trainer:      true,
		HideFromHome: true,
		Description:  "Test activity description - Updated",
		Type:         "Run",
		GearID:       "b1234",
	}

	got, err := UpdateActivity(rc, 12345678987654321, update)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestUpdateActivityError(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := UpdateActivity(rc, 12345678987654321, &UpdatableActivity{})
	if err == nil {
		t.Error("expected error, got nil")
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
