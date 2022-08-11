package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSuccessfulCallback(t *testing.T) {
	os.Setenv("STRAVA_VERIFY_TOKEN", "mytoken")
	req := httptest.NewRequest(http.MethodGet, "/?hub.mode=subscribe&hub.challenge=mychallenge&hub.verify_token=mytoken", nil)
	w := httptest.NewRecorder()
	callbackHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status to be %d got %d", http.StatusOK, res.StatusCode)
	}

	expected := "{\"hub.challenge\":\"mychallenge\"}\n"
	if string(data) != expected {
		t.Errorf("expected '%s' got '%v'", expected, string(data))
	}
}

func TestMissingChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hub.mode=subscribe", nil)
	w := httptest.NewRecorder()
	callbackHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status to be %d got %d", http.StatusBadRequest, res.StatusCode)
	}

	expected := "missing query param: hub.challenge"
	if string(data) != expected {
		t.Errorf("expected '%s' got '%v'", expected, string(data))
	}
}

func TestMissingVerifyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hub.mode=subscribe&hub.challenge=challenge", nil)
	w := httptest.NewRecorder()
	callbackHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status to be %d got %d", http.StatusBadRequest, res.StatusCode)
	}

	expected := "missing query param: hub.verify_token"
	if string(data) != expected {
		t.Errorf("expected '%s' got '%v'", expected, string(data))
	}
}

func TestIncorrectVerifyToken(t *testing.T) {
	os.Setenv("STRAVA_VERIFY_TOKEN", "mytoken")
	req := httptest.NewRequest(http.MethodGet, "/?hub.mode=subscribe&hub.challenge=challenge&hub.verify_token=wrong", nil)
	w := httptest.NewRecorder()
	callbackHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status to be %d got %d", http.StatusBadRequest, res.StatusCode)
	}

	expected := "verify token mismatch"
	if string(data) != expected {
		t.Errorf("expected '%s' got '%v'", expected, string(data))
	}
}
