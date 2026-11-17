package imagetype

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSniff_DetectsSupportedImageTypes pins that leading magic bytes map onto
// the content types the cover pipeline supports, and that plain text is not
// mistaken for an image.
func TestSniff_DetectsSupportedImageTypes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "jpeg",
			data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01},
			want: "image/jpeg",
		},
		{
			name: "png",
			data: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D},
			want: "image/png",
		},
		{
			name: "gif89a",
			data: []byte{'G', 'I', 'F', '8', '9', 'a', 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			want: "image/gif",
		},
		{
			name: "gif87a",
			data: []byte{'G', 'I', 'F', '8', '7', 'a', 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			want: "image/gif",
		},
		{
			name: "webp",
			data: []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '},
			want: "image/webp",
		},
		{
			name: "plain text",
			data: []byte("just some plain text, not an image at all"),
			want: "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Sniff(tt.data))
		})
	}
}

// TestSniff_OnlyFirst512BytesMatter pins the DetectContentType contract: bytes
// past the 512-byte window must not change the result.
func TestSniff_OnlyFirst512BytesMatter(t *testing.T) {
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "trailing text after an image header is ignored",
			data: append(append([]byte{}, jpegHeader...), bytes.Repeat([]byte("x"), 1000)...),
			want: "image/jpeg",
		},
		{
			name: "image magic past the window is not detected",
			data: append(bytes.Repeat([]byte("a"), 600), jpegHeader...),
			want: "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Greater(t, len(tt.data), 512, "the fixture must exceed the sniff window")
			assert.Equal(t, tt.want, Sniff(tt.data))
		})
	}
}

// TestMIMEForExt_MapsSupportedExtensions pins the case-insensitive extension
// table and that unsupported or empty extensions are rejected.
func TestMIMEForExt_MapsSupportedExtensions(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want string
		ok   bool
	}{
		{name: "jpg", ext: "jpg", want: "image/jpeg", ok: true},
		{name: "uppercase JPG", ext: "JPG", want: "image/jpeg", ok: true},
		{name: "jpeg", ext: "jpeg", want: "image/jpeg", ok: true},
		{name: "mixed case Jpeg", ext: "Jpeg", want: "image/jpeg", ok: true},
		{name: "png", ext: "png", want: "image/png", ok: true},
		{name: "uppercase PNG", ext: "PNG", want: "image/png", ok: true},
		{name: "webp", ext: "webp", want: "image/webp", ok: true},
		{name: "uppercase WEBP", ext: "WEBP", want: "image/webp", ok: true},
		{name: "gif", ext: "gif", want: "image/gif", ok: true},
		{name: "uppercase GIF", ext: "GIF", want: "image/gif", ok: true},
		{name: "svg unsupported", ext: "svg", ok: false},
		{name: "bmp unsupported", ext: "bmp", ok: false},
		{name: "empty unsupported", ext: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MIMEForExt(tt.ext)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtForMIME_MapsSupportedTypes pins the reverse table from sniffed MIME
// onto the canonical key extension, and that non-image types are rejected.
func TestExtForMIME_MapsSupportedTypes(t *testing.T) {
	tests := []struct {
		name string
		mime string
		want string
		ok   bool
	}{
		{name: "jpeg", mime: "image/jpeg", want: "jpg", ok: true},
		{name: "png", mime: "image/png", want: "png", ok: true},
		{name: "webp", mime: "image/webp", want: "webp", ok: true},
		{name: "gif", mime: "image/gif", want: "gif", ok: true},
		{name: "svg unsupported", mime: "image/svg+xml", ok: false},
		{name: "plain text unsupported", mime: "text/plain", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtForMIME(tt.mime)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
