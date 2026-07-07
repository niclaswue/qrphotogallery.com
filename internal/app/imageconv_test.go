package app

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"os"
	"testing"
)

// TestHeicToJPEG decodes a real HEIC sample (an iPhone-style HEVC still) and
// asserts we get back a valid, non-trivial JPEG. This is the core of the HEIC
// upload feature; if the pure-Go decoder ever regresses under CGO_ENABLED=0
// this test catches it.
func TestHeicToJPEG(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample.heic")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	out, err := heicToJPEG(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("heicToJPEG: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("heicToJPEG returned empty output")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not a decodable image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg output, got %q", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		t.Fatalf("decoded image has bad dimensions: %dx%d", cfg.Width, cfg.Height)
	}
}

func TestHeicToJPEGRejectsNonHeic(t *testing.T) {
	if _, err := heicToJPEG(bytes.NewReader([]byte("not an image"))); err == nil {
		t.Fatal("expected error decoding garbage input, got nil")
	}
}

func TestDisplayFilename(t *testing.T) {
	cases := map[string]string{
		"photo.heic":    "photo.jpg",
		"photo.HEIC":    "photo.jpg",
		"IMG_1234.heif": "IMG_1234.jpg",
		"no-extension":  "no-extension.jpg",
		"a.b.c.heic":    "a.b.c.jpg",
	}
	for in, want := range cases {
		if got := displayFilename(in); got != want {
			t.Errorf("displayFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
