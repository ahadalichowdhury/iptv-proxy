package utils

import (
	"net/http"
	"strings"
)

// Upstream session headers commonly required by Flussonic/Streamer and similar CDNs.
var upstreamSessionHeaders = []string{
	"X-Sid",
	"X-Vsaas-Session",
	"Session",
	"X-Originator",
}

// SessionAuthFromHeaders builds an auth query value from upstream session response headers.
func SessionAuthFromHeaders(h http.Header) string {
	if h == nil {
		return ""
	}

	pairs := make([]string, 0, len(upstreamSessionHeaders))
	for _, key := range upstreamSessionHeaders {
		if v := strings.TrimSpace(h.Get(key)); v != "" {
			pairs = append(pairs, key+":"+v)
		}
	}

	return strings.Join(pairs, "|")
}

// MergeAuth combines auth from the client query param with session headers from upstream.
// Upstream session values override duplicate keys so refreshed X-Sid wins.
func MergeAuth(existing, fromUpstream string) string {
	merged := parseAuthPairs(existing)
	for key, value := range parseAuthPairs(fromUpstream) {
		merged[key] = value
	}
	return formatAuthPairs(merged)
}

func parseAuthPairs(auth string) map[string]string {
	auth = strings.TrimSpace(auth)
	if auth == "" {
		return map[string]string{}
	}

	out := make(map[string]string)

	appendPair := func(part string) {
		part = strings.TrimSpace(part)
		if part == "" {
			return
		}
		for _, segment := range splitAuthSegments(part) {
			key, value, ok := splitAuthPair(segment)
			if !ok {
				continue
			}
			out[CanonicalHeaderName(key)] = value
		}
	}

	if strings.Contains(auth, "|") {
		for _, part := range strings.Split(auth, "|") {
			appendPair(part)
		}
		return out
	}

	appendPair(auth)

	if len(out) == 0 {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return map[string]string{"Authorization": auth}
		}
		return map[string]string{"Authorization": "Bearer " + auth}
	}

	return out
}

func formatAuthPairs(pairs map[string]string) string {
	if len(pairs) == 0 {
		return ""
	}

	preferred := []string{
		"Referer",
		"Origin",
		"User-Agent",
		"Authorization",
		"Cookie",
	}

	ordered := make([]string, 0, len(pairs))
	seen := make(map[string]bool, len(pairs))

	for _, preferredKey := range append(preferred, upstreamSessionHeaders...) {
		canonical := CanonicalHeaderName(preferredKey)
		if value, ok := pairs[canonical]; ok && !seen[canonical] {
			ordered = append(ordered, canonical+":"+value)
			seen[canonical] = true
		}
	}

	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, key := range keys {
		ordered = append(ordered, key+":"+pairs[key])
	}

	return strings.Join(ordered, "|")
}
