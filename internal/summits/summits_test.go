package summits

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateSummit(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(io.Discard)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	db.AutoMigrate(&model.Athlete{}, &model.Summit{})

	tests := []struct {
		desc          string
		activity      *strava.Activity
		expectedRun   float64
		expectedRide  float64
		expectedError bool
	}{
		{
			desc: "Create new summit record for a run",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Run",
				TotalElevationGain: 500,
				StartDate:          time.Now(),
			},
			expectedRun:   500,
			expectedRide:  0,
			expectedError: false,
		},
		{
			desc: "Update summit record for a ride",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Ride",
				TotalElevationGain: 300,
				StartDate:          time.Now(),
			},
			expectedRun:   500,
			expectedRide:  300,
			expectedError: false,
		},
		{
			desc: "Add more elevation gain to the run",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Run",
				TotalElevationGain: 200,
				StartDate:          time.Now(),
			},
			expectedRun:   700,
			expectedRide:  300,
			expectedError: false,
		},
		{
			desc: "Create new summit record for a different year",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Ride",
				TotalElevationGain: 1500,
				StartDate:          time.Now().AddDate(2, 0, 0),
			},
			expectedRun:   0,
			expectedRide:  1500,
			expectedError: false,
		},
		{
			desc: "Unsupported activity type",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Swim",
				TotalElevationGain: 200,
				StartDate:          time.Now(),
			},
			expectedRun:   700,
			expectedRide:  300,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := UpdateSummit(db, tt.activity)
			if (err != nil) != tt.expectedError {
				t.Fatalf("unexpected error: %v", err)
			}

			var summit model.Summit
			activityYear := int64(tt.activity.StartDate.Year())
			err = db.Where("athlete_id = ? AND year = ?", tt.activity.Athlete.ID, activityYear).First(&summit).Error
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if summit.Run != tt.expectedRun {
				t.Errorf("expected Run to be %f, got %f", tt.expectedRun, summit.Run)
			}
			if summit.Ride != tt.expectedRide {
				t.Errorf("expected Ride to be %f, got %f", tt.expectedRide, summit.Ride)
			}
		})
	}
}

func TestGetSummitForActivity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	db.AutoMigrate(&model.Athlete{}, &model.Summit{})

	summit := &model.Summit{
		AthleteID: 1,
		Year:      int64(time.Now().Year()),
		Run:       1000,
		Ride:      500,
	}
	db.Create(summit)

	tests := []struct {
		desc              string
		activity          *strava.Activity
		expectedType      string
		expectedElevation float64
		expectedNilResult bool
	}{
		{
			desc: "Get summit for a run activity",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Run",
				TotalElevationGain: 500,
				StartDate:          time.Now(),
			},
			expectedType:      "Run",
			expectedElevation: 500,
			expectedNilResult: false,
		},
		{
			desc: "Get summit for a ride activity",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Ride",
				TotalElevationGain: 500,
				StartDate:          time.Now(),
			},
			expectedType:      "Ride",
			expectedElevation: 500,
			expectedNilResult: false,
		},
		{
			desc: "Get summit for an unsupported activity type",
			activity: &strava.Activity{
				Athlete:            strava.Athlete{ID: 1},
				Type:               "Swim",
				TotalElevationGain: 200,
				StartDate:          time.Now(),
			},
			expectedType:      "",
			expectedElevation: 0,
			expectedNilResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := GetSummitForActivity(db, tt.activity)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectedNilResult {
				if result != nil {
					t.Fatal("expected result to be nil")
				}
			} else {
				if result == nil {
					t.Fatal("expected result not to be nil")
				}
				if result.Type != tt.expectedType {
					t.Errorf("expected Type to be '%s', got '%s'", tt.expectedType, result.Type)
				}
				if result.TotalElevationGain != tt.expectedElevation {
					t.Errorf("expected TotalElevationGain to be %f, got %f", tt.expectedElevation, result.TotalElevationGain)
				}
			}
		})
	}
}
