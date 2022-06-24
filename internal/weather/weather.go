package weather

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	owm "github.com/lildude/openweathermap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var apiKey = os.Getenv("OWM_API_KEY")

// GetWeather returns the weather conditions in a pretty string
func GetWeather(start_date time.Time, elapsed int32) string {
	lon, _ := strconv.ParseFloat(os.Getenv("OWM_LON"), 64)
	lat, _ := strconv.ParseFloat(os.Getenv("OWM_LAT"), 64)
	coord := &owm.Coordinates{
		Longitude: lon,
		Latitude:  lat,
	}

	// st, _ := time.Parse("2006-01-02T15:04:05Z", start_date)
	sts := start_date.Unix()
	ets := sts + int64(elapsed)

	sw, err := owm.NewOneCall("C", "EN", apiKey, []string{})
	if err != nil {
		log.Println(err)
	}

	err = sw.OneCallTimeMachine(coord, sts)
	if err != nil {
		log.Println(err)
	}

	ew, err := owm.NewOneCall("C", "EN", apiKey, []string{})
	if err != nil {
		log.Println(err)
	}

	err = ew.OneCallTimeMachine(coord, ets)
	if err != nil {
		log.Println(err)
	}

	p, err := owm.NewPollution(apiKey)
	if err != nil {
		log.Println(err)
	}

	params := &owm.PollutionParameters{
		Location: *coord,
		Path:     "/history",
		Start:    sts,
		End:      ets,
	}

	if err := p.PollutionByParams(params); err != nil {
		log.Println(err)
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

	aqiIcon := map[int]string{
		1: "💚", // Good
		2: "💛", // Fair
		3: "🧡", // Moderate
		4: "🤎", // Poor
		5: "🖤", // Very Poor
	}

	icon := strings.Trim(sw.Data[0].Weather[0].Icon, "dn")

	// :start.weatherIcon :start.summary | 🌡 :start.temperature–:end.temperature°C | 👌 :activityFeel°C | 💦 :start.humidity–:end.humidity% | 💨 :start.windSpeed–:end.windSpeedkm/h :start.windDirection | AQI :airquality.icon
	//⛅ Partly Cloudy | 🌡 18–19°C | 👌 19°C | 💦 58–55% | 💨 16–15km/h ↙ | AQI 49 💚

	weather := fmt.Sprintf("%s %s | 🌡 %d-%d°C | 👌 %d°C | 💦 %d-%d%% | 💨 %d-%dkm/h %s | AQI %s\n",
		weatherIcon[icon], cases.Title(language.BritishEnglish).String(sw.Data[0].Weather[0].Description),
		int(math.Round(sw.Data[0].Temp)), int(math.Round(ew.Data[0].Temp)),
		int(math.Round(sw.Data[0].FeelsLike)),
		sw.Data[0].Humidity, ew.Data[0].Humidity,
		int(math.Round(sw.Data[0].WindSpeed)*3.6), int(math.Round(ew.Data[0].WindSpeed)*3.6),
		windDirectionIcon(sw.Data[0].WindDeg),
		aqiIcon[p.List[0].Main.AQI])

	return weather
}

// Return an icon indicating the wind direction.
// Remember, this is the direction the wind is blowing from
// so we point the arrow in the direction it is going.
func windDirectionIcon(deg int) string {
	switch {
	case (deg >= 338 && deg <= 22):
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
