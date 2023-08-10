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

	latIn := 51.509865
	lonIn := -0.118092
	latInStr := strconv.FormatFloat(latIn, 'f', -1, 64)
	lonInStr := strconv.FormatFloat(lonIn, 'f', -1, 64)
	appIDIn := "123456789"
	t.Setenv("OWM_LAT", latInStr)
	t.Setenv("OWM_LON", lonInStr)
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
		if lat != latInStr || lon != lonInStr || appid != appIDIn || units != "metric" || lang != "en" || dt != startOut {
			t.Errorf(
				"Expected lat=%s, lon=%s, appid=%s, units=metric, lang=en, dt=%s, got lat=%s, lon=%s, appid=%s, units=%s, lang=%s, dt=%s",
				latInStr, lonInStr, appIDIn, startOut, lat, lon, appid, units, lang, dt,
			)
		}

		resp, _ := os.ReadFile("testdata/weather.json")
		fmt.Fprintln(w, string(resp))
	})

	got, err := getWeather(rc, startIn, latIn, lonIn)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}
	want := data{
		Lat:       0,
		Lon:       0,
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

	got, err := getWeather(rc, 0, 0, 0)
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
		resp := `{"list":[{"main":{"aqi":1},"components":{"pm2_5": 10.0, "co": 1.92,"no2": 12.51},"dt": 1691658340}]}`
		fmt.Fprintln(w, resp)
	})

	got, err := GetWeatherLine(rc, startIn, elapsed, 0, 0)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}

	want := &WeatherInfo{
		Start: periodWeatherInfo{
			Icon:      "☀️",
			Desc:      "Clear Sky",
			Temp:      19,
			FeelsLike: 16,
			Humidity:  64,
			WindSpeed: 14,
			WindDir:   "↓",
			Lat:       0,
			Lon:       0,
		},
		End: periodWeatherInfo{
			Icon:      "☀️",
			Desc:      "Clear Sky",
			Temp:      19,
			FeelsLike: 16,
			Humidity:  64,
			WindSpeed: 14,
			WindDir:   "↓",
			Lat:       0,
			Lon:       0,
		},
		Aqi: "💚",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
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
	midPoint := startIn.Add(time.Duration(elapsed/2) * time.Second).Unix()

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
		start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
		end, _ := strconv.ParseInt(q.Get("end"), 10, 64)
		// Confirm we receive the right query params
		if start != midPoint-1800 || end != midPoint+1800 {
			t.Errorf("Expected start=%d, end=%d, got start=%d, end=%d", midPoint-1800, midPoint+1800, start, end)
		}

		resp := `{"list":[{"main":{"aqi":1},"components":{"pm2_5": 10.0, "co": 1.92,"no2": 12.51},"dt": 1691658340}]}`
		fmt.Fprintln(w, resp)
	})

	got, err := GetWeatherLine(rc, startIn, elapsed, 0, 0)
	if err != nil {
		t.Errorf("expected nil error, got %q", err)
	}

	want := &WeatherInfo{
		Start: periodWeatherInfo{
			Icon:      "☀️",
			Desc:      "Clear Sky",
			Temp:      19,
			FeelsLike: 16,
			Humidity:  64,
			WindSpeed: 14,
			WindDir:   "↓",
			Lat:       0,
			Lon:       0,
		},
		End: periodWeatherInfo{
			Icon:      "☀️",
			Desc:      "Clear Sky",
			Temp:      23,
			FeelsLike: 26,
			Humidity:  94,
			WindSpeed: 3,
			WindDir:   "↙",
			Lat:       0,
			Lon:       0,
		},
		Aqi: "💚",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
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

// TestGetPollution tests the getPollution function by mocking the response from the API
// and ensuring we get the expected emoji back.
//
// This test uses the lat value to return the pm2_5 value we use to modify the response.
func TestGetPollutionForAllLevels(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	tests := []struct {
		mockPM2_5 float64
		want      string
	}{
		{10, "💚"},
		{20, "💛"},
		{40, "🧡"},
		{80, "❤️"},
		{160, "💜"},
		{320, "🤎"},
		{400, "🖤"},
	}
	mux.HandleFunc("/data/2.5/air_pollution/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("lat")
		pm2_5, _ := strconv.ParseFloat(q, 64)
		resp := fmt.Sprintf(`{"list":[{"main":{"aqi":1},"components":{"pm2_5": %.2f, "co": 1.92,"no2": 12.51},"dt": 1691658340}]}`, pm2_5)
		fmt.Fprintln(w, resp)
	})

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.2f", tt.mockPM2_5), func(t *testing.T) {
			got := getPollution(rc, 1691648340, 1691658340, tt.mockPM2_5, -0.118092)

			if got != tt.want {
				t.Errorf("aqi %.2f expected %s, got %s", tt.mockPM2_5, tt.want, got)
			}
		})
	}
}

func TestGetPollutionWithErrorReturnsQuestionMark(t *testing.T) {
	rc, _, teardown := setup() // We're not using the mux as we'll be failing before then
	defer teardown()

	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	latIn := 51.509865
	lonIn := -0.118092

	got := getPollution(rc, 0, 0, latIn, lonIn)
	if got != "?" {
		t.Errorf("expected ?, got %q", got)
	}
}

func TestGetCurrentPollutionIfEndHourSameAsNowHour(t *testing.T) {
	rc, mux, teardown := setup()
	defer teardown()

	latIn := 51.509865
	lonIn := -0.118092

	mux.HandleFunc("/data/2.5/air_pollution", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		lat := q.Get("lat")
		lon := q.Get("lon")
		// Confirm we receive the right query params
		if lat != fmt.Sprintf("%f", latIn) || lon != fmt.Sprintf("%f", lonIn) {
			t.Errorf("Expected lat=%f, lon=%f, got lat=%s, lon=%s", latIn, lonIn, lat, lon)
		}

		resp := `{"list":[{"main":{"aqi":1},"components":{"pm2_5": 10.0, "co": 1.92,"no2": 12.51},"dt": 1691658340}]}`
		fmt.Fprintln(w, resp)
	})

	got := getPollution(rc, time.Now().Unix(), time.Now().Unix(), latIn, lonIn)
	if got != "💚" {
		t.Errorf("expected 💚, got %q", got)
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
