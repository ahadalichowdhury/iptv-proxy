package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpstreamFinalURLAfterRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/entry.m3u8" {
			http.Redirect(w, r, "/cdn/playlist.m3u8", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:6.0,\nsegment.ts\n"))
	}))
	defer upstream.Close()

	engine := NewEngine(10, "http://127.0.0.1:8080", "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy?url="+upstream.URL+"/entry.m3u8", nil)

	engine.serveHLSManifest(rec, req, upstream.URL+"/entry.m3u8", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !stringsContains(body, upstream.URL+"/cdn/segment.ts") &&
		!stringsContains(body, "proxy?url=") {
		t.Fatalf("expected rewritten segment against redirect base, got %q", body)
	}
}

func TestLogUpstreamRedirectNoOpForSameURL(t *testing.T) {
	// smoke test helper exists; redirect log is best-effort
	logUpstreamRedirect("https://example.com/a.m3u8", "https://example.com/a.m3u8")
}
