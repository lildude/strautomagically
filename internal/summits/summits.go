// Package summits provides functionality to update and retrieve summit records
// for athletes based on their activities.
package summits

import (
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ActivitySummit struct {
	Type               string
	TotalElevationGain float64
}

// UpdateSummit updates the total elevation gain for a given athlete and year.
func UpdateSummit(db *gorm.DB, activity *strava.Activity) error {
	athleteID := activity.Athlete.ID
	activityYear := activity.StartDate.Year()
	var summit model.Summit

	// Use FirstOrCreate to find the record or create it if it doesn't exist
	result := db.Where(model.Summit{AthleteID: athleteID, Year: int64(activityYear)}).FirstOrCreate(&summit)
	if result.Error != nil {
		logrus.Errorf("Failed to find or create summit record: %v", result.Error)
		return result.Error
	}

	// Update the appropriate field based on activity type
	updated := false
	switch activity.Type {
	case "Run":
		summit.Run += activity.TotalElevationGain
		updated = true
	case "Ride":
		summit.Ride += activity.TotalElevationGain
		updated = true
	default:
		return nil // Ignore unsupported activity types
	}

	// Save the updated summit record only if an update occurred
	if updated {
		saveResult := db.Save(&summit)
		if saveResult.Error != nil {
			logrus.Errorf("Failed to save summit record: %v", saveResult.Error)
			return saveResult.Error
		}
	} else {
		logrus.Debug("No update needed for summit record.")
	}

	return nil
}

// GetSummitForActivity retrieves the latest summit total for the given athlete and year.
func GetSummitForActivity(db *gorm.DB, activity *strava.Activity) (*ActivitySummit, error) {
	if activity.Type != "Run" && activity.Type != "Ride" {
		return nil, nil // Ignore unsupported activity types
	}

	athleteID := activity.Athlete.ID
	activityYear := activity.StartDate.Year()
	var summit model.Summit
	err := db.Where("athlete_id = ? AND year = ?", athleteID, activityYear).First(&summit).Error
	if err != nil && err.Error() == "record not found" {
		return nil, nil
	}

	activitySummit := ActivitySummit{
		Type:               activity.Type,
		TotalElevationGain: activity.TotalElevationGain,
	}

	return &activitySummit, nil
}
