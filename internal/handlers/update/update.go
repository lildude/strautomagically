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
	"strings"
	"text/template"

	"github.com/lildude/strautomagically/internal/calendarevent"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/lildude/strautomagically/internal/summits"
	"github.com/lildude/strautomagically/internal/weather"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
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

	db, err := database.InitDB()
	if err != nil {
		log.Println("[ERROR] unable to connect to database:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var athlete model.Athlete
	db.First(&athlete, "strava_athlete_id = ?", webhook.OwnerID)

	if athlete.LastActivityID == webhook.ObjectID && os.Getenv("DEBUG") != "1" {
		w.WriteHeader(http.StatusOK)
		log.Println("[INFO] ignoring repeat event")
		return
	}

	// Create the OAuth http.Client
	authToken := &oauth2.Token{}
	if err := athlete.StravaAuthToken.AssignTo(authToken); err != nil {
		log.Println("[ERROR] unable to assign Strava auth token:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if authToken.AccessToken == "" {
		log.Println("[ERROR] no access token found")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authToken = &oauth2.Token{AccessToken: authToken.AccessToken}
	ts := strava.OauthConfig.TokenSource(r.Context(), authToken)
	tc := oauth2.NewClient(r.Context(), ts)

	newToken, err := ts.Token()
	if err != nil {
		log.Println("[ERROR] unable to refresh token:", err)
		return
	}

	if newToken.AccessToken != authToken.AccessToken {
		db.Model(&athlete).Update("strava_auth_token", newToken.AccessToken)
		log.Println("[INFO] updated token")
	}

	surl, _ := url.Parse(strava.BaseURL)
	sc := client.NewClient(surl, tc)

	activity, err := strava.GetActivity(sc, webhook.ObjectID) //nolint:contextcheck // TODO: pass context rather then generate in the package.
	if err != nil {
		log.Println("[ERROR] unable to get activity:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Activity:%s (%d)", activity.Name, activity.ID)

	// Update the summit record
	err = summits.UpdateSummit(db, activity)
	if err != nil {
		log.Println("[ERROR] unable to update summit record:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] updated summit record for athlete %d", athlete.StravaAthleteID)

	baseURL := &url.URL{Scheme: "https", Host: "api.openweathermap.org", Path: "/data/3.0/onecall"}
	wclient := client.NewClient(baseURL, nil)
	trcal := calendarevent.NewCalendarService(http.DefaultClient, "https://api.trainerroad.com/v1/calendar/ics", os.Getenv("TRAINERROAD_CAL_ID"))
	update, msg := constructUpdate(wclient, activity, trcal, db) //nolint:contextcheck // TODO: pass context rather then generate in the package.

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
	}

	// Update the athlete's last activity ID
	db.Model(&athlete).Updates(map[string]interface{}{
		"last_activity_id": webhook.ObjectID,
	})

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(``)); err != nil {
		log.Println("[ERROR]", err)
	}
}

type descriptionContent struct {
	Description string
	Weather     *weather.WeatherInfo
	Summit      *summits.ActivitySummit
}

func constructUpdate(wclient *client.Client, activity *strava.Activity, trcal *calendarevent.CalendarService, db *gorm.DB) (ua *strava.UpdatableActivity, msg string) {
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
				activity.Description = ""
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

	w, _ := weather.GetWeatherLine(wclient, activity.StartDateLocal, int32(activity.ElapsedTime), lat, lon) //nolint:gosec // disable G115
	if painCave {
		// Put lat and lon back to 0 for easier templating
		w.Start.Lat, w.Start.Lon, w.End.Lat, w.End.Lon = 0, 0, 0, 0
	}

	summit, err := summits.GetSummitForActivity(db, activity)
	if err != nil {
		log.Println("[ERROR] unable to get summit:", err)
	}

	descriptionContent := descriptionContent{
		Description: activity.Description,
		Weather:     w,
		Summit:      summit,
	}

	update.Description, err = execTemplate("description.tmpl", descriptionContent)
	if err != nil {
		log.Println("[ERROR] unable to parse description template:", err)
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
