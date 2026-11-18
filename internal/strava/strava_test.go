package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
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

func TestActivity64BitIDs(t *testing.T) {
	for _, id := range []int64{1<<53 + 1, math.MaxInt64} {
		t.Run(strconv.FormatInt(id, 10), func(t *testing.T) {
			rc, mux, teardown := setup()
			defer teardown()

			mux.HandleFunc(fmt.Sprintf("/api/v3/activities/%d", id), func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"id":%d,"id_str":"%d"}`, id, id)
			})

			for _, method := range []string{http.MethodGet, http.MethodPut} {
				t.Run(method, func(t *testing.T) {
					var got *Activity
					var err error
					if method == http.MethodGet {
						got, err = GetActivity(t.Context(), rc, id)
					} else {
						got, err = UpdateActivity(t.Context(), rc, id, &UpdatableActivity{Name: "Updated"})
					}
					if err != nil {
						t.Fatal(err)
					}
					if got.ID != id {
						t.Errorf("activity ID = %d, want %d", got.ID, id)
					}
				})
			}
		})
	}
}

func TestWebhook64BitIDs(t *testing.T) {
	for _, id := range []int64{1<<53 + 1, math.MaxInt64} {
		t.Run(strconv.FormatInt(id, 10), func(t *testing.T) {
			body := fmt.Sprintf(`{"object_id":%d,"owner_id":%d}`, id, id)
			var webhook WebhookPayload
			if err := json.Unmarshal([]byte(body), &webhook); err != nil {
				t.Fatal(err)
			}
			if webhook.ObjectID != id || webhook.OwnerID != id {
				t.Errorf("webhook IDs = (%d, %d), want (%d, %d)", webhook.ObjectID, webhook.OwnerID, id, id)
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
