package weather

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lildude/strautomagically/internal/client"
	gc "github.com/lildude/strautomagically/internal/client"
)

func TestGetWeatherSameHour(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	lat_in := "51.509865"
	lon_in := "-0.118092"
	appid_in := "123456789"
	os.Setenv("OWM_LAT", lat_in)
	os.Setenv("OWM_LON", lon_in)
	os.Setenv("OWM_API_KEY", appid_in)
	start_in := time.Date(2006, 1, 2, 15, 0o4, 0o5, 0, time.UTC)
	start_out := strconv.FormatInt(start_in.Unix(), 10)
	elapsed := int32(60 * 40)
	end_in := start_in.Add(time.Duration(elapsed) * time.Second)
	end_out := strconv.FormatInt(end_in.Unix(), 10)

	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		lat := q.Get("lat")
		lon := q.Get("lon")
		appid := q.Get("appid")
		units := q.Get("units")
		lang := q.Get("lang")
		dt := q.Get("dt")
		// Confirm we receive the right query params
		if lat != lat_in || lon != lon_in || appid != appid_in || units != "metric" || lang != "en" || dt != start_out {
			t.Errorf("Expected lat=%s, lon=%s, appid=%s, units=metric, lang=en, dt=%s, got lat=%s, lon=%s, appid=%s, units=%s, lang=%s, dt=%s", lat_in, lon_in, appid_in, start_out, lat, lon, appid, units, lang, dt)
		}

		resp := `{"data":[{"temp":19.13,"feels_like":16.44,"humidity":64,"clouds":0,"wind_speed":3.6,"wind_deg":340,"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}]}]}`
		fmt.Fprintln(w, resp)
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		lat := q.Get("lat")
		lon := q.Get("lon")
		appid := q.Get("appid")
		units := q.Get("units")
		start := q.Get("start")
		end := q.Get("end")
		// Confirm we receive the right query params
		if lat != lat_in || lon != lon_in || appid != appid_in || units != "metric" || start != start_out || end != end_out {
			t.Errorf("Expected lat=%s, lon=%s, appid=%s, units=metric, start=%s, end=%s, got lat=%s, lon=%s, appid=%s, units=%s, start=%s, end=%s", lat_in, lon_in, appid_in, start_out, end_out, lat, lon, appid, units, start, end)
		}

		resp := `{"list":[{"dt":1605182400,"main":{"aqi":1}}]}`
		fmt.Fprintln(w, resp)
	})

	got := GetWeather(client, start_in, elapsed)
	want := "☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | 💨 14-14km/h ↓ | AQI 💚\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestGetWeatherDiffHours(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	lat_in := "51.509865"
	lon_in := "-0.118092"
	appid_in := "123456789"
	os.Setenv("OWM_LAT", lat_in)
	os.Setenv("OWM_LON", lon_in)
	os.Setenv("OWM_API_KEY", appid_in)
	start_in := time.Date(2006, 1, 2, 15, 0o4, 0o5, 0, time.UTC)
	start_out := strconv.FormatInt(start_in.Unix(), 10)
	elapsed := int32(60 * 65)
	end_in := start_in.Add(time.Duration(elapsed) * time.Second)
	end_out := strconv.FormatInt(end_in.Unix(), 10)

	// Handle start request
	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		dt := r.URL.Query().Get("dt")

		// Return response for first request
		var resp string
		if dt == start_out {
			resp = `{"data":[{"temp":19.13,"feels_like":16.44,"humidity":64,"clouds":0,"wind_speed":3.6,"wind_deg":340,"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}]}]}`
		}
		if dt == end_out {
			resp = `{"data":[{"temp":23.13,"feels_like":26.44,"humidity":94,"clouds":13,"wind_speed":0.6,"wind_deg":40,"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}]}]}`
		}
		fmt.Fprintln(w, resp)
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		start := q.Get("start")
		end := q.Get("end")
		// Confirm we receive the right query params
		if start != start_out || end != end_out {
			t.Errorf("Expected start=%s, end=%s, got start=%s, end=%s", start_out, end_out, start, end)
		}

		resp := `{"list":[{"dt":1605182400,"main":{"aqi":1}}]}`
		fmt.Fprintln(w, resp)
	})

	got := GetWeather(client, start_in, elapsed)
	want := "☀️ Clear Sky | 🌡 19-23°C | 👌 16°C | 💦 64-94% | 💨 14-3km/h ↓ | AQI 💚\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestWindDirectionIcon(t *testing.T) {
	tests := []struct {
		degrees int
		want    string
	}{
		{0, "↓"},
		{45, "↙️"},
		{90, "←"},
		{135, "↖️"},
		{180, "↑"},
		{225, "↗️"},
		{270, "→"},
		{315, "↘️"},
		{360, "↓"},
		{-1, ""},
		{361, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.degrees), func(t *testing.T) {
			got := windDirectionIcon(tt.degrees)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
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
	c := gc.NewClient(url, nil)

	return c, mux, server.URL, server.Close
}
