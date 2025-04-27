// Package database provides functions to initialize and manage the database connection
// and perform schema migrations for the application.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/lildude/strautomagically/internal/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SetTestDB sets the test database instance for unit tests.
var testDB *gorm.DB

func SetTestDB(db *gorm.DB) {
	testDB = db
}

// InitDB initializes the database connection and performs schema migration.
func InitDB() (*gorm.DB, error) {
	if testDB != nil {
		return testDB, nil
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logrus.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&model.Athlete{}, &model.Summit{}, &model.AdminUser{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// InitAdminUser ensures an admin user exists, creating one if necessary.
// Uses *sql.DB because it executes raw SQL.
func InitAdminUser(db *sql.DB, username, password string) error {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM admin_users WHERE username = $1)", username).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check if admin user exists: %w", err)
	}

	if !exists {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		_, err = db.Exec("INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)", username, string(hashedPassword))
		if err != nil {
			return fmt.Errorf("failed to insert initial admin user: %w", err)
		}
		logrus.Infof("Initial admin user '%s' created.", username)
	}
	return nil
}

// GetAdminUser retrieves the admin user by username.
// Uses *sql.DB because it executes raw SQL.
func GetAdminUser(db *sql.DB, username string) (*model.AdminUser, error) {
	user := &model.AdminUser{}
	err := db.QueryRow("SELECT username, password_hash FROM admin_users WHERE username = $1", username).Scan(&user.Username, &user.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}
	return user, nil
}

// --- Athlete CRUD --- Uses *gorm.DB ---

// GetAllAthletes retrieves all athletes from the database.
func GetAllAthletes(db *gorm.DB) ([]model.Athlete, error) {
	var athletes []model.Athlete
	if err := db.Order("id").Find(&athletes).Error; err != nil {
		return nil, fmt.Errorf("failed to query athletes: %w", err)
	}
	return athletes, nil
}

// GetAthleteByID retrieves a single athlete by its ID.
func GetAthleteByID(db *gorm.DB, id uint) (*model.Athlete, error) {
	var athlete model.Athlete
	if err := db.First(&athlete, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get athlete by ID %d: %w", id, err)
	}
	return &athlete, nil
}

// UpdateAthlete updates an existing athlete in the database.
func UpdateAthlete(db *gorm.DB, athlete *model.Athlete) error {
	// Save updates all fields, including zero values.
	// Use Updates if you only want to update non-zero fields or specific fields.
	if err := db.Save(athlete).Error; err != nil {
		return fmt.Errorf("failed to update athlete %d: %w", athlete.ID, err)
	}
	return nil
}

// --- Summit CRUD --- Uses *gorm.DB ---

// GetAllSummits retrieves all summits from the database.
func GetAllSummits(db *gorm.DB) ([]model.Summit, error) {
	var summits []model.Summit
	// Example ordering, adjust as needed
	if err := db.Order("athlete_id, year desc").Find(&summits).Error; err != nil {
		return nil, fmt.Errorf("failed to query summits: %w", err)
	}
	return summits, nil
}

// GetSummitByID retrieves a single summit by its ID.
func GetSummitByID(db *gorm.DB, id uint) (*model.Summit, error) {
	var summit model.Summit
	if err := db.First(&summit, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get summit by ID %d: %w", id, err)
	}
	return &summit, nil
}

// UpdateSummit updates an existing summit in the database.
func UpdateSummit(db *gorm.DB, summit *model.Summit) error {
	if err := db.Save(summit).Error; err != nil {
		return fmt.Errorf("failed to update summit %d: %w", summit.ID, err)
	}
	return nil
}

// DeleteAthlete removes an athlete from the database.
// Consider implications before enabling deletion (e.g., related summits).
// func DeleteAthlete(db *gorm.DB, id uint) error {
// 	if err := db.Delete(&model.Athlete{}, id).Error; err != nil {
// 		return fmt.Errorf("failed to delete athlete %d: %w", id, err)
// 	}
// 	return nil
// }

// DeleteSummit removes a summit from the database.
// func DeleteSummit(db *gorm.DB, id uint) error {
// 	if err := db.Delete(&model.Summit{}, id).Error; err != nil {
// 		return fmt.Errorf("failed to delete summit %d: %w", id, err)
// 	}
// 	return nil
// }
