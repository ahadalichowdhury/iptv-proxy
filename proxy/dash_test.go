package proxy

import "testing"

func TestRewriteDashManifestInjectsBaseURL(t *testing.T) {
	engine := NewEngine(10, "http://127.0.0.1:8080", "")
	body := []byte(`<?xml version="1.0"?><MPD type="dynamic"><Period id="0"><AdaptationSet><Representation><SegmentTemplate media="v_$Time$.m4s" initialization="y.m4s"/></Representation></AdaptationSet></Period></MPD>`)

	rewritten, err := engine.rewriteDashManifest(
		body,
		"https://cdn.example.com/HKTV/HKWebApp/manifest.mpd",
		"",
	)
	if err != nil {
		t.Fatalf("rewriteDashManifest() error = %v", err)
	}

	content := string(rewritten)
	if !stringsContains(content, "<BaseURL>https://cdn.example.com/HKTV/HKWebApp/</BaseURL>") {
		t.Fatalf("expected injected BaseURL, got %q", content)
	}
}

func TestRewriteDashManifestResolvesRelativeBaseURL(t *testing.T) {
	engine := NewEngine(10, "http://127.0.0.1:8080", "")
	body := []byte(`<?xml version="1.0"?><MPD type="dynamic"><Period id="1"><BaseURL>dash/</BaseURL><AdaptationSet/></Period></MPD>`)

	rewritten, err := engine.rewriteDashManifest(
		body,
		"https://cdn.example.com/live/stream.isml/dash/.mpd",
		"",
	)
	if err != nil {
		t.Fatalf("rewriteDashManifest() error = %v", err)
	}

	content := string(rewritten)
	if !stringsContains(content, "<BaseURL>https://cdn.example.com/live/stream.isml/dash/dash/</BaseURL>") {
		t.Fatalf("expected resolved BaseURL, got %q", content)
	}
}

func TestIsValidDashManifest(t *testing.T) {
	if !isValidDashManifest([]byte(`<MPD type="dynamic"></MPD>`)) {
		t.Fatal("expected valid dash manifest")
	}
}

func stringsContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
