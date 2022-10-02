package callback

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCallbackHandler(t *testing.T) {
	tests := []struct {
		name        string
		queryParams string
		wantStatus  int
	}{
		{
			"successful callback",
			"hub.mode=subscribe&hub.challenge=mychallenge&hub.verify_token=mytoken",
			http.StatusOK,
		},
		{
			"missing query param: hub.challenge",
			"hub.mode=subscribe&hub.verify_token=mytoken",
			http.StatusBadRequest,
		},
		{
			"missing query param: hub.verify_token",
			"hub.mode=subscribe&hub.challenge=mychallenge",
			http.StatusBadRequest,
		},
		{
			"verify token mismatch",
			"hub.mode=subscribe&hub.challenge=mychallenge&hub.verify_token=wrong",
			http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STRAVA_VERIFY_TOKEN", "mytoken")
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryParams, nil)
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
