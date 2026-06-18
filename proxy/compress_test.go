package proxy

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func TestDecompressIfGzip(t *testing.T) {
	plain := []byte("#EXTM3U\n#EXTINF:6.0,\nsegment.ts\n")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(plain); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	got, err := decompressIfGzip(compressed.Bytes(), "")
	if err != nil {
		t.Fatalf("decompressIfGzip() error = %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("expected %q, got %q", plain, got)
	}
}
