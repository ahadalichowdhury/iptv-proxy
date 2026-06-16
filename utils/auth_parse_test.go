package utils

import "testing"

func TestParseStreamSource(t *testing.T) {
	source := "https://d1voy022wjnlk0.bioscopelive.com/out/v1/test/index.m3u8|x-authorization=eyJhbGciOiJIUzI1NiJ9.test.sig"
	streamURL, auth := ParseStreamSource(source)

	wantURL := "https://d1voy022wjnlk0.bioscopelive.com/out/v1/test/index.m3u8"
	if streamURL != wantURL {
		t.Fatalf("url = %q, want %q", streamURL, wantURL)
	}
	if auth != "x-authorization:eyJhbGciOiJIUzI1NiJ9.test.sig" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestHeadersForURLWithEqualsAuth(t *testing.T) {
	h := HeadersForURL(
		"https://d1voy022wjnlk0.bioscopelive.com/out/v1/test/index.m3u8",
		"x-authorization:eyJhbGciOiJIUzI1NiJ9.test.sig",
	)

	if got := h.Get("x-authorization"); got != "eyJhbGciOiJIUzI1NiJ9.test.sig" {
		t.Fatalf("x-authorization = %q", got)
	}
}

func TestParseStreamSourceMultipleHeaders(t *testing.T) {
	source := "https://example.com/live.m3u8|Referer=https://example.com/|x-authorization=token123"
	_, auth := ParseStreamSource(source)

	if auth != "Referer:https://example.com/|x-authorization:token123" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestParseStreamSourceWithAmpersandAuth(t *testing.T) {
	url, auth := ParseStreamSource("https://cdn.example.com/live.m3u8|User-Agent=TiviMate/4.7.0&Referer=https://example.com/")

	if url != "https://cdn.example.com/live.m3u8" {
		t.Fatalf("url = %q", url)
	}
	if auth != "User-Agent:TiviMate/4.7.0|Referer:https://example.com/" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestParseStreamSourceRefererWithQueryAndUserAgent(t *testing.T) {
	source := "https://cdn.example.com/live.m3u8|Referer=https://example.com/path?foo=1&bar=2&User-Agent=CustomUA"
	_, auth := ParseStreamSource(source)

	if auth != "Referer:https://example.com/path?foo=1&bar=2|User-Agent:CustomUA" {
		t.Fatalf("auth = %q", auth)
	}
}
