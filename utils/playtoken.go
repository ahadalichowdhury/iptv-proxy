package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const defaultPlayTokenTTL = 4 * time.Hour

type playTokenPayload struct {
	U string `json:"u"`
	A string `json:"a"`
	E int64  `json:"e"`
}

var (
	ErrPlayTokenDisabled = errors.New("play token secret is not configured")
	ErrPlayTokenInvalid  = errors.New("invalid play token")
	ErrPlayTokenExpired  = errors.New("play token expired")
)

func PlayTokenKey(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if len(trimmed) < 32 {
		return nil, ErrPlayTokenDisabled
	}
	sum := sha256.Sum256([]byte(trimmed))
	return sum[:], nil
}

func encryptPlayTokenPayload(secret string, payload playTokenPayload) (string, error) {
	key, err := PlayTokenKey(secret)
	if err != nil {
		return "", err
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nil, nonce, raw, nil)
	out := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func EncryptPlayToken(secret, targetURL, auth string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = defaultPlayTokenTTL
	}

	return encryptPlayTokenPayload(secret, playTokenPayload{
		U: strings.TrimSpace(targetURL),
		A: auth,
		E: time.Now().Add(ttl).Unix(),
	})
}

func DecryptPlayToken(secret, token string) (targetURL, auth string, err error) {
	key, err := PlayTokenKey(secret)
	if err != nil {
		return "", "", err
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return "", "", ErrPlayTokenInvalid
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) <= nonceSize {
		return "", "", ErrPlayTokenInvalid
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", "", ErrPlayTokenInvalid
	}

	var payload playTokenPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return "", "", ErrPlayTokenInvalid
	}

	if payload.U == "" {
		return "", "", ErrPlayTokenInvalid
	}

	if payload.E > 0 && time.Now().Unix() > payload.E {
		return "", "", ErrPlayTokenExpired
	}

	return payload.U, payload.A, nil
}
