// Package summits provides functionality to update and retrieve summit records
// for athletes based on their activities.
package summits

import (
	"errors"
	"log/slog"

	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"gorm.io/gorm"
)

// ActivitySummit represents the summit contribution of a single activity.
type ActivitySummit struct {
	Type               string
	TotalElevationGain float64
}

// UpdateSummit updates the total elevation gain for a given athlete and year
// based on the supplied activity.
func UpdateSummit(db *gorm.DB, activity *strava.Activity) error {
	athleteID := activity.Athlete.ID
	activityYear := activity.StartDate.Year()
	var summit model.Summit

	// Use FirstOrCreate to find the record or create it if it doesn't exist
	result := db.Where(model.Summit{AthleteID: athleteID, Year: int64(activityYear)}).FirstOrCreate(&summit)
	if result.Error != nil {
		slog.Error("failed to find or create summit record", "error", result.Error)
		return result.Error
	}

	// Update the appropriate field based on activity type
	switch activity.Type {
	case "Run":
		summit.Run += activity.TotalElevationGain
	case "Ride":
		summit.Ride += activity.TotalElevationGain
	default:
		return nil // Ignore unsupported activity types
	}

	if saveResult := db.Save(&summit); saveResult.Error != nil {
		slog.Error("failed to save summit record", "error", saveResult.Error)
		return saveResult.Error
	}

	return nil
}

// GetSummitForActivity retrieves the summit contribution for the given activity.
func GetSummitForActivity(db *gorm.DB, activity *strava.Activity) (*ActivitySummit, error) {
	if activity.Type != "Run" && activity.Type != "Ride" {
		return nil, nil // Ignore unsupported activity types
	}

	athleteID := activity.Athlete.ID
	activityYear := activity.StartDate.Year()
	var summit model.Summit
	err := db.Where("athlete_id = ? AND year = ?", athleteID, activityYear).First(&summit).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("failed to query summit record", "error", err)
		return nil, err
	}

	activitySummit := ActivitySummit{
		Type:               activity.Type,
		TotalElevationGain: activity.TotalElevationGain,
	}

	return &activitySummit, nil
}
