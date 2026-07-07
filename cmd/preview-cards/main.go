// preview-cards renders a matrix of single-card preview images (one PDF per
// design × language × side) and converts them to WebP for use on the marketing
// site. It is a build-time utility — not part of the running app.
//
// Run from the repo root:
//
//	go run ./cmd/preview-cards
//
// Requires `typst`, `pdftoppm`, and `cwebp` on PATH.
//
// Outputs land under pb_public/static/img/cards/. Filenames follow the pattern
//
//	{design}-{lang}-{front|back}.webp
//
// The landing page picks three of these to display in the print-preview
// section; see views/landing.html.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/niclaswue/template-qr-photo/internal/app"
	"github.com/skip2/go-qrcode"
)

// language defines the localized content for one preview card.
type language struct {
	Code      string
	EventTitle string
	Prompts   []string
	Labels    map[string]string
}

var languages = []language{
	{
		Code:      "en",
		EventTitle: "Anna & Marc",
		Prompts: []string{
			"Capture the dance floor at its peak",
			"A candid of the hosts",
			"The best dressed guest",
		},
		Labels: map[string]string{
			"photo":             "Photo",
			"how_it_works":      "How it works",
			"instruction_lead":  "Scan, snap, and add it to the album ",
			"instruction_emph":  "instantly",
			"instruction_tail":  ".",
			"no":                "No.",
		},
	},
	{
		Code:      "de",
		EventTitle: "Anna & Marc",
		Prompts: []string{
			"Die Tanzfläche auf ihrem Höhepunkt",
			"Ein spontanes Foto der Gastgeber",
			"Der bestangezogene Gast",
		},
		Labels: map[string]string{
			"photo":             "Foto",
			"how_it_works":      "So funktioniert's",
			"instruction_lead":  "Scannen, knipsen — und das Foto landet ",
			"instruction_emph":  "sofort",
			"instruction_tail":  " im Album.",
			"no":                "Nr.",
		},
	},
}

// printPrompt mirrors the JSON shape consumed by templates/cards/classic.typ.
type printPrompt struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	SortOrder int    `json:"sort_order"`
	ShortURL  string `json:"short_url"`
}

type printDesign struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Text       string `json:"text"`
}

type printJob struct {
	EventTitle    string            `json:"event_title"`
	EventDate     string            `json:"event_date"`
	AppURL        string            `json:"app_url"`
	AppURLDisplay string            `json:"app_url_display"`
	Lang          string            `json:"lang"`
	Labels        map[string]string `json:"labels"`
	Design        printDesign       `json:"design"`
	Prompts       []printPrompt     `json:"prompts"`
}

const (
	appURL        = "https://example.com"
	appURLDisplay = "example.com"
	outDir        = "pb_public/static/img/cards"
	// Render at a generous DPI so the cards look crisp when scaled in the
	// browser. 320 DPI on a 91x59mm card gives roughly 1146x744 px.
	renderDPI = "320"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("preview-cards: %v", err)
	}
}

func run() error {
	for _, tool := range []string{"typst", "pdftoppm", "cwebp"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("missing required tool %q on PATH: %w", tool, err)
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	for _, design := range app.Designs {
		for _, lang := range languages {
			for _, side := range []string{"front", "back"} {
				if err := renderOne(design, lang, side); err != nil {
					return fmt.Errorf("%s/%s/%s: %w", design.ID, lang.Code, side, err)
				}
				log.Printf("✓ %s-%s-%s.webp", design.ID, lang.Code, side)
			}
		}
	}
	return nil
}

// renderOne builds a single-prompt typst job, compiles it, then converts the
// resulting PDF to a WebP image. The temp working dir lives under pb_data/ so
// that paths resolve inside the typst --root sandbox.
func renderOne(design app.Design, lang language, side string) error {
	work, err := os.MkdirTemp("pb_data", "preview-")
	if err != nil {
		return fmt.Errorf("tmp dir: %w", err)
	}
	defer os.RemoveAll(work)
	if err := os.MkdirAll(filepath.Join(work, "qr"), 0o755); err != nil {
		return err
	}

	prompts := make([]printPrompt, 0, len(lang.Prompts))
	for i, txt := range lang.Prompts {
		id := fmt.Sprintf("p%d", i+1)
		prompts = append(prompts, printPrompt{
			ID:        id,
			Text:      txt,
			SortOrder: i + 1,
			ShortURL:  fmt.Sprintf("%s/e/preview/%s", appURL, id),
		})
		png, err := qrcode.Encode(prompts[i].ShortURL, qrcode.Medium, 512)
		if err != nil {
			return fmt.Errorf("qr: %w", err)
		}
		if err := os.WriteFile(filepath.Join(work, "qr", id+".png"), png, 0o644); err != nil {
			return err
		}
	}

	job := printJob{
		EventTitle:    lang.EventTitle,
		EventDate:     "2026-09-12",
		AppURL:        appURL,
		AppURLDisplay: appURLDisplay,
		Lang:          lang.Code,
		Labels:        lang.Labels,
		Design: printDesign{
			ID:         design.ID,
			Name:       design.Name,
			Primary:    design.Primary,
			Secondary:  design.Secondary,
			Accent:     design.Accent,
			Background: design.Background,
			Text:       design.Text,
		},
		Prompts: prompts,
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "data.json"), data, 0o644); err != nil {
		return err
	}

	templatePath := filepath.Join("templates", "print", design.ID+".typ")
	if _, err := os.Stat(templatePath); err != nil {
		templatePath = filepath.Join("templates", "print", "classic.typ")
	}

	pdfPath := filepath.Join(work, "card.pdf")
	mode := "preview-" + side
	cmd := exec.Command(
		"typst", "compile",
		"--root", ".",
		"--font-path", filepath.Join("data", "fonts"),
		"--input", "render="+work,
		"--input", "mode="+mode,
		templatePath, pdfPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("typst: %w", err)
	}

	// pdftoppm writes "<prefix>-1.png"; pick page 1.
	pngPrefix := filepath.Join(work, "card")
	cmd = exec.Command("pdftoppm", "-r", renderDPI, "-png", pdfPath, pngPrefix)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdftoppm: %w", err)
	}

	pngPath := pngPrefix + "-1.png"
	webpPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.webp", design.ID, lang.Code, side))
	cmd = exec.Command("cwebp", "-quiet", "-q", "90", pngPath, "-o", webpPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cwebp: %w", err)
	}
	return nil
}
