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

	"github.com/lildude/strautomagically/internal/client"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// weather struct holds just the data we need from the OpenWeatherMap API
type weather struct {
	Data []struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int64   `json:"humidity"`
		WindSpeed float64 `json:"wind_speed"`
		WindDeg   int     `json:"wind_deg"`
		Weather   []struct {
			Main        string `json:"main"`
			Icon        string `json:"icon"`
			Description string `json:"description"`
		} `json:"weather"`
	} `json:"data"`
}

// pollution struct holds just the data we need from the OpenWeatherMap API
type pollution struct {
	List []struct {
		Main struct {
			AQI int `json:"AQI"`
		} `json:"main"`
	} `json:"list"`
}

// GetWeather returns the weather conditions in a pretty string
func GetWeather(c *client.Client, start_date time.Time, elapsed int32) string {
	sts := start_date.Unix()
	end_date := start_date.Add(time.Duration(elapsed) * time.Second)
	ets := end_date.Unix()

	params := defaultParams()
	params.Add("dt", fmt.Sprintf("%d", sts))
	c.BaseURL.Path = "/data/3.0/onecall/timemachine"
	c.BaseURL.RawQuery = params.Encode()
	req, err := c.NewRequest("GET", "", nil)
	if err != nil {
		log.Println(err)
		return ""
	}

	// Get weather at start of activity
	sw := weather{}
	_, err = c.Do(context.Background(), req, &sw)
	if err != nil {
		log.Println(err)
		return ""
	}

	// Get weather at end of activity
	// Only get this if we cross the hour as it'll be the same as the start
	ew := weather{}
	if start_date.Hour() == end_date.Hour() {
		ew = sw
	} else {
		params.Set("dt", fmt.Sprintf("%d", ets))
		c.BaseURL.RawQuery = params.Encode()
		req, err = c.NewRequest("GET", "", nil)
		if err != nil {
			log.Println(err)
			return ""
		}

		_, err = c.Do(context.Background(), req, &ew)
		if err != nil {
			log.Println(err)
			return ""
		}
	}

	// Return early if we don't have any data
	if len(sw.Data) == 0 || len(ew.Data) == 0 {
		return ""
	}

	weatherIcon := map[string]string{
		"01": "☀️", // Clear
		"02": "🌤",  // Partly cloudy
		"03": "⛅",  // Scattered clouds
		"04": "🌥",  // Broken clouds
		"09": "🌧",  // Shower/rain
		"10": "🌦",  // Rain
		"11": "⛈",  // Thunderstorm
		"13": "🌨",  // Snow
		"50": "🌫",  // Mist
	}

	// get aqi icon
	aqi := getPollution(c, sts, ets)

	swd := sw.Data[0]
	ewd := ew.Data[0]

	icon := strings.Trim(sw.Data[0].Weather[0].Icon, "dn")

	// TODO: make me templatable
	// :start.weatherIcon :start.summary | 🌡 :start.temperature–:end.temperature°C | 👌 :activityFeel°C | 💦 :start.humidity–:end.humidity% | 💨 :start.windSpeed–:end.windSpeedkm/h :start.windDirection | AQI :airquality.icon
	//⛅ Partly Cloudy | 🌡 18–19°C | 👌 19°C | 💦 58–55% | 💨 16–15km/h ↙ | AQI 💚

	weather := fmt.Sprintf("%s %s | 🌡 %d-%d°C | 👌 %d°C | 💦 %d-%d%% | 💨 %d-%dkm/h %s | AQI %s\n",
		weatherIcon[icon], cases.Title(language.BritishEnglish).String(swd.Weather[0].Description),
		int(math.Round(swd.Temp)), int(math.Round(ewd.Temp)),
		int(math.Round(swd.FeelsLike)),
		swd.Humidity, ewd.Humidity,
		int(math.Round(swd.WindSpeed)*3.6), int(math.Round(ewd.WindSpeed)*3.6),
		windDirectionIcon(swd.WindDeg),
		aqi)

	return weather
}

// getPollution returns the AQI icon for the given period
func getPollution(c *client.Client, start_date, end_date int64) string {
	params := defaultParams()
	params.Set("start", fmt.Sprintf("%d", start_date))
	params.Set("end", fmt.Sprintf("%d", end_date))
	c.BaseURL.Path = "/data/2.5/air_pollution/history"
	c.BaseURL.RawQuery = params.Encode()
	req, err := c.NewRequest("GET", "", nil)
	if err != nil {
		log.Println(err)
		return ""
	}

	p := pollution{}
	_, err = c.Do(context.Background(), req, &p)
	if err != nil {
		log.Println(err)
		return ""
	}

	aqiIcon := map[int]string{
		1: `💚`, // Good
		2: `💛`, // Fair
		3: `🧡`, // Moderate
		4: `🤎`, // Poor
		5: `🖤`, // Very Poor
	}

	aqi := "?"
	if len(p.List) > 0 {
		aqi = aqiIcon[p.List[0].Main.AQI]
	}

	return aqi
}

// defaultParams returns a url.Values object with the default parameters used for all queries
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
		return "↙️"
	case (deg >= 68 && deg <= 112):
		return "←"
	case (deg >= 113 && deg <= 157):
		return "↖️"
	case (deg >= 158 && deg <= 202):
		return "↑"
	case (deg >= 203 && deg <= 247):
		return "↗️"
	case (deg >= 248 && deg <= 292):
		return "→"
	case (deg >= 293 && deg <= 337):
		return "↘️"
	}
	return ""
}
