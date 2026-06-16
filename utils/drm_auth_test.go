package utils

import "testing"

func TestStripDrmAuth(t *testing.T) {
	auth := "Referer:https://example.com/|X-Drm-Scheme:widevine|X-Drm-License-Url:https://license.example.com/|User-Agent:Dalvik"
	stripped := StripDrmAuth(auth)

	if stripped != "Referer:https://example.com/|User-Agent:Dalvik" {
		t.Fatalf("StripDrmAuth = %q", stripped)
	}
}

func TestStripDrmAuthEmpty(t *testing.T) {
	if StripDrmAuth("") != "" {
		t.Fatal("expected empty")
	}
}
