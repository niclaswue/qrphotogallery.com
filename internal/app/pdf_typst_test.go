package app

import (
	"os/exec"
	"testing"
)

func TestRenderTypstPoster(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	t.Chdir("../..")
	job := printJob{
		EventTitle:    "Anna & Marc",
		EventDate:     "2026-09-12",
		AppURL:        "https://example.com",
		AppURLDisplay: "example.com",
		EventURL:      "https://example.com/e/abc",
		Palette:       posterPalette,
		Poster: &posterRender{
			Title:   "Anna & Marc",
			Heading: "Share your photos & videos",
			Caption: "Scan the QR code and add your best moments.",
			ScanMe:  "Scan me",
			Footer:  "example.com",
		},
	}
	pdf, err := renderTypstPoster(job)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(pdf) < 1000 {
		t.Fatalf("pdf too small: %d bytes", len(pdf))
	}
	if string(pdf[:4]) != "%PDF" {
		t.Fatalf("not a pdf, header = %q", string(pdf[:8]))
	}
}
