package admin

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/lildude/strautomagically/internal/auth"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/sessions"
)

// ShowLoginForm displays the admin login page.
func ShowLoginForm(w http.ResponseWriter, r *http.Request) {
	// Use the templates variable defined in dashboard.go
	err := templates.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleLogin processes the admin login attempt.
func HandleLogin(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		adminUser, err := database.GetAdminUser(db, username)
		if err != nil {
			log.Printf("Error getting admin user: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if adminUser == nil || !auth.CheckPasswordHash(password, adminUser.PasswordHash) {
			log.Printf("Login failed for user: %s", username)
			// Use the templates variable defined in dashboard.go
			err := templates.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Invalid credentials"})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Authentication successful
		session, err := sessions.GetSession(r)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		session.Values["authenticated"] = true
		session.Values["username"] = username
		if err := sessions.SaveSession(r, w, session); err != nil {
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusFound)
	}
}

// HandleLogout logs the admin user out.
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := sessions.GetSession(r)
	if err != nil {
		http.Error(w, "Failed to get session", http.StatusInternalServerError)
		return
	}

	// Clear session values
	delete(session.Values, "authenticated")
	delete(session.Values, "username")
	session.Options.MaxAge = -1 // Expire cookie immediately

	if err := sessions.SaveSession(r, w, session); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/login", http.StatusFound)
}
