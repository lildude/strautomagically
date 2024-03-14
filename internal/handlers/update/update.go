// Package update implements the update handler for Strava activities.
package update

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/calendarevent"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/lildude/strautomagically/internal/weather"
	"golang.org/x/oauth2"
)

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	var webhook strava.WebhookPayload
	if r.Body == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &webhook); err != nil {
		log.Println("[ERROR] unable to unmarshal webhook payload:", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We only react to new activities for now
	if webhook.AspectType != "create" {
		w.WriteHeader(http.StatusOK)
		log.Println("[INFO] ignoring non-create webhook")
		return
	}

	rcache, err := cache.NewRedisCache(os.Getenv("REDIS_URL")) //nolint:contextcheck // TODO: pass context rather then generate in the package.
	if err != nil {
		log.Println("[ERROR] unable to create redis cache:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// See if we've seen this activity before
	aid, err := rcache.Get("strava_activity")
	if err != nil {
		log.Println("[ERROR] unable to get activity id from cache:", err)
	}
	// Convert aid to int
	s, _ := aid.(string)
	aidInt, _ := strconv.ParseInt(s, 10, 64)

	if os.Getenv("ENV") != "dev" && aidInt == webhook.ObjectID {
		w.WriteHeader(http.StatusOK)
		log.Println("[INFO] ignoring repeat event")
		return
	}

	// Create the OAuth http.Client
	// ctx := context.Background()
	authToken := &oauth2.Token{}
	err = rcache.GetJSON("strava_auth_token", &authToken)
	if err != nil {
		log.Println("[ERROR] unable to get token:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// The Oauth2 library handles refreshing the token if it's expired.
	ts := strava.OauthConfig.TokenSource(r.Context(), authToken)
	tc := oauth2.NewClient(r.Context(), ts)
	surl, _ := url.Parse(strava.BaseURL)

	newToken, err := ts.Token()
	if err != nil {
		log.Println("[ERROR] unable to refresh token:", err)
		return
	}
	if newToken.AccessToken != authToken.AccessToken {
		err = rcache.SetJSON("strava_auth_token", newToken)
		if err != nil {
			log.Println("[ERROR] unable to store token:", err)
			return
		}
		log.Println("[INFO] updated token")
	}

	sc := client.NewClient(surl, tc)

	activity, err := strava.GetActivity(sc, webhook.ObjectID) //nolint:contextcheck // TODO: pass context rather then generate in the package.
	if err != nil {
		log.Println("[ERROR] unable to get activity:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Activity:%s (%d)", activity.Name, activity.ID)

	baseURL := &url.URL{Scheme: "https", Host: "api.openweathermap.org", Path: "/data/3.0/onecall"}
	wclient := client.NewClient(baseURL, nil)
	trcal := calendarevent.NewCalendarService(http.DefaultClient, "https://api.trainerroad.com/v1/calendar/ics", os.Getenv("TRAINERROAD_CAL_ID"))
	update, msg := constructUpdate(wclient, activity, trcal) //nolint:contextcheck // TODO: pass context rather then generate in the package.

	// Don't update the activity if DEBUG=1
	if os.Getenv("DEBUG") == "1" {
		log.Printf("[DEBUG] update: %+v\n", update)
		log.Println("[DEBUG] message:", msg)
		return
	}

	if !reflect.DeepEqual(update, strava.UpdatableActivity{}) {
		var updated *strava.Activity
		updated, err = strava.UpdateActivity(sc, webhook.ObjectID, update) //nolint:contextcheck // TODO: pass context rather then generate in the package.
		if err != nil {
			log.Println("[ERROR] unable to update activity:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.Printf("[INFO] Activity:%s (%d): %s", updated.Name, updated.ID, msg)

		// Cache activity ID if we've succeeded
		err = rcache.Set("strava_activity", webhook.ObjectID)
		if err != nil {
			log.Println("[ERROR] unable to cache activity id:", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(`success`)); err != nil {
		log.Println("[ERROR]", err)
	}
}

func constructUpdate(wclient *client.Client, activity *strava.Activity, trcal *calendarevent.CalendarService) (ua *strava.UpdatableActivity, msg string) {
	var update strava.UpdatableActivity
	var title string
	msg = "no activity changes"
	const trainer = "b9880609" // Tacx Neo 2T Turbo
	const bike = "b10013574"   // Dolan Tuono Disc
	const shoes = "g10043849"  // No name, Not running shoes

	// TODO: Move these to somewhere more configurable
	switch activity.Type {
	// I'll never handcycle. This is used for testing only
	case "Handcycle":
		return &update, msg

	case "Ride":
		title = activity.Name
		// Get the name from TrainerRoad calendar, prepend it with TR and set gear to trainer
		// if external_id starts with trainerroad else set gear to bike and append "- Outside"

		// We assume we've already done this if the activity name starts with TR
		if !strings.HasPrefix(activity.Name, "TR: ") {
			event, err := trcal.GetCalendarEvent(activity.StartDate)
			if err != nil {
				log.Println("[ERROR] unable to get TrainerRoad calendar event:", err)
			}

			// We assume if there is an event for the day, the activity is the same
			if event != nil && event.Summary != "" {
				log.Println("[INFO] found TrainerRoad calendar event:", event.Summary)
				title = "TR: " + event.Summary
			} else {
				log.Println("[INFO] no TrainerRoad calendar event found")
			}
		}

		if activity.ExternalID != "" && activity.ExternalID[0:11] == "trainerroad" {
			update.GearID = trainer
			update.Trainer = true
		} else {
			update.GearID = bike
			if strings.HasPrefix(title, "TR: ") {
				title += " - Outside"
			}
		}

		if title != activity.Name {
			update.Name = title
		}

		msg = "prefixed name of ride with TR and set gear"

	case "Rowing":
		// Workouts created in ErgZone will have the name in the first line of the description
		lines := strings.Split(activity.Description, "\n")
		if len(lines) > 0 {
			// We only want the first line if the description contains the app.erg.zone URL
			if strings.Contains(activity.Description, "app.erg.zone") {
				title = lines[0]
				update.Description = "\n"
			}
		}

		// Fallback to the name if there is no description or it doesn't contain "app.erg.zone"
		if title == "" {
			switch activity.Name {
			case "v250m/1:30r...7 row", "v5:00/1:00r...15 row":
				title = "Speed Pyramid Row w/ 1.5' Active RI per 250m work"
			case "8x500m/3:30r row", "v5:00/1:00r...17 row":
				title = "8x 500m w/ 3.5' Active RI Row"
			case "5x1500m/5:00r row":
				title = "5x 1500m w/ 5' RI Row"
			case "4x2000m/5:00r row", "v5:00/1:00r...9 row":
				title = "4x 2000m w/5' Active RI Row"
			case "4x1000m/5:00r row":
				title = "4x 1000m /5' RI Row"
			case "v3000m/5:00r...3 row", "v5:00/1:00r...7 row":
				title = "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row"
			case "5:00 row":
				title = "Warm-up Row"
				update.HideFromHome = true
			}
		}
		update.Name = title
		if title != "" {
			msg = "set title to " + title
		}

	// Nothing to change here yet
	// case "Run":
	// 	return &update, msg

	case "VirtualRide":
		// Set gear to trainer
		update.GearID = trainer
		update.Trainer = true
		msg = "set gear to trainer"
	case "Walk":
		// Mute walks and set shoes
		update.HideFromHome = true
		update.GearID = shoes
		msg = "muted walk"
	case "WeightTraining":
		// Set Humane Burpees Title for WeightLifting activities between 3 & 7 minutes long
		if activity.ElapsedTime >= 180 && activity.ElapsedTime <= 420 {
			update.HideFromHome = true
			update.Name = "Humane Burpees"
			msg = "set humane burpees title"
		}
	}

	// Do nothing if we've already got weather data
	if strings.Contains(activity.Description, "AQI") {
		return &update, msg
	}

	painCave, lat, lon := true, float64(0), float64(0)
	if len(activity.StartLatlng) > 0 && activity.Type != "VirtualRide" {
		painCave, lat, lon = false, activity.StartLatlng[0], activity.StartLatlng[1]
	}

	w, _ := weather.GetWeatherLine(wclient, activity.StartDateLocal, int32(activity.ElapsedTime), lat, lon)
	if painCave {
		// Put lat and lon back to 0 for easier templating
		w.Start.Lat, w.Start.Lon, w.End.Lat, w.End.Lon = 0, 0, 0, 0
	}

	if w != nil {
		wtr, err := execTemplate("weather.tmpl", w)
		if err != nil {
			log.Println("[ERROR] unable to parse weather template:", err)
		}

		if activity.Description != "" && update.Description != "\n" {
			update.Description = activity.Description + "\n\n"
		}
		update.Description += wtr
		msg += " & added weather"
	}

	return &update, msg
}

func execTemplate(tmpl string, data interface{}) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Test and dev/prod use different paths
	templatePath := filepath.Join(wd, "templates", tmpl)
	if os.Getenv("ENV") == "test" {
		templatePath = filepath.Join(wd, "..", "..", "..", "templates", tmpl)
	}

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = t.Execute(&tpl, data)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}
