package main

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestNewClientWithoutToken(t *testing.T) {
	// Discard logs to avoid polluting test output
	log.SetOutput(ioutil.Discard)
	client, _ := newStravaClient()
	if client == nil {
		t.Errorf("expected client to be non-nil")
	}
}
