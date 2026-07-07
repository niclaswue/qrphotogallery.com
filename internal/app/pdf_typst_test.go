package app

import (
	"os/exec"
	"testing"
)

func TestRenderTypstCards(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	// renderTypstCards uses paths relative to the project root (templates/,
	// data/fonts, pb_data). go test runs in this package directory, so chdir
	// up to the repo root before invoking it.
	t.Chdir("../..")
	job := printJob{
		EventTitle:    "Anna & Marc",
		EventDate:     "2026-09-12",
		AppURL:        "https://example.com",
		AppURLDisplay: "example.com",
		Labels: map[string]string{
			"photo":            "Photo",
			"how_it_works":     "How it works",
			"instruction_lead": "Scan, snap, and add it to the album ",
			"instruction_emph": "instantly",
			"instruction_tail": ".",
			"no":               "No.",
		},
		Design: printDesign{
			ID:         "classic",
			Name:       "Classic White",
			Primary:    "#0a0a0a",
			Secondary:  "#5a5249",
			Accent:     "#c8a26a",
			Background: "#f6f4ee",
			Text:       "#0a0a0a",
		},
		Prompts: []printPrompt{
			{ID: "p1", Text: "Capture the dance floor at its peak", SortOrder: 1, ShortURL: "https://example.com/e/abc/p1"},
			{ID: "p2", Text: "Photograph something old, something new, something borrowed, something blue — get creative with composition!", SortOrder: 2, ShortURL: "https://example.com/e/abc/p2"},
			{ID: "p3", Text: "Snap the table decoration up close", SortOrder: 3, ShortURL: "https://example.com/e/abc/p3"},
			{ID: "p4", Text: "Catch a toast in action", SortOrder: 4, ShortURL: "https://example.com/e/abc/p4"},
			{ID: "p5", Text: "A candid of the hosts", SortOrder: 5, ShortURL: "https://example.com/e/abc/p5"},
			{ID: "p6", Text: "Photograph the venue before guests arrive", SortOrder: 6, ShortURL: "https://example.com/e/abc/p6"},
			{ID: "p7", Text: "A group photo of your table", SortOrder: 7, ShortURL: "https://example.com/e/abc/p7"},
			{ID: "p8", Text: "The best dressed guest", SortOrder: 8, ShortURL: "https://example.com/e/abc/p8"},
			{ID: "p9", Text: "A lovely toast", SortOrder: 9, ShortURL: "https://example.com/e/abc/p9"},
		},
	}
	pdf, err := renderTypstCards("classic", job)
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

func TestRenderTypstPoster(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	t.Chdir("../..")
	d := Designs[0]
	job := printJob{
		EventTitle:    "Anna & Marc",
		EventDate:     "2026-09-12",
		AppURL:        "https://example.com",
		AppURLDisplay: "example.com",
		EventURL:      "https://example.com/e/abc",
		Design: printDesign{
			ID: d.ID, Name: d.Name, Primary: d.Primary, Secondary: d.Secondary,
			Accent: d.Accent, Background: d.Background, Text: d.Text,
		},
		Poster: &posterRender{
			Title:   "Anna & Marc",
			Heading: "Photo Challenge",
			Caption: "Scan the code, get a prompt, snap a photo.",
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

func TestRenderCardPNG(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	t.Chdir("../..")
	for _, side := range []string{"front", "back"} {
		png, err := RenderCardPNG(Designs[0], side, "Anna & Marc",
			"Capture the dance floor at its peak", "en",
			"https://example.com/e/abc/p1", "https://example.com", 144)
		if err != nil {
			t.Fatalf("render %s: %v", side, err)
		}
		if len(png) < 8 || string(png[1:4]) != "PNG" {
			t.Fatalf("%s: not a png, header = %q", side, string(png[:8]))
		}
	}
}
