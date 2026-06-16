package utils

import (
	"net/http"
	"strings"
)

// BrowserUserAgent is used for sources that explicitly expect a desktop browser.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// NSPlayerUserAgent mimics Network Stream Player's default Android client fingerprint
// when the playlist does not supply Referer, Origin, or Cookie.
const NSPlayerUserAgent = "Dalvik/2.1.0 (Linux; U; Android 13; SM-A037F Build/TP1A.220624.014) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.45 Mobile Safari/537.36"

// UpstreamHeaderOptions tunes header generation for manifests vs media segments.
type UpstreamHeaderOptions struct {
	// MediaSegment is true for TS/fMP4/AAC segment fetches (not master/media playlists).
	MediaSegment bool
}

// HeadersForURL builds upstream request headers using the NS Player profile by default.
func HeadersForURL(rawURL, auth string) http.Header {
	return HeadersForUpstream(rawURL, auth, UpstreamHeaderOptions{})
}

// HeadersForUpstream builds upstream headers.
//
// Profile selection:
//   - Explicit playlist: auth contains Referer, Origin, and/or Cookie → send those exactly.
//   - NS Player default: no Referer/Origin/Cookie → Dalvik UA, identity encoding, no Referer/Origin.
func HeadersForUpstream(rawURL, auth string, opts UpstreamHeaderOptions) http.Header {
	authHeaders := ParseAuthHeaderMap(auth)
	if HasExplicitStreamIdentity(authHeaders) {
		return buildExplicitPlaylistHeaders(rawURL, authHeaders, opts)
	}
	return buildNSPlayerHeaders(rawURL, authHeaders, opts)
}

// HasExplicitStreamIdentity reports whether the source supplied identity headers
// that must be forwarded verbatim (IPTV playlist / NS Player manual fields).
func HasExplicitStreamIdentity(authHeaders map[string]string) bool {
	for key := range authHeaders {
		switch CanonicalHeaderName(key) {
		case "Referer", "Origin", "Cookie":
			return true
		}
	}
	return false
}

func buildNSPlayerHeaders(rawURL string, auth map[string]string, opts UpstreamHeaderOptions) http.Header {
	h := make(http.Header)

	if ua := strings.TrimSpace(auth["User-Agent"]); ua != "" {
		h.Set("User-Agent", ua)
	} else {
		h.Set("User-Agent", NSPlayerUserAgent)
	}

	h.Set("Accept-Encoding", "identity")
	h.Set("Accept", "*/*")
	h.Set("Connection", "keep-alive")

	applyAuthHeadersExcept(h, auth, "User-Agent")

	if shouldSendIcyMetadata(rawURL, auth, opts) {
		if h.Get("Icy-MetaData") == "" {
			h.Set("Icy-MetaData", "1")
		}
	}

	return h
}

func buildExplicitPlaylistHeaders(rawURL string, auth map[string]string, opts UpstreamHeaderOptions) http.Header {
	h := make(http.Header)

	for key, value := range auth {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		h.Set(CanonicalHeaderName(key), value)
	}

	if h.Get("User-Agent") == "" {
		h.Set("User-Agent", NSPlayerUserAgent)
	}
	if h.Get("Accept-Encoding") == "" {
		h.Set("Accept-Encoding", "identity")
	}
	if h.Get("Accept") == "" {
		h.Set("Accept", "*/*")
	}
	if h.Get("Connection") == "" {
		h.Set("Connection", "keep-alive")
	}

	if shouldSendIcyMetadata(rawURL, auth, opts) && h.Get("Icy-MetaData") == "" {
		h.Set("Icy-MetaData", "1")
	}

	return h
}

func applyAuthHeadersExcept(h http.Header, auth map[string]string, skipKeys ...string) {
	skip := make(map[string]struct{}, len(skipKeys))
	for _, key := range skipKeys {
		skip[CanonicalHeaderName(key)] = struct{}{}
	}

	for key, value := range auth {
		canonical := CanonicalHeaderName(key)
		if _, omitted := skip[canonical]; omitted {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		h.Set(canonical, value)
	}
}

func shouldSendIcyMetadata(rawURL string, auth map[string]string, opts UpstreamHeaderOptions) bool {
	if strings.TrimSpace(auth["Icy-MetaData"]) != "" {
		return true
	}

	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, ".ts") || strings.Contains(lower, "mp2t") {
		return true
	}
	if opts.MediaSegment && (strings.Contains(lower, "/live/") || strings.Contains(lower, "/hls/")) {
		return true
	}
	return strings.Contains(lower, "icy") || strings.Contains(lower, "shoutcast")
}

func CanonicalHeaderName(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	switch strings.ToLower(key) {
	case "referer", "referrer":
		return "Referer"
	case "user-agent", "useragent":
		return "User-Agent"
	case "origin":
		return "Origin"
	case "cookie":
		return "Cookie"
	case "authorization":
		return "Authorization"
	case "icy-metadata":
		return "Icy-MetaData"
	case "accept-encoding":
		return "Accept-Encoding"
	default:
		return http.CanonicalHeaderKey(key)
	}
}

// ParseAuthHeaderMap parses pipe-delimited auth into a header map.
func ParseAuthHeaderMap(auth string) map[string]string {
	return parseAuthPairs(auth)
}

func applyAuthHeaders(h http.Header, auth string) {
	for key, value := range ParseAuthHeaderMap(auth) {
		if strings.TrimSpace(value) == "" {
			continue
		}
		h.Set(CanonicalHeaderName(key), value)
	}
}

// ContentTypeForURL returns a sensible Content-Type when upstream omits one.
func ContentTypeForURL(rawURL, upstream string) string {
	if ct := strings.TrimSpace(upstream); ct != "" {
		return ct
	}

	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, ".m3u8"), strings.Contains(lower, "mpegurl"):
		return "application/vnd.apple.mpegurl"
	case strings.Contains(lower, ".ts"), strings.Contains(lower, "mp2t"):
		return "video/MP2T"
	case strings.Contains(lower, ".mp4"):
		return "video/mp4"
	case strings.Contains(lower, ".mpd"):
		return "application/dash+xml"
	default:
		return "application/octet-stream"
	}
}
