// Package update implements the update handler for Strava activities.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
		slog.Error("unable to unmarshal webhook payload", "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We only react to new activities for now
	if webhook.AspectType != "create" {
		w.WriteHeader(http.StatusOK)
		slog.Info("ignoring non-create webhook")
		return
	}

	db, err := database.InitDB()
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var athlete model.Athlete
	if err := db.First(&athlete, "strava_athlete_id = ?", webhook.OwnerID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("unable to query athlete", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if athlete.LastActivityID == webhook.ObjectID && os.Getenv("DEBUG") != "1" {
		w.WriteHeader(http.StatusOK)
		slog.Info("ignoring repeat event")
		return
	}

	// Create the OAuth http.Client
	authToken := &oauth2.Token{}
	if athlete.StravaAuthToken != "" {
		if err := json.Unmarshal([]byte(athlete.StravaAuthToken), authToken); err != nil {
			slog.Error("unable to unmarshal Strava auth token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	if authToken.AccessToken == "" {
		slog.Error("no access token found")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// The Oauth2 library handles refreshing the token if it's expired.
	ts := strava.OauthConfig.TokenSource(r.Context(), authToken)
	tc := oauth2.NewClient(r.Context(), ts)
	surl, _ := url.Parse(strava.BaseURL)

	newToken, err := ts.Token()
	if err != nil {
		slog.Error("unable to refresh token", "error", err)
		return
	}
	if newToken.AccessToken != authToken.AccessToken {
		t, mErr := json.Marshal(newToken)
		if mErr != nil {
			slog.Error("unable to marshal token", "error", mErr)
		} else {
			db.Model(&athlete).Update("strava_auth_token", string(t))
			slog.Info("updated token")
		}
	}

	sc := client.NewClient(surl, tc)

	activity, err := strava.GetActivity(r.Context(), sc, webhook.ObjectID)
	if err != nil {
		slog.Error("unable to get activity", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	slog.Info("activity received", "name", activity.Name, "id", activity.ID)

	// Update the summit record for this athlete
	if err := summits.UpdateSummit(db, activity); err != nil {
		slog.Error("unable to update summit record", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := &url.URL{Scheme: "https", Host: "api.openweathermap.org", Path: "/data/3.0/onecall"}
	wclient := client.NewClient(baseURL, nil)
	trcal := calendarevent.NewCalendarService(http.DefaultClient, "https://api.trainerroad.com/v1/calendar/ics", os.Getenv("TRAINERROAD_CAL_ID"))
	update, msg := constructUpdate(r.Context(), wclient, activity, trcal, db)

	// Don't update the activity if DEBUG=1
	if os.Getenv("DEBUG") == "1" {
		slog.Debug("update", "update", update)
		slog.Debug("message", "msg", msg)
		return
	}

	if !reflect.DeepEqual(update, strava.UpdatableActivity{}) {
		var updated *strava.Activity
		updated, err = strava.UpdateActivity(r.Context(), sc, webhook.ObjectID, update)
		if err != nil {
			slog.Error("unable to update activity", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		slog.Info("activity updated", "name", updated.Name, "id", updated.ID, "msg", msg)
	}

	// Record the last activity ID we've processed for this athlete
	db.Model(&athlete).Update("last_activity_id", webhook.ObjectID)

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(`success`)); err != nil {
		slog.Error("write failed", "error", err)
	}
}

// descriptionContent is the data passed to the description template.
type descriptionContent struct {
	Description string
	Weather     *weather.WeatherInfo
	Summit      *summits.ActivitySummit
}

func constructUpdate(ctx context.Context, wclient *client.Client, activity *strava.Activity, trcal *calendarevent.CalendarService, db *gorm.DB) (ua *strava.UpdatableActivity, msg string) {
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
		// if external_id starts with trainerroad else set gear to bike and append "- Outside".
		// We assume we've already done this if the activity name starts with TR.
		if !strings.HasPrefix(activity.Name, "TR: ") {
			event, err := trcal.GetCalendarEvent(ctx, activity.StartDate)
			if err != nil {
				slog.Error("unable to get TrainerRoad calendar event", "error", err)
			}

			// We assume if there is an event for the day, the activity is the same
			if event != nil && event.Summary != "" {
				slog.Info("found TrainerRoad calendar event", "summary", event.Summary)
				title = "TR: " + event.Summary
			} else {
				slog.Info("no TrainerRoad calendar event found")
			}
		}

		if strings.HasPrefix(activity.ExternalID, "trainerroad") {
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

	wi, _ := weather.GetWeatherLine(ctx, wclient, activity.StartDateLocal, activity.ElapsedTime, lat, lon)
	if wi == nil {
		return &update, msg
	}
	if painCave {
		// Put lat and lon back to 0 for easier templating
		wi.Start.Lat, wi.Start.Lon, wi.End.Lat, wi.End.Lon = 0, 0, 0, 0
	}

	summit, err := summits.GetSummitForActivity(db, activity)
	if err != nil {
		slog.Error("unable to get summit", "error", err)
	}

	content := descriptionContent{
		Description: activity.Description,
		Weather:     wi,
		Summit:      summit,
	}

	desc, err := execTemplate("description.tmpl", content)
	if err != nil {
		slog.Error("unable to parse description template", "error", err)
	}
	update.Description = desc

	return &update, msg
}

func execTemplate(tmpl string, data any) (string, error) {
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
