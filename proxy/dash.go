package proxy

import (
	"bytes"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	dashLocationPattern   = regexp.MustCompile(`(?i)<Location[^>]*>([^<]+)</Location>`)
	dashBaseURLPattern    = regexp.MustCompile(`(?i)(<BaseURL[^>]*>)([^<]+)(</BaseURL>)`)
	dashPeriodOpenPattern = regexp.MustCompile(`(?i)(<Period\b[^>]*>)`)
)

func isDashManifestURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, ".mpd") || strings.Contains(lower, "/manifest.mpd")
}

func isValidDashManifest(body []byte) bool {
	return strings.Contains(string(body), "<MPD")
}

func isDynamicDashManifest(body []byte) bool {
	return strings.Contains(string(body), `type="dynamic"`) ||
		strings.Contains(string(body), `type='dynamic'`)
}

func dashManifestBaseDir(manifestURL string) (string, error) {
	parsed, err := url.Parse(manifestURL)
	if err != nil {
		return "", err
	}

	dir := path.Dir(parsed.Path)
	if dir == "." || dir == "/" {
		dir = ""
	}

	base := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   dir + "/",
	}
	return base.String(), nil
}

func (e *Engine) rewriteDashManifest(body []byte, manifestURL, auth string) ([]byte, error) {
	finalBase, err := dashManifestBaseDir(manifestURL)
	if err != nil {
		return nil, err
	}

	content := string(body)

	content = dashLocationPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := dashLocationPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		resolved, ok := resolveDashReference(manifestURL, strings.TrimSpace(sub[1]))
		if !ok {
			return match
		}
		return strings.Replace(match, sub[1], e.proxyURL(resolved, auth), 1)
	})

	// Relative BaseURL values (e.g. "dash/") resolve against the proxied manifest URL in
	// the browser, sending segment requests to the proxy host instead of the CDN.
	content = dashBaseURLPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := dashBaseURLPattern.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		resolved, ok := resolveDashReference(manifestURL, strings.TrimSpace(sub[2]))
		if !ok {
			return match
		}
		return sub[1] + escapeXML(resolved) + sub[3]
	})

	if !strings.Contains(strings.ToLower(content), "<baseurl") {
		content = dashPeriodOpenPattern.ReplaceAllString(
			content,
			`${1}<BaseURL>`+escapeXML(finalBase)+`</BaseURL>`,
		)
	}

	return []byte(content), nil
}

func resolveDashReference(baseURL, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}

	parsedRef, err := url.Parse(ref)
	if err != nil {
		return "", false
	}

	if parsedRef.IsAbs() {
		return parsedRef.String(), true
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", false
	}

	return base.ResolveReference(parsedRef).String(), true
}

func escapeXML(value string) string {
	var buf bytes.Buffer
	for _, r := range value {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func dashManifestContentType() string {
	return "application/dash+xml"
}

func dashManifestCacheAllowed(body []byte) bool {
	return !isDynamicDashManifest(body)
}
