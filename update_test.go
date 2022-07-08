package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	mockhttp "github.com/karupanerura/go-mock-http-response"
)

type MockTransport struct {
	Host      string
	Transport *http.Transport
}

func (m MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = m.Host

	return m.Transport.RoundTrip(req)
}

func mockResponse(statusCode int, headers map[string]string, body []byte) {
	http.DefaultClient = mockhttp.NewResponseMock(statusCode, headers, body).MakeClient()
}

func TestUpdateHandler(t *testing.T) {
	// skip until we've refactored
	t.SkipNow()
	testCases := []struct {
		name             string
		payload          []byte
		expLogMsg        string
		mockActivityResp []byte
	}{
		{
			name:      "update event",
			payload:   []byte(`{"aspect_type":"update","event_time":1516126040,"object_id":1360128428,"object_type":"activity","owner_id":134815,"subscription_id": 120475,"updates":{"title": "Messy"}}`),
			expLogMsg: "ignoring non-create webhook\n",
		},
		{
			name:      "delete event",
			payload:   []byte(`{"aspect_type":"update","event_time":1516126040,"object_id":1360128428,"object_type":"activity","owner_id":134815,"subscription_id":120475}`),
			expLogMsg: "ignoring non-create webhook\n",
		},
		{
			name:             "create event with nothing to do",
			payload:          []byte(`{"aspect_type":"create","event_time":1516126040,"object_id":1360128428,"object_type":"activity","owner_id":134815,"subscription_id":120475}`),
			expLogMsg:        "nothing to do\n",
			mockActivityResp: []byte(`{"id":1360128428,"type":"Run","elapsed_time":99009, "external_id":"garmin_push_12345","trainer":"false"}`),
		},
	}

	r := miniredis.RunT(t)
	defer r.Close()
	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Use a faux logger so we can parse the content to find our debug messages to confirm our tests
			var fauxLog bytes.Buffer
			log.SetFlags(0)
			log.SetOutput(&fauxLog)
			req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(tc.payload))
			rr := httptest.NewRecorder()

			// Mock the Strava Server for our OAuth requests and the activity endpoint
			// mux := http.NewServeMux()
			// mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
			// 	fmt.Println("got oauth request")
			// 	w.WriteHeader(http.StatusOK)
			// 	w.Write([]byte(`{"token_type": "Bearer","expires_at": 1568775134,"expires_in": 21600,"refresh_token": "123456","access_token": "987654"}`))
			// })
			// mux.HandleFunc("/api/v3/activities/1360128428", func(w http.ResponseWriter, r *http.Request) {
			// 	fmt.Println("got activity request")
			// 	w.WriteHeader(http.StatusOK)
			// 	w.Write(tc.mockActivityResp)
			// })

			// s := httptest.NewServer(mux)
			// defer s.Close()

			// serverURL, _ := url.Parse(s.URL)
			// Transport = MockTransport{
			// 	Host:      serverURL.Host,
			// 	Transport: &http.Transport{},
			// }
			// http.DefaultTransport = Transport

			// Lets try with gock
			// defer gock.Off()

			// gock.Observe(gock.DumpRequest)
			// gock.New("https://www.strava.com").
			// 	Post("/api/v3/oauth/token").
			// 	Reply(200).
			// 	JSON(map[string]string{"token_type": "Bearer", "expires_at": "1568775134", "expires_in": "21600", "refresh_token": "123456", "access_token": "987654"})

			// gock.New("https://www.strava.com").
			// 	Get("/api/v3/activities/1360128428").
			// 	Reply(200).
			// 	JSON(map[string]string{"id": "1360128428", "type": "Run", "elapsed_time": "99009", "external_id": "garmin_push_12345", "trainer": "false"})

			// updateHandler(rr, req)

			// What about using mock-http-response
			mockResponse(http.StatusOK, map[string]string{"Content-Type": "application/json"}, []byte(`{"token_type": "Bearer","expires_at": 1568775134,"expires_in": 21600,"refresh_token": "123456","access_token": "987654"}`))
			mockResponse(http.StatusOK, map[string]string{"Content-Type": "application/json"}, tc.mockActivityResp)

			handler := http.HandlerFunc(updateHandler)
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
			}
			if tc.expLogMsg != fauxLog.String() {
				t.Errorf("expected log msg '%s', got '%s'", tc.expLogMsg, fauxLog.String())
			}
		})
	}
}
