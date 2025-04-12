package database

import (
	"log"
	"os"

	"github.com/lildude/strautomagically/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SetTestDB sets the test database instance for unit tests
var testDB *gorm.DB

func SetTestDB(db *gorm.DB) {
	testDB = db
}

// InitDB initializes the database connection and performs schema migration
func InitDB() (*gorm.DB, error) {
	if testDB != nil {
		return testDB, nil
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&model.Athlete{}, &model.Summit{})
	if err != nil {
		return nil, err
	}

	return db, nil
}
