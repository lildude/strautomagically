package weather

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/lildude/strautomagically/internal/client"
)

func TestGetWeather(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	latIn := "51.509865"
	lonIn := "-0.118092"
	appIDIn := "123456789"
	t.Setenv("OWM_LAT", latIn)
	t.Setenv("OWM_LON", lonIn)
	t.Setenv("OWM_API_KEY", appIDIn)
	startIn := time.Date(2006, 1, 2, 15, 0o4, 0o5, 0, time.UTC).Unix()
	startOut := strconv.FormatInt(startIn, 10)

	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		lat := q.Get("lat")
		lon := q.Get("lon")
		appid := q.Get("appid")
		units := q.Get("units")
		lang := q.Get("lang")
		dt := q.Get("dt")
		// Confirm we receive the right query params
		if lat != latIn || lon != lonIn || appid != appIDIn || units != "metric" || lang != "en" || dt != startOut {
			t.Errorf(
				"Expected lat=%s, lon=%s, appid=%s, units=metric, lang=en, dt=%s, got lat=%s, lon=%s, appid=%s, units=%s, lang=%s, dt=%s",
				latIn, lonIn, appIDIn, startOut, lat, lon, appid, units, lang, dt,
			)
		}

		resp, _ := os.ReadFile("testdata/weather.json")
		fmt.Fprintln(w, string(resp))
	})

	got, err := getWeather(rc, startIn)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	want := data{
		Temp:      19.13,
		FeelsLike: 16.44,
		Humidity:  64,
		WindSpeed: 3.6,
		WindDeg:   340,
		Weather: []weather{
			{
				Main:        "Clear",
				Description: "clear sky",
				Icon:        "01d",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestGetWeatherWithErrorReturnsEmptyStruct(t *testing.T) {
	rc, _, teardown := setup() // We're not using the mux as we'll be failing before then
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	got, err := getWeather(rc, 0)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	want := data{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected empty struct, got %v", got)
	}
}

func TestGetWeatherLineSameHour(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	startIn := time.Date(2006, 1, 2, 15, 0o4, 0o5, 0, time.UTC)
	elapsed := int32(60 * 40)

	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		resp, _ := os.ReadFile("testdata/weather.json")
		fmt.Fprintln(w, string(resp))
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"list":[{"dt":1605182401,"main":{"aqi":1}}]}`
		fmt.Fprintln(w, resp)
	})

	got, err := GetWeatherLine(rc, startIn, elapsed)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	want := "The Pain Cave: ☀️ Clear Sky | 🌡 19-19°C | 👌 16°C | 💦 64-64% | AQI 💚\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestGetWeatherLineDiffHours(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	startIn := time.Date(2006, 1, 2, 15, 0o4, 0o5, 0, time.UTC)
	startOut := strconv.FormatInt(startIn.Unix(), 10)
	elapsed := int32(60 * 65)
	endIn := startIn.Add(time.Duration(elapsed) * time.Second)
	endOut := strconv.FormatInt(endIn.Unix(), 10)

	// Handle start request
	mux.HandleFunc("/data/3.0/onecall/timemachine", func(w http.ResponseWriter, r *http.Request) {
		dt := r.URL.Query().Get("dt")

		// Return response for first request
		var resp []byte
		if dt == startOut {
			resp, _ = os.ReadFile("testdata/weather.json")
		}
		if dt == endOut {
			resp, _ = os.ReadFile("testdata/weather2.json")
		}
		fmt.Fprintln(w, string(resp))
	})

	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		start := q.Get("start")
		end := q.Get("end")
		// Confirm we receive the right query params
		if start != startOut || end != endOut {
			t.Errorf("Expected start=%s, end=%s, got start=%s, end=%s", startOut, endOut, start, end)
		}

		resp := `{"list":[{"dt":1605182400,"main":{"aqi":1}}]}`
		fmt.Fprintln(w, resp)
	})

	got, err := GetWeatherLine(rc, startIn, elapsed)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	want := "The Pain Cave: ☀️ Clear Sky | 🌡 19-23°C | 👌 16°C | 💦 64-94% | AQI 💚\n"
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
		{45, "↙"},
		{90, "←"},
		{135, "↖"},
		{180, "↑"},
		{225, "↗"},
		{270, "→"},
		{315, "↘"},
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

func TestGetPollutionForAllLevels(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	tests := []struct {
		mockAQI int
		want    string
	}{ // Not sure why I can't do direct emoji comparison here
		{1, `\U1F49A`}, // 💚
		{2, `\U1F49B`}, // 💛
		{3, `\U1F9E1`}, // 🧡
		{4, `\U1F90E`}, // 🤎
		{5, `\U1F5A4`}, // 🖤
	}
	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("start")
		aqi, _ := strconv.Atoi(q)
		resp := fmt.Sprintf(`{"list":[{"dt":1605182400,"main":{"aqi":%d}}]}`, aqi)
		fmt.Fprintln(w, resp)
	})

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.mockAQI), func(t *testing.T) {
			// Mock the aqi by fudging the start_date as we don't care about it in this test
			got := getPollution(rc, int64(tt.mockAQI), 123999)
			if got == tt.want {
				t.Errorf("aqi %d expected %q, got %q", tt.mockAQI, tt.want, got)
			}
		})
	}
}

func TestGetPollutionWithErrorReturnsQuestionMark(t *testing.T) {
	rc, _, teardown := setup() // We're not using the mux as we'll be failing before then
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	got := getPollution(rc, 0, 0)
	if got != "?" {
		t.Errorf("expected ?, got %q", got)
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
