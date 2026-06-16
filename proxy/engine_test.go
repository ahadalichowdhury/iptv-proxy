package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpstreamFollowsRedirectWithCookieJar(t *testing.T) {
	t.Parallel()

	var finalRequest *http.Request

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") == "" {
			t.Fatalf("expected token query on redirect target, got %q", r.URL.String())
		}
		if cookie, err := r.Cookie("gate"); err != nil || cookie.Value != "abc" {
			t.Fatalf("expected gate cookie on redirect target, got err=%v", err)
		}
		finalRequest = r
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer tokenServer.Close()

	gateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "gate", Value: "abc", Path: "/"})
		location := tokenServer.URL + "/segment.ts?token=signed"
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer gateServer.Close()

	engine := NewEngine(8, "http://127.0.0.1:8080/proxy")
	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/proxy?url="+gateServer.URL, nil)

	resp, cancel, err := engine.doUpstream(req, gateServer.URL, "", upstreamStream)
	if err != nil {
		t.Fatalf("doUpstream error: %v", err)
	}
	cancel()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if finalRequest == nil {
		t.Fatal("redirect target was not requested")
	}
	if !strings.Contains(finalRequest.URL.String(), "token=signed") {
		t.Fatalf("final url = %q", finalRequest.URL.String())
	}
}

func TestInheritRedirectHeaders(t *testing.T) {
	first, err := http.NewRequest(http.MethodGet, "http://starhub.pro/live/744517.ts", nil)
	if err != nil {
		t.Fatal(err)
	}
	first.Header.Set("User-Agent", "Dalvik-test")
	first.Header.Set("Icy-MetaData", "1")
	first.Header.Set("X-Custom", "sid-1")

	next, err := http.NewRequest(http.MethodGet, "http://cdn.example/744517.ts?token=x", nil)
	if err != nil {
		t.Fatal(err)
	}

	inheritRedirectHeaders(next, first)

	if got := next.Header.Get("User-Agent"); got != "Dalvik-test" {
		t.Fatalf("User-Agent = %q", got)
	}
	if got := next.Header.Get("Icy-MetaData"); got != "1" {
		t.Fatalf("Icy-MetaData = %q", got)
	}
	if got := next.Header.Get("X-Custom"); got != "sid-1" {
		t.Fatalf("X-Custom = %q", got)
	}
}

func TestManifestCacheTTL(t *testing.T) {
	master := []byte(`#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000000
stream.m3u8
`)
	if ttl, ok := manifestCacheTTL(master); !ok || ttl != masterPlaylistCacheTTL {
		t.Fatalf("master playlist cache = (%v, %v)", ttl, ok)
	}

	liveMedia := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:100
#EXTINF:6.0,
segment100.ts
#EXTINF:6.0,
segment101.ts
`)
	if ttl, ok := manifestCacheTTL(liveMedia); ok {
		t.Fatalf("live media playlist should not cache, got ttl=%v", ttl)
	}

	vod := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
segment0.ts
#EXT-X-ENDLIST
`)
	if ttl, ok := manifestCacheTTL(vod); !ok || ttl != vodPlaylistCacheTTL {
		t.Fatalf("vod playlist cache = (%v, %v)", ttl, ok)
	}
}

func TestShouldBypassManifestCache(t *testing.T) {
	liveMedia := []byte("#EXTM3U\n#EXTINF:6.0,\nseg.ts\n")
	if !shouldBypassManifestCache(liveMedia) {
		t.Fatal("live media should bypass cache")
	}
}

func TestIsHLSManifest(t *testing.T) {
	if !isHLSManifest([]byte("#EXTM3U\n#EXTINF:6.0,\n")) {
		t.Fatal("expected valid manifest")
	}
	if isHLSManifest([]byte("<html><title>403 Forbidden</title></html>")) {
		t.Fatal("html error page must not be treated as manifest")
	}
}

func TestHandleStreamPostRejectsDataURL(t *testing.T) {
	engine := NewEngine(8, "http://127.0.0.1:8080/proxy")
	dataURL := "data:application/json;base64,eyJrZXlzIjpbXX0="
	req := httptest.NewRequest(
		http.MethodPost,
		"http://proxy.local/proxy?url="+dataURL,
		strings.NewReader("license"),
	)
	rec := httptest.NewRecorder()

	engine.HandleStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIsUpstreamHTTPURL(t *testing.T) {
	if !isUpstreamHTTPURL("https://cdn.example.com/cenc.mpd") {
		t.Fatal("https url should be allowed")
	}
	if isUpstreamHTTPURL("data:application/json;base64,abc") {
		t.Fatal("data url should be rejected")
	}
}
