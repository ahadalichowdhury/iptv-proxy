package utils

import (
	"net/http"
	"testing"
)

func TestSessionAuthFromHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-Sid", "abc-123")
	h.Set("Session", "sess-456")

	got := SessionAuthFromHeaders(h)
	want := "X-Sid:abc-123|Session:sess-456"
	if got != want {
		t.Fatalf("SessionAuthFromHeaders() = %q, want %q", got, want)
	}
}

func TestMergeAuth(t *testing.T) {
	t.Run("upstream sid added", func(t *testing.T) {
		got := MergeAuth("", "X-Sid:new-sid")
		if got != "X-Sid:new-sid" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("upstream sid overrides stale sid", func(t *testing.T) {
		got := MergeAuth("X-Sid:old-sid", "X-Sid:new-sid")
		if got != "X-Sid:new-sid" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("preserves bearer and adds sid", func(t *testing.T) {
		got := MergeAuth("my-token", "X-Sid:new-sid")
		if got != "X-Sid:new-sid|Authorization:Bearer my-token" {
			t.Fatalf("got %q", got)
		}
	})
}
