package utils

import "strings"

// StripDrmAuth removes X-Drm-* metadata from auth before upstream CDN requests.
// DRM fields are consumed by the browser player, not forwarded to manifest/segment hosts.
func StripDrmAuth(auth string) string {
	pairs := parseAuthPairs(auth)
	if len(pairs) == 0 {
		return auth
	}

	filtered := make(map[string]string, len(pairs))
	for key, value := range pairs {
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.HasPrefix(lower, "x-drm-") {
			continue
		}
		filtered[key] = value
	}

	return formatAuthPairs(filtered)
}
