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

	if strings.Contains(auth, "|") {
		out := make(map[string]string)
		for _, part := range strings.Split(auth, "|") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key, value, ok := strings.Cut(part, ":")
			if !ok {
				continue
			}
			out[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		return out
	}

	if strings.Contains(auth, ":") {
		key, value, ok := strings.Cut(auth, ":")
		if ok {
			return map[string]string{strings.TrimSpace(key): strings.TrimSpace(value)}
		}
	}

	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return map[string]string{"Authorization": auth}
	}

	return map[string]string{"Authorization": "Bearer " + auth}
}

func formatAuthPairs(pairs map[string]string) string {
	if len(pairs) == 0 {
		return ""
	}

	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}

	// Stable order: session headers first, then anything else sorted.
	ordered := make([]string, 0, len(keys))
	for _, preferred := range upstreamSessionHeaders {
		if value, ok := pairs[preferred]; ok {
			ordered = append(ordered, preferred+":"+value)
		}
	}
	for _, key := range keys {
		if containsHeader(upstreamSessionHeaders, key) {
			continue
		}
		ordered = append(ordered, key+":"+pairs[key])
	}

	return strings.Join(ordered, "|")
}

func containsHeader(headers []string, key string) bool {
	for _, h := range headers {
		if strings.EqualFold(h, key) {
			return true
		}
	}
	return false
}
