package main

import "testing"

func TestBrowserURLShowsLocalhostForAllInterfaceBind(t *testing.T) {
	got := browserURL("0.0.0.0", "8000")
	if got != "http://localhost:8000" {
		t.Fatalf("browserURL returned %q, want localhost URL", got)
	}
}

func TestBrowserURLKeepsExplicitHost(t *testing.T) {
	got := browserURL("127.0.0.1", "9000")
	if got != "http://127.0.0.1:9000" {
		t.Fatalf("browserURL returned %q, want explicit host URL", got)
	}
}
