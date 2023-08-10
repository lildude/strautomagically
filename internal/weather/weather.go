// Package weather implements methods to gather weather and AQI from OpenWeatherMap and present it in a pretty string.
package weather

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	goaqi "github.com/lildude/go-aqi"
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
		Components components `json:"components"`
	} `json:"list"`
}

type components struct {
	CO   float64 `json:"co"`
	NO   float64 `json:"no"`
	NO2  float64 `json:"no2"`
	O3   float64 `json:"o3"`
	SO2  float64 `json:"so2"`
	PM25 float64 `json:"pm2_5"`
	PM10 float64 `json:"pm10"`
	NH3  float64 `json:"nh3"`
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
func GetWeatherLine(c *client.Client, startDate time.Time, elapsed int32, lat, lon float64) (*WeatherInfo, error) {
	sts := startDate.Unix()
	endDate := startDate.Add(time.Duration(elapsed) * time.Second)
	ets := endDate.Unix()

	// Get weather at start of activity
	sw, err := getWeather(c, sts, lat, lon)
	if err != nil {
		return nil, err
	}

	// Get weather at end of activity
	// Only get this if we cross the hour as it'll be the same as the start
	ew := data{}
	if startDate.Hour() == endDate.Hour() {
		ew = sw
	} else {
		ew, err = getWeather(c, ets, lat, lon)
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
	aqi := getPollution(c, sts, ets, lat, lon)

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
func getWeather(c *client.Client, dt int64, lat, lon float64) (data, error) {
	params := queryParams(lat, lon)
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

// getPollution returns the AQI icon for the midpoint of the given period.
func getPollution(c *client.Client, startDate, endDate int64, lat, lon float64) string {
	aqi := "?"
	params := queryParams(lat, lon)
	c.BaseURL.Path = "/data/2.5/air_pollution"
	endDateTime := time.Unix(endDate, 0)

	// Get historical AQI if the end time is before the last hour point before now
	if endDateTime.Before(time.Now().Add(-1 * time.Hour)) {
		c.BaseURL.Path += "/history"
		midPoint := (startDate + endDate) / 2
		// Start and end need to be at least 1 hour apart
		params.Set("start", fmt.Sprintf("%d", midPoint-1800))
		params.Set("end", fmt.Sprintf("%d", midPoint+1800))
	}
	c.BaseURL.RawQuery = params.Encode()
	req, err := c.NewRequest(context.Background(), "GET", "", nil)
	if err != nil {
		return aqi
	}

	p := pollution{}
	r, err := c.Do(req, &p)
	if err != nil {
		log.Println("[ERROR] Failed to get pollution: ", err)
		return aqi
	}
	defer r.Body.Close()

	// OpenWeatherMap uses a non-standard AQI scale so we need to convert it.
	// Converting to the scale from https://aqicn.org/scale/.
	results, err := goaqi.Calculate(
		goaqi.PM25{Concentration: p.List[0].Components.PM25},
		goaqi.CO{Concentration: p.List[0].Components.CO},
		goaqi.NO2{Concentration: p.List[0].Components.NO2},
	)
	if err != nil {
		fmt.Println(err)
		return aqi
	}

	aqiIcon := map[string]string{
		"Good":          "💚",  //  Good
		"Moderate":      "💛",  // Moderate
		"Sensitive":     "🧡",  // Unhealthy for sensitive groups
		"Unhealthy":     "❤️", // Unhealthy
		"VeryUnhealthy": "💜",  // Very Unhealthy
		"Hazardous":     "🤎",  // Hazardous
		"VeryHazardous": "🖤",  // Very Hazardous
	}

	if len(p.List) > 0 {
		aqi = aqiIcon[results.Index.Key]
	}

	return aqi
}

// queryParams returns a url.Values object with the parameters used for all queries.
func queryParams(lat, lon float64) url.Values {
	params := url.Values{}
	params.Add("lat", os.Getenv("OWM_LAT"))
	params.Add("lon", os.Getenv("OWM_LON"))
	params.Add("lang", "en")
	params.Add("units", "metric")
	params.Add("appid", os.Getenv("OWM_API_KEY"))

	if lat != 0 && lon != 0 {
		params.Set("lat", fmt.Sprintf("%f", lat))
		params.Set("lon", fmt.Sprintf("%f", lon))
	}
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
