// Package database provides functions to initialize and manage the database connection
// and perform schema migrations for the application.
package database

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/lildude/strautomagically/internal/model"
	"gorm.io/gorm"
)

// azureDir is the writable, persistent directory available to the Azure Functions
// custom handler. When present, the SQLite database is stored here so it survives
// restarts and redeploys.
const azureDir = "/home/site/wwwroot"

// dbFile is the name of the SQLite database file.
const dbFile = "database.db"

// testDB holds the test database instance for unit tests.
var testDB *gorm.DB

// SetTestDB sets the test database instance for unit tests.
func SetTestDB(db *gorm.DB) {
	testDB = db
}

// DatabasePath returns the path to the SQLite database file.
//
// It honours the DATABASE_PATH environment variable if set. Otherwise, when
// running on Azure (where /home/site/wwwroot exists and persists) the database
// is stored at /home/site/wwwroot/database.db, and locally it is stored as
// database.db in the current working directory.
func DatabasePath() string {
	if p := os.Getenv("DATABASE_PATH"); p != "" {
		return p
	}

	if info, err := os.Stat(azureDir); err == nil && info.IsDir() {
		return filepath.Join(azureDir, dbFile)
	}

	return dbFile
}

// InitDB initializes the SQLite database connection and performs schema migration.
func InitDB() (*gorm.DB, error) {
	if testDB != nil {
		return testDB, nil
	}

	db, err := gorm.Open(sqlite.Open(DatabasePath()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&model.Athlete{}); err != nil {
		return nil, err
	}

	return db, nil
}
