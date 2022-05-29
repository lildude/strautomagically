package main

import (
	"testing"
)

func TestNewClientWithoutToken(t *testing.T) {
	client, _ := newStravaClient()
	if client == nil {
		t.Errorf("expected client to be non-nil")
	}
}

func TestNewClientWithAuthToken(t *testing.T) {
}
