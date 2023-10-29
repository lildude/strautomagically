package calendarevent

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestGetTrainerRoadCalendarEvent(t *testing.T) {
	resp, _ := os.ReadFile("testdata/trainerroad.ics")
	mockClient := &MockClient{
		DoFunc: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(resp))),
			}, nil
		},
	}
	cs := CalendarService{
		Client:  mockClient,
		BaseURL: "https://api.trainerroad.com/v1/calendar/ics",
		CalID:   "foobar",
	}
	t.Run("should return an event", func(t *testing.T) {
		start := time.Date(2023, 12, 6, 0, 0, 0, 1, time.UTC)

		event, err := cs.GetCalendarEvent(start)
		if err != nil {
			t.Errorf("unexpected error = %v", err)
			return
		}
		if event == nil {
			t.Errorf("expected an event but got %v", event)
			return
		}
		if event.Summary != "Truchas -3" {
			t.Errorf("expected event.Summary to be Truchas -3 but got %v", event.Summary)
		}
	})

	t.Run("should return an error if the request fails", func(t *testing.T) {
		mockClient := &MockClient{
			DoFunc: func(*http.Request) (*http.Response, error) {
				return nil, http.ErrHandlerTimeout
			},
		}
		cs.Client = mockClient

		start := time.Date(2023, 12, 6, 0, 0, 0, 1, time.UTC)
		_, err := cs.GetCalendarEvent(start)
		if err == nil {
			t.Errorf("expected an error but got nil")
			return
		}
	})

	t.Run("should return nil if no events found", func(t *testing.T) {
		start := time.Date(2025, 12, 6, 0, 0, 0, 1, time.UTC)
		event, _ := cs.GetCalendarEvent(start)
		if event != nil {
			t.Errorf("expected event to be nil but got %v", event)
			return
		}
	})
}
