package app

import (
	"testing"

	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func TestDetectUploadFormatVideoContainers(t *testing.T) {
	videoCases := []struct {
		name string
		body []byte
		want string
	}{
		{"phone.mp4", []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm'}, "mp4"},
		{"camera.MOV", []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'q', 't', ' ', ' ', 0, 0, 0, 0}, "mov"},
		{"clip.webm", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x93, 0x42, 0x82, 0x88}, "webm"},
	}
	for _, tc := range videoCases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := filesystem.NewFileFromBytes(tc.body, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			format, kind, ok := detectUploadFormat(file)
			if !ok || format != tc.want || kind != "video" {
				t.Fatalf("got (%q, %q, %v), want (%q, video, true)", format, kind, ok, tc.want)
			}
		})
	}
}

func TestDetectUploadFormatRejectsRenamedGarbage(t *testing.T) {
	file, err := filesystem.NewFileFromBytes([]byte("this is not a media file"), "malware.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if format, kind, ok := detectUploadFormat(file); ok {
		t.Fatalf("accepted garbage as %s/%s", kind, format)
	}
}
