package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	json.Unmarshal(resp, want)

	got, err := GetActivity(context.Background(), rc, 12345678987654321)
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
	slog.SetDefault(slog.New(slog.DiscardHandler))

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := GetActivity(context.Background(), rc, 12345678987654321)
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
	json.Unmarshal(resp, want)

	update := &UpdatableActivity{
		Name:         "Test Activity - Updated",
		Commute:      true,
		Trainer:      true,
		HideFromHome: true,
		Description:  "Test activity description - Updated",
		Type:         "Run",
		GearID:       "b1234",
	}

	got, err := UpdateActivity(context.Background(), rc, 12345678987654321, update)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestUpdateActivitySportType(t *testing.T) {
	tests := []struct {
		name   string
		update UpdatableActivity
		want   map[string]any
	}{
		{
			name:   "convert to virtual rowing alongside other changes",
			update: UpdatableActivity{SportType: "VirtualRow", Name: "Warm-up Row", HideFromHome: true},
			want:   map[string]any{"sport_type": "VirtualRow", "name": "Warm-up Row", "hide_from_home": true},
		},
		{
			name:   "omit sport type when unchanged",
			update: UpdatableActivity{Name: "Updated activity"},
			want:   map[string]any{"name": "Updated activity"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, mux, teardown := setup()
			defer teardown()

			var got map[string]any
			mux.HandleFunc("/api/v3/activities/123", func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decoding update request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			if _, err := UpdateActivity(context.Background(), rc, 123, &tc.update); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected payload %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestUpdateActivityError(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	// Discard logs to avoid polluting test output
	slog.SetDefault(slog.New(slog.DiscardHandler))

	mux.HandleFunc("/api/v3/activities/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := UpdateActivity(context.Background(), rc, 12345678987654321, &UpdatableActivity{})
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
