package media

import (
	"bytes"
	"image"
	"os"
	"strings"
	"testing"
)

func TestConvertWebP(t *testing.T) {
	cases := []struct {
		file, ext, contentType, format string
	}{
		{"testdata/opaque.webp", "jpg", "image/jpeg", "jpeg"},
		{"testdata/transparent.webp", "png", "image/png", "png"},
	}

	for _, c := range cases {
		f, err := os.Open(c.file)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		data, ext, contentType, err := ConvertWebP(f)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if ext != c.ext || contentType != c.contentType {
			t.Errorf("%s: got %s %s, want %s %s", c.file, ext, contentType, c.ext, c.contentType)
		}

		img, format, err := image.Decode(bytes.NewReader(data))
		if err != nil || format != c.format {
			t.Fatalf("%s: output is not a valid %s: %v", c.file, c.format, err)
		}
		if b := img.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
			t.Errorf("%s: got size %v, want 16x16", c.file, b.Size())
		}
	}
}

func TestConvertWebPInvalid(t *testing.T) {
	if _, _, _, err := ConvertWebP(strings.NewReader("not an image")); err == nil {
		t.Error("expected an error for invalid input")
	}
}
