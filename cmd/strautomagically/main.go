package main

import (
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/database"
	adminHandlers "github.com/lildude/strautomagically/internal/handlers/admin"
	"github.com/lildude/strautomagically/internal/handlers/auth"
	"github.com/lildude/strautomagically/internal/handlers/callback"
	"github.com/lildude/strautomagically/internal/handlers/update"
	"github.com/lildude/strautomagically/internal/middleware"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/sirupsen/logrus"

	// Autoloads .env file to supply environment variables.
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq" // PostgreSQL driver
)

var Version = "dev"

func main() {
	// Database setup
	gormDB, err := database.InitDB() // Use InitDB which returns *gorm.DB
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}
	sqlDB, err := gormDB.DB() // Get the underlying *sql.DB
	if err != nil {
		logrus.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	// Note: We don't defer sqlDB.Close() here because gorm manages the connection pool.
	// Gorm's Close() method handles closing the underlying connection if necessary.

	// Use environment variables for initial admin credentials or provide defaults
	adminUser := os.Getenv("ADMIN_USERNAME")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "password" // Change this default!
		logrus.Warn("ADMIN_PASSWORD not set, using default 'password'. Set this environment variable.")
	}
	if err := database.InitAdminUser(sqlDB, adminUser, adminPass); err != nil {
		logrus.Fatalf("Failed to initialize admin user: %v", err)
	}

	// HTTP Server setup
	mux := http.NewServeMux()

	// Serve static files (including admin CSS)
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public routes
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/webhook", webhookHandler)
	mux.HandleFunc("/auth", auth.AuthHandler)
	mux.HandleFunc("/auth/callback", callback.CallbackHandler)

	// Public admin routes (login)
	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminHandlers.ShowLoginForm(w, r)
		case http.MethodPost:
			adminHandlers.HandleLogin(sqlDB)(w, r) // Login uses *sql.DB
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Protected admin routes - Pass *gorm.DB
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", adminHandlers.ShowDashboard(gormDB))
	adminMux.HandleFunc("/athletes/update", adminHandlers.HandleAthleteUpdate(gormDB))
	adminMux.HandleFunc("/summits/update", adminHandlers.HandleSummitUpdate(gormDB))
	adminMux.HandleFunc("/logout", adminHandlers.HandleLogout)

	// Apply authentication middleware ONLY to the protected admin routes
	protectedAdminHandler := middleware.RequireAuthentication(adminMux)
	mux.Handle("/admin/", http.StripPrefix("/admin", protectedAdminHandler))

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}

	logrus.Infof("Starting server on port %s", port)
	//#nosec: G114
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logrus.Fatalf("Server failed: %v", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Strautomagically-Version", Version)
	// If DISABLE_SIGNUP is set, return a 200 OK response
	if os.Getenv("DISABLE_SIGNUP") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	} else {
		stateToken := os.Getenv("STRAVA_STATE_TOKEN")
		url := strava.OauthConfig.AuthCodeURL(stateToken)
		if _, err := w.Write([]byte(`<!DOCTYPE html>
			<html>
			<head>
				<title>Strautomagically</title>
			</head>
			<body>
				<h1>Strautomagically</h1>
				<p>
				<a href="` + url + `" style="display: inline-block; padding: 10px 20px; background-color: #fc5200; color: white; text-decoration: none; border-radius: 5px;">Connect</a>
				</p>
			</body>
			</html>`)); err != nil {
			logrus.WithError(err).Error("Failed to write index page response")
		}
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		callback.CallbackHandler(w, r)
	}
	if r.Method == http.MethodPost {
		update.UpdateHandler(w, r)
	}
}
