package utils

import (
	"strings"
	"testing"
	"time"
)

func TestEncryptDecryptPlayToken(t *testing.T) {
	secret := "test-play-token-secret-at-least-32-characters-long"
	target := "https://cdn.example.com/live/main.m3u8"
	auth := "Referer:https://portal.example/|Authorization:Bearer abc"

	token, err := EncryptPlayToken(secret, target, auth, time.Hour)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if strings.Contains(token, "cdn.example.com") {
		t.Fatalf("token must not contain upstream host, got %q", token)
	}

	gotURL, gotAuth, err := DecryptPlayToken(secret, token)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if gotURL != target {
		t.Fatalf("url mismatch: got %q want %q", gotURL, target)
	}
	if gotAuth != auth {
		t.Fatalf("auth mismatch: got %q want %q", gotAuth, auth)
	}
}

func TestDecryptPlayTokenExpired(t *testing.T) {
	secret := "test-play-token-secret-at-least-32-characters-long"
	token, err := encryptPlayTokenPayload(secret, playTokenPayload{
		U: "https://cdn.example.com/a.m3u8",
		E: time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, _, err = DecryptPlayToken(secret, token)
	if err != ErrPlayTokenExpired {
		t.Fatalf("expected ErrPlayTokenExpired, got %v", err)
	}
}
