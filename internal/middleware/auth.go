package middleware

import (
	"net/http"

	"github.com/lildude/strautomagically/internal/sessions"
)

// RequireAuthentication is a middleware that checks if the user is authenticated.
func RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := sessions.GetSession(r)
		if err != nil {
			http.Error(w, "Failed to get session", http.StatusInternalServerError)
			return
		}

		// Check if user is authenticated
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// User is authenticated, call the next handler
		next.ServeHTTP(w, r)
	})
}
