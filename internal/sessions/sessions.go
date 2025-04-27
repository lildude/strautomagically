// Package sessions provides session management for the admin interface.
package sessions

import (
	"net/http"
	"os"

	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

// SetupSessionStore initializes the session store and must be called before using sessions.
func SetupSessionStore() {
	// Ensure SESSION_KEY is set
	if os.Getenv("SESSION_KEY") == "" {
		panic("SESSION_KEY environment variable not set")
	}
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 8, // 8 hours
		HttpOnly: true,
		Secure:   os.Getenv("ENV") != "dev", // Use secure cookies in production
		SameSite: http.SameSiteLaxMode,
	}
}

// GetSession retrieves a session from the request.
func GetSession(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, "admin-session")
}

// SaveSession saves the session.
func SaveSession(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	return store.Save(r, w, session)
}
