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

type printPalette struct {
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Text       string `json:"text"`
}

type printJob struct {
	EventTitle    string        `json:"event_title"`
	EventDate     string        `json:"event_date"`
	AppURL        string        `json:"app_url"`
	AppURLDisplay string        `json:"app_url_display"`
	EventURL      string        `json:"event_url"`
	Palette       printPalette  `json:"palette"`
	Poster        *posterRender `json:"poster"`
}

type posterRender struct {
	Title   string `json:"title"`
	Heading string `json:"heading"`
	Caption string `json:"caption"`
	ScanMe  string `json:"scan_me"`
	Footer  string `json:"footer"`
}

func stripURLDisplay(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	return strings.TrimRight(value, "/")
}

func renderTypstPoster(job printJob) ([]byte, error) {
	jobID, err := randomID(8)
	if err != nil {
		return nil, fmt.Errorf("job id: %w", err)
	}
	root := filepath.Join("pb_data", "typst", jobID)
	if err := os.MkdirAll(filepath.Join(root, "qr"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir tmp: %w", err)
	}
	defer os.RemoveAll(root)

	png, err := qrcode.Encode(job.EventURL, qrcode.Medium, 1024)
	if err != nil {
		return nil, fmt.Errorf("qr: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "qr", "event.png"), png, 0o644); err != nil {
		return nil, fmt.Errorf("write qr: %w", err)
	}
	data, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.json"), data, 0o644); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}
	return runTypstCompile(root, filepath.Join("templates", "print", "poster.typ"))
}

func runTypstCompile(root, templatePath string, extraArgs ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"compile",
		"--root", ".",
		"--font-path", filepath.Join("data", "fonts"),
		"--input", "render=" + root,
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

func randomID(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
