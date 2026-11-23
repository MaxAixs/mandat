package videotype

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSniff_DetectsSupportedVideoTypes pins that ISO BMFF bytes (ftyp at
// offset 4) map onto video/mp4, EBML magic maps onto video/webm, and that
// images, text, empty and truncated inputs are not mistaken for video.
func TestSniff_DetectsSupportedVideoTypes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "mp4 isom",
			data: []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0x00, 0x00, 0x00, 0x00},
			want: "video/mp4",
		},
		{
			name: "mp4 mp42",
			data: []byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00},
			want: "video/mp4",
		},
		{
			name: "webm",
			data: []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81, 0x01},
			want: "video/webm",
		},
		{
			name: "png is not video",
			data: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D},
			want: "",
		},
		{
			name: "jpeg is not video",
			data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01},
			want: "",
		},
		{
			name: "plain text is not video",
			data: []byte("just some plain text, not a video at all"),
			want: "",
		},
		{
			name: "empty input",
			data: []byte{},
			want: "",
		},
		{
			name: "nil input",
			data: nil,
			want: "",
		},
		{
			name: "shorter than ftyp window",
			data: []byte{0x00, 0x00, 0x00},
			want: "",
		},
		{
			name: "shorter than ebml magic",
			data: []byte{0x1A, 0x45, 0xDF},
			want: "",
		},
		{
			name: "short text",
			data: []byte("short"),
			want: "",
		},
		{
			name: "seven bytes are not enough",
			data: []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y'},
			want: "",
		},
		{
			name: "ftyp at offset zero is not mp4",
			data: []byte{'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0x00, 0x00, 0x00, 0x00},
			want: "",
		},
		{
			name: "ftyp at offset five is not mp4",
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		{name: "mp4", ext: "mp4", want: "video/mp4", ok: true},
		{name: "uppercase MP4", ext: "MP4", want: "video/mp4", ok: true},
		{name: "mixed case Mp4", ext: "Mp4", want: "video/mp4", ok: true},
		{name: "webm", ext: "webm", want: "video/webm", ok: true},
		{name: "uppercase WEBM", ext: "WEBM", want: "video/webm", ok: true},
		{name: "mixed case WebM", ext: "WebM", want: "video/webm", ok: true},
		{name: "mov unsupported", ext: "mov", ok: false},
		{name: "avi unsupported", ext: "avi", ok: false},
		{name: "png unsupported", ext: "png", ok: false},
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
// onto the canonical key extension, and that other types are rejected.
func TestExtForMIME_MapsSupportedTypes(t *testing.T) {
	tests := []struct {
		name string
		mime string
		want string
		ok   bool
	}{
		{name: "mp4", mime: "video/mp4", want: "mp4", ok: true},
		{name: "webm", mime: "video/webm", want: "webm", ok: true},
		{name: "quicktime unsupported", mime: "video/quicktime", ok: false},
		{name: "avi unsupported", mime: "video/x-msvideo", ok: false},
		{name: "png unsupported", mime: "image/png", ok: false},
		{name: "plain text unsupported", mime: "text/plain", ok: false},
		{name: "empty unsupported", mime: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtForMIME(tt.mime)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSniffLen_Is512Bytes pins the exported sniff window callers must read
// before calling Sniff.
func TestSniffLen_Is512Bytes(t *testing.T) {
	assert.Equal(t, 512, SniffLen)
}
