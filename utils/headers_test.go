package utils

import (
	"strings"
	"testing"
)

func TestHeadersForURLNSPlayerDefaultProfile(t *testing.T) {
	h := HeadersForURL("http://starhub.pro/live/farhat-3379/67897-913379/744517.ts", "")

	if got := h.Get("User-Agent"); got != NSPlayerUserAgent {
		t.Fatalf("User-Agent = %q, want Dalvik NS profile", got)
	}
	if got := h.Get("Accept-Encoding"); got != "identity" {
		t.Fatalf("Accept-Encoding = %q, want identity", got)
	}
	if got := h.Get("Referer"); got != "" {
		t.Fatalf("Referer should be omitted, got %q", got)
	}
	if got := h.Get("Origin"); got != "" {
		t.Fatalf("Origin should be omitted, got %q", got)
	}
	if got := h.Get("Icy-MetaData"); got != "1" {
		t.Fatalf("Icy-MetaData = %q, want 1 for .ts", got)
	}
}

func TestHeadersForURLNSPlayerWithSessionAuthOnly(t *testing.T) {
	h := HeadersForURL("http://172.20.3.1:3255/stream/index.m3u8", "X-Sid:test-session-id")

	if got := h.Get("X-Sid"); got != "test-session-id" {
		t.Fatalf("X-Sid header = %q, want test-session-id", got)
	}
	if got := h.Get("User-Agent"); !strings.HasPrefix(got, "Dalvik/") {
		t.Fatalf("User-Agent = %q, want NS Dalvik default", got)
	}
	if got := h.Get("Referer"); got != "" {
		t.Fatalf("Referer should be omitted in NS profile, got %q", got)
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
	if got := h.Get("Referer"); got != "" {
		t.Fatalf("Referer should be omitted, got %q", got)
	}
}

func TestHeadersForURLBearerToken(t *testing.T) {
	h := HeadersForURL("http://example.com/live/index.m3u8", "plain-token")

	if got := h.Get("Authorization"); got != "Bearer plain-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := h.Get("User-Agent"); !strings.HasPrefix(got, "Dalvik/") {
		t.Fatalf("User-Agent = %q", got)
	}
}

func TestHeadersForURLAuthorizationHeader(t *testing.T) {
	h := HeadersForURL("http://example.com/live/index.m3u8", "Authorization: Bearer abc")

	if got := h.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestHeadersForURLPreservesPlaylistReferer(t *testing.T) {
	h := HeadersForURL(
		"https://cdn7.example.com/hls/telmunda.m3u8",
		"Referer:https://executeandship.com/",
	)

	if got := h.Get("Referer"); got != "https://executeandship.com/" {
		t.Fatalf("Referer = %q, want https://executeandship.com/", got)
	}
	if got := h.Get("Origin"); got != "" {
		t.Fatalf("Origin should not be auto-injected in explicit mode, got %q", got)
	}
}

func TestHeadersForURLAllPlaylistHeaders(t *testing.T) {
	auth := "Referer:https://executeandship.com/|User-Agent:TiviMate/4.7.0|Cookie:session=abc123"
	h := HeadersForURL("https://cdn7.example.com/hls/telmunda.m3u8", auth)

	if got := h.Get("Referer"); got != "https://executeandship.com/" {
		t.Fatalf("Referer = %q", got)
	}
	if got := h.Get("User-Agent"); got != "TiviMate/4.7.0" {
		t.Fatalf("User-Agent = %q", got)
	}
	if got := h.Get("Cookie"); got != "session=abc123" {
		t.Fatalf("Cookie = %q", got)
	}
	if got := h.Get("Origin"); got != "" {
		t.Fatalf("Origin should not be injected, got %q", got)
	}
}

func TestHeadersForURLExplicitCookieOnly(t *testing.T) {
	h := HeadersForURL(
		"http://1495678.kawasakininja.us/live/farhat-3379/67897-913379/744517.ts",
		"Cookie:session=abc",
	)

	if got := h.Get("Cookie"); got != "session=abc" {
		t.Fatalf("Cookie = %q", got)
	}
	if got := h.Get("Referer"); got != "" {
		t.Fatalf("Referer should not be injected, got %q", got)
	}
}

func TestHeadersForUpstreamMediaSegmentIcyMetadata(t *testing.T) {
	h := HeadersForUpstream(
		"http://starhub.pro/live/farhat-3379/67897-913379/744517",
		"",
		UpstreamHeaderOptions{MediaSegment: true},
	)

	if got := h.Get("Icy-MetaData"); got != "1" {
		t.Fatalf("Icy-MetaData = %q, want 1 for /live/ media segment", got)
	}
}

func TestHasExplicitStreamIdentity(t *testing.T) {
	if HasExplicitStreamIdentity(ParseAuthHeaderMap("User-Agent:Custom")) {
		t.Fatal("User-Agent alone should not trigger explicit mode")
	}
	if !HasExplicitStreamIdentity(ParseAuthHeaderMap("Referer:https://a.com")) {
		t.Fatal("Referer should trigger explicit mode")
	}
	if !HasExplicitStreamIdentity(ParseAuthHeaderMap("Cookie:a=b")) {
		t.Fatal("Cookie should trigger explicit mode")
	}
}

func TestParseAuthHeaderMapRefererWithQuery(t *testing.T) {
	auth := "Referer:https://example.com/path?foo=1&bar=2|User-Agent:CustomUA"
	pairs := ParseAuthHeaderMap(auth)

	if pairs["Referer"] != "https://example.com/path?foo=1&bar=2" {
		t.Fatalf("Referer = %q", pairs["Referer"])
	}
	if pairs["User-Agent"] != "CustomUA" {
		t.Fatalf("User-Agent = %q", pairs["User-Agent"])
	}
}
