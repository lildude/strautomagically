// Package weather implements methods to gather weather and AQI from OpenWeatherMap and present it in a pretty string.
package weather

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lildude/strautomagically/internal/client"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// weatherData struct holds just the data we need from the OpenWeatherMap API.
type weatherData struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Data []data  `json:"data"`
}

type data struct {
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Temp      float64   `json:"temp"`
	FeelsLike float64   `json:"feels_like"`
	Humidity  int64     `json:"humidity"`
	WindSpeed float64   `json:"wind_speed"`
	WindDeg   int       `json:"wind_deg"`
	Weather   []weather `json:"weather"`
}

type weather struct {
	Main        string `json:"main"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

// pollution struct holds just the data we need from the OpenWeatherMap API.
type pollution struct {
	List []struct {
		Main struct {
			AQI int `json:"AQI"`
		} `json:"main"`
	} `json:"list"`
}

type QueryParams struct {
	Lat   float64
	Lon   float64
	Lang  string
	Units string
	AppID string
}

type periodWeatherInfo struct {
	Icon      string
	Desc      string
	Temp      int
	FeelsLike int
	Humidity  int64
	WindSpeed int
	WindDir   string
	Lat       float64
	Lon       float64
}

type WeatherInfo struct {
	Start periodWeatherInfo
	End   periodWeatherInfo
	Aqi   string
}

// GetWeatherLine returns the weather conditions in a struct for passing to the templating.
func GetWeatherLine(c *client.Client, startDate time.Time, elapsed int32) (*WeatherInfo, error) {
	sts := startDate.Unix()
	endDate := startDate.Add(time.Duration(elapsed) * time.Second)
	ets := endDate.Unix()

	// Get weather at start of activity
	sw, err := getWeather(c, sts)
	if err != nil {
		return nil, err
	}

	// Get weather at end of activity
	// Only get this if we cross the hour as it'll be the same as the start
	ew := data{}
	if startDate.Hour() == endDate.Hour() {
		ew = sw
	} else {
		ew, err = getWeather(c, ets)
		if err != nil {
			// If we can't get the end weather, just use the start weather
			ew = sw
		}
	}

	// Return early if we don't have any data
	if sw.Weather[0].Description == "" || ew.Weather[0].Description == "" {
		return nil, err
	}

	weatherIcon := map[string]string{
		"01": "\u2600\uFE0F", // Clear
		"02": "🌤",            // Partly cloudy
		"03": "⛅",            // Scattered clouds
		"04": "🌥",            // Broken clouds
		"09": "🌧",            // Shower/rain
		"10": "🌦",            // Rain
		"11": "⛈",            // Thunderstorm
		"13": "🌨",            // Snow
		"50": "🌫",            // Mist
	}

	// get aqi icon
	aqi := getPollution(c, sts, ets)

	icon := strings.Trim(sw.Weather[0].Icon, "dn")

	// mps -> kph
	speedFactor := 3.6

	sp := periodWeatherInfo{
		Icon:      weatherIcon[icon],
		Desc:      cases.Title(language.BritishEnglish).String(sw.Weather[0].Description),
		Temp:      int(math.Round(sw.Temp)),
		FeelsLike: int(math.Round(sw.FeelsLike)),
		Humidity:  sw.Humidity,
		WindSpeed: int(math.Round(sw.WindSpeed) * speedFactor),
		WindDir:   windDirectionIcon(sw.WindDeg),
		Lat:       sw.Lat,
		Lon:       sw.Lon,
	}

	ep := periodWeatherInfo{
		Icon:      weatherIcon[icon],
		Desc:      cases.Title(language.BritishEnglish).String(ew.Weather[0].Description),
		Temp:      int(math.Round(ew.Temp)),
		FeelsLike: int(math.Round(ew.FeelsLike)),
		Humidity:  ew.Humidity,
		WindSpeed: int(math.Round(ew.WindSpeed) * speedFactor),
		WindDir:   windDirectionIcon(ew.WindDeg),
		Lat:       ew.Lat,
		Lon:       ew.Lon,
	}

	wi := WeatherInfo{
		Start: sp,
		End:   ep,
		Aqi:   aqi,
	}

	return &wi, nil
}

// getWeather returns the weather conditions for the given time.
func getWeather(c *client.Client, dt int64) (data, error) {
	params := defaultParams()
	params.Add("dt", fmt.Sprintf("%d", dt))
	c.BaseURL.Path = "/data/3.0/onecall/timemachine"
	c.BaseURL.RawQuery = params.Encode()
	req, err := c.NewRequest(context.Background(), "GET", "", nil)
	if err != nil {
		return data{}, err
	}

	// Get weather at start of activity
	w := weatherData{}
	r, err := c.Do(req, &w)
	if err != nil {
		return data{}, err
	}
	defer r.Body.Close()
	data := w.Data[0]
	data.Lat = w.Lat
	data.Lon = w.Lon

	return data, nil
}

// getPollution returns the AQI icon for the given period.
func getPollution(c *client.Client, startDate, endDate int64) string {
	aqi := "?"
	params := defaultParams()
	params.Set("start", fmt.Sprintf("%d", startDate))
	params.Set("end", fmt.Sprintf("%d", endDate))
	c.BaseURL.Path = "/data/2.5/air_pollution/history"
	c.BaseURL.RawQuery = params.Encode()
	req, err := c.NewRequest(context.Background(), "GET", "", nil)
	if err != nil {
		return aqi
	}

	p := pollution{}
	r, err := c.Do(req, &p)
	if err != nil {
		return aqi
	}
	defer r.Body.Close()

	aqiIcon := map[int]string{
		1: `💚`, // Good
		2: `💛`, // Fair
		3: `🧡`, // Moderate
		4: `🤎`, // Poor
		5: `🖤`, // Very Poor
	}

	if len(p.List) > 0 {
		aqi = aqiIcon[p.List[0].Main.AQI]
	}

	return aqi
}

// defaultParams returns a url.Values object with the default parameters used for all queries.
func defaultParams() url.Values {
	params := url.Values{}
	params.Add("lat", os.Getenv("OWM_LAT"))
	params.Add("lon", os.Getenv("OWM_LON"))
	params.Add("lang", "en")
	params.Add("units", "metric")
	params.Add("appid", os.Getenv("OWM_API_KEY"))
	return params
}

// Return an icon indicating the wind direction.
// Remember, this is the direction the wind is blowing from
// so we point the arrow in the direction it is going.
func windDirectionIcon(deg int) string {
	switch {
	case (deg >= 338 && deg <= 360) || (deg >= 0 && deg <= 22):
		return "↓"
	case (deg >= 23 && deg <= 67):
		return "↙"
	case (deg >= 68 && deg <= 112):
		return "←"
	case (deg >= 113 && deg <= 157):
		return "↖"
	case (deg >= 158 && deg <= 202):
		return "↑"
	case (deg >= 203 && deg <= 247):
		return "↗"
	case (deg >= 248 && deg <= 292):
		return "→"
	case (deg >= 293 && deg <= 337):
		return "↘"
	}
	return ""
}
