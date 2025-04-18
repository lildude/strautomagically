package callback

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallbackHandler(t *testing.T) {
	tests := []struct {
		name        string
		queryParams string
		wantStatus  int
	}{
		{
			name:        "Successful callback",
			queryParams: "hub.mode=subscribe&hub.challenge=mychallenge&hub.verify_token=mytoken",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "missing query param: hub.challenge",
			queryParams: "hub.mode=subscribe&hub.verify_token=mytoken",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing query param: hub.verify_token",
			queryParams: "hub.mode=subscribe&hub.challenge=mychallenge",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "verify token mismatch",
			queryParams: "hub.mode=subscribe&hub.challenge=mychallenge&hub.verify_token=wrong",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STRAVA_VERIFY_TOKEN", "mytoken")
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryParams, http.NoBody)
			w := httptest.NewRecorder()
			CallbackHandler(w, req)
			res := w.Result()
			defer res.Body.Close()
			data, err := io.ReadAll(res.Body)
			if err != nil {
				t.Errorf("expected error to be nil got %v", err)
			}

			if res.StatusCode != tt.wantStatus {
				t.Errorf("expected status to be %d got %d", tt.wantStatus, res.StatusCode)
			}

			if tt.wantStatus == http.StatusOK {
				expected := "{\"hub.challenge\":\"mychallenge\"}\n"
				if string(data) != expected {
					t.Errorf("expected '%s' got '%v'", expected, string(data))
				}
			}

			if tt.wantStatus == http.StatusBadRequest {
				expected := tt.name
				if string(data) != expected {
					t.Errorf("expected '%s' got '%v'", expected, string(data))
				}
			}
		})
	}
}
