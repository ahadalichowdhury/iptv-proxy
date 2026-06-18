package proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

func decompressIfGzip(body []byte, contentEncoding string) ([]byte, error) {
	if !shouldGunzip(body, contentEncoding) {
		return body, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gzip decode: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}

	return decompressed, nil
}

func shouldGunzip(body []byte, contentEncoding string) bool {
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		return true
	}

	return len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b
}
