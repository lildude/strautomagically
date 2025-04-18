package model

import (
	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)

// Athlete represents an athlete in the database
type Athlete struct {
	gorm.Model
	LastActivityID    int64
	StravaAthleteID   int64
	StravaAthleteName string
	StravaAuthToken   pgtype.JSONB `gorm:"type:jsonb;default:'{}'"`
}

// Summit represents a summit record in the database
type Summit struct {
	gorm.Model
	AthleteID int64
	Year      int64
	Run       float64
	Ride      float64
}

// ActivityContent represents the content of an activity
type ActivityContent struct {
	Description string
	Weather     string
	Summit      string
}
