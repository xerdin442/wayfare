package storage

import (
	"bytes"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/xerdin442/wayfare/shared/util"
)

// newFileHeader builds a real *multipart.FileHeader by encoding and re-parsing
// a multipart form, the same way gin does for an upload
func newFileHeader(t *testing.T, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "upload")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	w.Close()

	form, err := multipart.NewReader(&body, w.Boundary()).ReadForm(64 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return form.File["file"][0]
}

var (
	pngHeader  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	heicHeader = []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00")
)

func TestValidateImage_AcceptsSupportedTypes(t *testing.T) {
	cases := map[string][]byte{
		"png":  pngHeader,
		"jpeg": jpegHeader,
		"heic": heicHeader,
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateImage(newFileHeader(t, content)); err != nil {
				t.Fatalf("expected %s to be accepted, got %v", name, err)
			}
		})
	}
}

func TestValidateImage_RejectsUnsupportedType(t *testing.T) {
	err := ValidateImage(newFileHeader(t, []byte("%PDF-1.7 not an image")))
	if !errors.Is(err, util.ErrUnsupportedFileType) {
		t.Fatalf("expected ErrUnsupportedFileType, got %v", err)
	}
}

func TestValidateImage_RejectsOversizedFile(t *testing.T) {
	content := append(bytes.Clone(pngHeader), make([]byte, MaxImageSize)...)

	err := ValidateImage(newFileHeader(t, content))
	if !errors.Is(err, util.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestParseImageMimetype_RewindsFile(t *testing.T) {
	file, err := newFileHeader(t, pngHeader).Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := parseImageMimetype(file); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The upload must send the whole file, starting from the first byte
	first := make([]byte, len(pngHeader))
	if _, err := file.Read(first); err != nil || !bytes.Equal(first, pngHeader) {
		t.Fatalf("expected the file to be rewound to the start, read %q (err %v)", first, err)
	}
}
