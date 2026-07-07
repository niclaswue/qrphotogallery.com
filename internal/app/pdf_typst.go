package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/skip2/go-qrcode"
)

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
	EventURL      string            `json:"event_url,omitempty"`
	Design        printDesign       `json:"design"`
	Prompts       []printPrompt     `json:"prompts"`
	Labels        map[string]string `json:"labels,omitempty"`
	Poster        *posterRender     `json:"poster,omitempty"`
}

// posterRender is the JSON contract consumed by templates/print/poster.typ:
// the final text content for the single-QR poster (colours come from
// printJob.Design). Built by buildPosterRender; the texts are already
// localized.
type posterRender struct {
	Title   string `json:"title"`   // event title, largest line
	Heading string `json:"heading"` // localized poster headline
	Caption string `json:"caption"` // localized instruction line
	ScanMe  string `json:"scan_me"` // small label next to the QR
	Footer  string `json:"footer"`  // app URL attribution
}

// stripURLDisplay drops the protocol and trailing slash so a URL prints
// cleanly in card footers, e.g. "https://example.com/" -> "example.com".
func stripURLDisplay(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimRight(u, "/")
}

// renderTypstCards writes data + QR PNGs into a temp dir under pb_data/typst,
// invokes the typst compiler with the given template, and returns the output
// bytes (a PDF unless extraArgs switch the format). The temp dir is removed on
// return (success or error).
func renderTypstCards(templateID string, job printJob, extraArgs ...string) ([]byte, error) {
	jobID, err := randomID(8)
	if err != nil {
		return nil, fmt.Errorf("job id: %w", err)
	}
	relRoot := filepath.Join("pb_data", "typst", jobID)
	if err := os.MkdirAll(filepath.Join(relRoot, "qr"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir tmp: %w", err)
	}
	defer os.RemoveAll(relRoot)

	for _, p := range job.Prompts {
		png, err := qrcode.Encode(p.ShortURL, qrcode.Medium, 512)
		if err != nil {
			return nil, fmt.Errorf("qr %s: %w", p.ID, err)
		}
		qrPath := filepath.Join(relRoot, "qr", p.ID+".png")
		if err := os.WriteFile(qrPath, png, 0o644); err != nil {
			return nil, fmt.Errorf("write qr: %w", err)
		}
	}

	dataPath := filepath.Join(relRoot, "data.json")
	dataBytes, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	if err := os.WriteFile(dataPath, dataBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}

	templatePath := filepath.Join("templates", "print", templateID+".typ")
	if _, err := os.Stat(templatePath); err != nil {
		// Fall back to classic if a per-design template doesn't exist yet.
		templatePath = filepath.Join("templates", "print", "classic.typ")
	}

	return runTypstCompile(relRoot, templatePath, extraArgs...)
}

// renderTypstPoster renders the single-QR poster PDF: one page with a large
// QR code that points at the event dispatcher (job.EventURL), themed by
// job.Design. The temp dir is removed on return.
func renderTypstPoster(job printJob) ([]byte, error) {
	jobID, err := randomID(8)
	if err != nil {
		return nil, fmt.Errorf("job id: %w", err)
	}
	relRoot := filepath.Join("pb_data", "typst", jobID)
	if err := os.MkdirAll(filepath.Join(relRoot, "qr"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir tmp: %w", err)
	}
	defer os.RemoveAll(relRoot)

	png, err := qrcode.Encode(job.EventURL, qrcode.Medium, 1024)
	if err != nil {
		return nil, fmt.Errorf("qr: %w", err)
	}
	if err := os.WriteFile(filepath.Join(relRoot, "qr", "event.png"), png, 0o644); err != nil {
		return nil, fmt.Errorf("write qr: %w", err)
	}

	dataBytes, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(relRoot, "data.json"), dataBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}

	return runTypstCompile(relRoot, filepath.Join("templates", "print", "poster.typ"))
}

// runTypstCompile shells out to `typst compile` for a prepared job directory
// and returns the output bytes (PDF by default; extraArgs can switch the
// format). Shared by the per-prompt cards and the poster.
func runTypstCompile(relRoot, templatePath string, extraArgs ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"compile",
		"--root", ".",
		"--font-path", filepath.Join("data", "fonts"),
		"--input", "render=" + relRoot,
	}
	args = append(args, extraArgs...)
	args = append(args, templatePath, "-")
	cmd := exec.Command("typst", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("typst: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
