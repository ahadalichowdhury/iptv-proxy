package utils

import (
	"net/http"
	"net/url"
	"strings"
)

// BrowserUserAgent is sent to upstream sources that expect a browser client.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// HeadersForURL builds upstream request headers from the target URL and optional auth token.
// auth may be a bare token (Authorization: Bearer ...) or "Key: Value" pairs separated by "|".
func HeadersForURL(rawURL, auth string) http.Header {
	h := make(http.Header)

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return h
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}

	host := parsed.Hostname()
	origin := scheme + "://" + host
	if port := parsed.Port(); port != "" {
		origin = scheme + "://" + host + ":" + port
	}

	referer := origin + "/"
	if parsed.Path != "" && parsed.Path != "/" {
		referer = scheme + "://" + parsed.Host + parsed.Path
	}

	h.Set("User-Agent", BrowserUserAgent)
	h.Set("Origin", origin)
	h.Set("Referer", referer)
	h.Set("Accept", "*/*")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Connection", "keep-alive")

	applyAuthHeaders(h, auth)
	applyDomainOverrides(h, host, origin, referer)

	return h
}

func applyAuthHeaders(h http.Header, auth string) {
	auth = strings.TrimSpace(auth)
	if auth == "" {
		return
	}

	if strings.Contains(auth, "|") {
		for _, part := range strings.Split(auth, "|") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key, value, ok := strings.Cut(part, ":")
			if !ok {
				continue
			}
			h.Set(strings.TrimSpace(key), strings.TrimSpace(value))
		}
		return
	}

	if key, value, ok := strings.Cut(auth, ":"); ok {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			h.Set(key, value)
			return
		}
	}

	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		h.Set("Authorization", auth)
		return
	}

	h.Set("Authorization", "Bearer "+auth)
}

func applyDomainOverrides(h http.Header, host, origin, referer string) {
	host = strings.ToLower(host)

	switch {
	case strings.Contains(host, "youtube.com"), strings.Contains(host, "googlevideo.com"):
		h.Set("Origin", "https://www.youtube.com")
		h.Set("Referer", "https://www.youtube.com/")

	case strings.Contains(host, "twitch.tv"), strings.Contains(host, "ttvnw.net"):
		h.Set("Origin", "https://www.twitch.tv")
		h.Set("Referer", "https://www.twitch.tv/")

	case strings.Contains(host, "dailymotion.com"), strings.Contains(host, "dmcdn.net"):
		h.Set("Origin", "https://www.dailymotion.com")
		h.Set("Referer", "https://www.dailymotion.com/")

	case strings.Contains(host, "facebook.com"), strings.Contains(host, "fbcdn.net"):
		h.Set("Origin", "https://www.facebook.com")
		h.Set("Referer", "https://www.facebook.com/")

	default:
		// Keep computed origin/referer for generic IPTV/CDN hosts.
		h.Set("Origin", origin)
		h.Set("Referer", referer)
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
