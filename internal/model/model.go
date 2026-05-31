// Package model defines the data models used in the application database.
package model

import "gorm.io/gorm"

// Athlete represents an athlete in the database.
type Athlete struct {
	gorm.Model

	LastActivityID    int64
	StravaAthleteID   int64
	StravaAthleteName string
	StravaAccessToken string
	// StravaAuthToken stores the athlete's OAuth2 token as a JSON-encoded string.
	StravaAuthToken    string
	StravaRefreshToken string
}

// Summit represents a summit record in the database, tracking the total
// elevation gain for an athlete in a given year, split by activity type.
type Summit struct {
	gorm.Model

	AthleteID int64
	Year      int64
	Run       float64
	Ride      float64
}
