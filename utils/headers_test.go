package utils

import (
	"testing"
)

func TestHeadersForURLWithSessionAuth(t *testing.T) {
	h := HeadersForURL("http://172.20.3.1:3255/stream/index.m3u8", "X-Sid:test-session-id")

	if got := h.Get("X-Sid"); got != "test-session-id" {
		t.Fatalf("X-Sid header = %q, want test-session-id", got)
	}

	if got := h.Get("Authorization"); got != "" {
		t.Fatalf("Authorization should be empty, got %q", got)
	}
}

func TestHeadersForURLWithMultipleAuthHeaders(t *testing.T) {
	auth := "X-Sid:sid-1|Session:sess-2"
	h := HeadersForURL("http://example.com/live/index.m3u8", auth)

	if got := h.Get("X-Sid"); got != "sid-1" {
		t.Fatalf("X-Sid = %q", got)
	}
	if got := h.Get("Session"); got != "sess-2" {
		t.Fatalf("Session = %q", got)
	}
}

func TestHeadersForURLBearerToken(t *testing.T) {
	h := HeadersForURL("http://example.com/live/index.m3u8", "plain-token")

	if got := h.Get("Authorization"); got != "Bearer plain-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestHeadersForURLAuthorizationHeader(t *testing.T) {
	h := HeadersForURL("http://example.com/live/index.m3u8", "Authorization: Bearer abc")

	if got := h.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization = %q", got)
	}
}
