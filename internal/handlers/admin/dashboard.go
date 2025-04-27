package admin

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgtype"
	"github.com/lildude/strautomagically/internal/database"
	"gorm.io/gorm"
)

// Define custom template functions
var funcMap = template.FuncMap{
	"toString": func(b []byte) string {
		return string(b)
	},
}

// Parse templates with custom functions - make this the single source
var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/admin/*.html"))

// ShowDashboard displays the main admin dashboard with lists of athletes and summits.
func ShowDashboard(gormDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		athletes, err := database.GetAllAthletes(gormDB)
		if err != nil {
			log.Printf("Error getting athletes: %v", err)
			http.Error(w, "Failed to load athletes", http.StatusInternalServerError)
			return
		}

		summits, err := database.GetAllSummits(gormDB)
		if err != nil {
			log.Printf("Error getting summits: %v", err)
			http.Error(w, "Failed to load summits", http.StatusInternalServerError)
			return
		}

		// Get query parameters for success/error messages
		successMsg := r.URL.Query().Get("success")
		errorMsg := r.URL.Query().Get("error")

		data := map[string]interface{}{
			"Athletes": athletes,
			"Summits":  summits,
			"Success":  successMsg,
			"Error":    errorMsg,
		}

		// Use the base name of the template file for execution
		err = templates.ExecuteTemplate(w, "dashboard.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// HandleAthleteUpdate handles updating an existing athlete.
func HandleAthleteUpdate(gormDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin?error=Failed+to+parse+form", http.StatusSeeOther)
			return
		}

		idStr := r.FormValue("id")
		name := r.FormValue("name")
		lastActivityIDStr := r.FormValue("last_activity_id")
		authTokenJSON := r.FormValue("strava_auth_token") // Get the JSON string

		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Redirect(w, r, "/admin?error=Invalid+athlete+ID", http.StatusSeeOther)
			return
		}

		lastActivityID, err := strconv.ParseInt(lastActivityIDStr, 10, 64)
		if err != nil {
			http.Redirect(w, r, "/admin?error=Invalid+Last+Activity+ID+format", http.StatusSeeOther)
			return
		}

		if name == "" {
			http.Redirect(w, r, "/admin?error=Athlete+name+cannot+be+empty", http.StatusSeeOther)
			return
		}

		// Validate and prepare the StravaAuthToken JSON
		var tokenJSON pgtype.JSONB
		if authTokenJSON != "" {
			// Basic validation: Check if it's valid JSON
			var js json.RawMessage
			if err := json.Unmarshal([]byte(authTokenJSON), &js); err != nil {
				http.Redirect(w, r, "/admin?error=Invalid+Strava+Auth+Token+JSON+format", http.StatusSeeOther)
				return
			}
			err = tokenJSON.Set([]byte(authTokenJSON))
			if err != nil {
				log.Printf("Error setting StravaAuthToken JSONB for athlete %d: %v", id, err)
				http.Redirect(w, r, "/admin?error=Failed+to+process+Strava+Auth+Token", http.StatusSeeOther)
				return
			}
		} else {
			// Handle empty input - set to empty JSON object '{}' or null?
			// Setting to '{}' to match default
			err = tokenJSON.Set([]byte("{}"))
			if err != nil {
				// This should ideally not fail for "{}"
				log.Printf("Error setting empty StravaAuthToken JSONB for athlete %d: %v", id, err)
				http.Redirect(w, r, "/admin?error=Failed+to+process+empty+Strava+Auth+Token", http.StatusSeeOther)
				return
			}
		}

		// Fetch the existing athlete to update
		athlete, err := database.GetAthleteByID(gormDB, uint(id))
		if err != nil || athlete == nil {
			log.Printf("Error finding athlete %d for update: %v", id, err)
			http.Redirect(w, r, "/admin?error=Athlete+not+found+for+update", http.StatusSeeOther)
			return
		}

		// Update fields
		athlete.StravaAthleteName = name
		athlete.LastActivityID = lastActivityID
		athlete.StravaAuthToken = tokenJSON // Update the token

		err = database.UpdateAthlete(gormDB, athlete)
		if err != nil {
			log.Printf("Error updating athlete %d: %v", id, err)
			http.Redirect(w, r, "/admin?error=Failed+to+update+athlete", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/admin?success=Athlete+updated+successfully", http.StatusSeeOther)
	}
}

// HandleSummitUpdate handles updating an existing summit record.
func HandleSummitUpdate(gormDB *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin?error=Failed+to+parse+form", http.StatusSeeOther)
			return
		}

		idStr := r.FormValue("id")
		runStr := r.FormValue("run")
		rideStr := r.FormValue("ride")
		// Year and AthleteID are generally not editable here, they define the record context.

		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Redirect(w, r, "/admin?error=Invalid+summit+ID", http.StatusSeeOther)
			return
		}

		run, err := strconv.ParseFloat(runStr, 64)
		if err != nil {
			http.Redirect(w, r, "/admin?error=Invalid+Run+value+format", http.StatusSeeOther)
			return
		}

		ride, err := strconv.ParseFloat(rideStr, 64)
		if err != nil {
			http.Redirect(w, r, "/admin?error=Invalid+Ride+value+format", http.StatusSeeOther)
			return
		}

		// Fetch the existing summit to update
		summit, err := database.GetSummitByID(gormDB, uint(id))
		if err != nil || summit == nil {
			log.Printf("Error finding summit %d for update: %v", id, err)
			http.Redirect(w, r, "/admin?error=Summit+record+not+found+for+update", http.StatusSeeOther)
			return
		}

		// Update fields
		summit.Run = run
		summit.Ride = ride

		err = database.UpdateSummit(gormDB, summit)
		if err != nil {
			log.Printf("Error updating summit %d: %v", id, err)
			http.Redirect(w, r, "/admin?error=Failed+to+update+summit+record", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/admin?success=Summit+record+updated+successfully", http.StatusSeeOther)
	}
}
