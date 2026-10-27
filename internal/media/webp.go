package media

import (
	"bytes"
	"io"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

// ConvertWebP re-encodes a WebP image as JPEG, or as PNG if it has transparency, because many email clients cannot render WebP.
func ConvertWebP(r io.Reader) (data []byte, ext, contentType string, err error) {
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, "", "", err
	}

	format, ext, contentType := imaging.PNG, "png", "image/png"
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		format, ext, contentType = imaging.JPEG, "jpg", "image/jpeg"
	}

	var out bytes.Buffer
	if err := imaging.Encode(&out, imaging.Clone(img), format, imaging.JPEGQuality(85)); err != nil {
		return nil, "", "", err
	}

	return out.Bytes(), ext, contentType, nil
}
