package utils

import (
	"net/url"
	"strings"
)

var kodiHeaderNames = []string{
	"Referer=",
	"User-Agent=",
	"Cookie=",
	"Origin=",
	"Authorization=",
}

// ParseStreamSource splits common IPTV source formats:
//   - https://cdn.example.com/live/index.m3u8|x-authorization=token
//   - https://cdn.example.com/live/index.m3u8|Referer=...|x-authorization=token
func ParseStreamSource(source string) (streamURL, auth string) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", ""
	}

	parts := strings.Split(source, "|")
	candidateURL := strings.TrimSpace(parts[0])
	if !isHTTPURL(candidateURL) || len(parts) == 1 {
		cleanedURL, queryAuth := extractHeaderQueryParams(candidateURL)
		if queryAuth != "" {
			return cleanedURL, queryAuth
		}
		return source, ""
	}

	cleanedURL, queryAuth := extractHeaderQueryParams(candidateURL)
	authParts := make([]string, 0, len(parts))
	if queryAuth != "" {
		authParts = append(authParts, queryAuth)
	}

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, segment := range splitAuthSegments(part) {
			if key, value, ok := splitAuthPair(segment); ok {
				authParts = append(authParts, key+":"+value)
			}
		}
	}

	return cleanedURL, strings.Join(authParts, "|")
}

var headerQueryKeys = map[string]string{
	"referer":         "Referer",
	"referrer":        "Referer",
	"http-referrer":   "Referer",
	"http-referer":    "Referer",
	"user-agent":      "User-Agent",
	"useragent":       "User-Agent",
	"cookie":          "Cookie",
	"origin":          "Origin",
	"authorization":   "Authorization",
}

func extractHeaderQueryParams(rawURL string) (string, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" {
		return rawURL, ""
	}

	query := parsed.Query()
	authParts := make([]string, 0)

	for key, values := range query {
		headerName, ok := headerQueryKeys[strings.ToLower(key)]
		if !ok || len(values) == 0 || strings.TrimSpace(values[0]) == "" {
			continue
		}
		authParts = append(authParts, headerName+":"+strings.TrimSpace(values[0]))
		query.Del(key)
	}

	parsed.RawQuery = query.Encode()
	cleaned := parsed.String()
	return cleaned, strings.Join(authParts, "|")
}

// splitAuthSegments splits Kodi-style suffixes without breaking query strings in values.
func splitAuthSegments(part string) []string {
	part = strings.TrimSpace(part)
	if part == "" {
		return nil
	}

	if strings.Contains(part, "|") || !strings.Contains(part, "&") {
		return []string{part}
	}

	indexes := findKodiHeaderSplitIndexes(part)
	if len(indexes) == 0 {
		return []string{part}
	}

	segments := make([]string, 0, len(indexes)+1)
	last := 0
	for _, idx := range indexes {
		if idx > last {
			segments = append(segments, strings.TrimSpace(part[last:idx]))
		}
		last = idx + 1
	}
	if last < len(part) {
		segments = append(segments, strings.TrimSpace(part[last:]))
	}

	return segments
}

func findKodiHeaderSplitIndexes(part string) []int {
	var indexes []int
	searchFrom := 0

	for searchFrom < len(part) {
		nextAmp := strings.Index(part[searchFrom:], "&")
		if nextAmp < 0 {
			break
		}
		idx := searchFrom + nextAmp
		rest := part[idx+1:]
		if isKodiHeaderStart(rest) {
			indexes = append(indexes, idx)
		}
		searchFrom = idx + 1
	}

	return indexes
}

func isKodiHeaderStart(value string) bool {
	for _, prefix := range kodiHeaderNames {
		if len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
			return true
		}
	}
	if len(value) > 2 && (value[0] == 'X' || value[0] == 'x') {
		if eq := strings.IndexByte(value, '='); eq > 1 {
			return true
		}
	}
	return false
}

func splitAuthPair(part string) (key, value string, ok bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", "", false
	}

	colonIndex := strings.Index(part, ":")
	equalsIndex := strings.Index(part, "=")

	if colonIndex > 0 && (equalsIndex < 0 || colonIndex < equalsIndex) {
		key = strings.TrimSpace(part[:colonIndex])
		value = strings.TrimSpace(part[colonIndex+1:])
		if key != "" && value != "" {
			return key, value, true
		}
	}

	// IPTV pipe format uses equals: x-authorization=token, Referer=https://...
	if equalsIndex > 0 {
		key = strings.TrimSpace(part[:equalsIndex])
		value = strings.TrimSpace(part[equalsIndex+1:])
		if key != "" && value != "" && !strings.Contains(key, "://") {
			return key, value, true
		}
	}

	if colonIndex > 0 {
		key = strings.TrimSpace(part[:colonIndex])
		value = strings.TrimSpace(part[colonIndex+1:])
		if key != "" && value != "" {
			return key, value, true
		}
	}

	return "", "", false
}

func isHTTPURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
