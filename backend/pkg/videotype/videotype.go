// Package videotype holds the single table of supported article video formats.
package videotype

import (
	"bytes"
	"strings"
)

// SniffLen is the number of leading bytes callers read for content sniffing.
const SniffLen = 512

const (
	mimeMP4  = "video/mp4"
	mimeWebM = "video/webm"
)

// ebmlMagic opens every Matroska/WebM file.
var ebmlMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}

// mimeByExt maps accepted filename extensions onto their MIME types.
var mimeByExt = map[string]string{
	"mp4":  mimeMP4,
	"webm": mimeWebM,
}

// extByMIME maps supported MIME types onto the canonical key extension.
var extByMIME = map[string]string{
	mimeMP4:  "mp4",
	mimeWebM: "webm",
}

// Sniff reports the video type of data from its leading bytes, or "" when unsupported.
func Sniff(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[4:8]) == "ftyp":
		return mimeMP4
	case bytes.HasPrefix(data, ebmlMagic):
		return mimeWebM
	}

	return ""
}

// MIMEForExt maps a filename extension (case-insensitive) onto its MIME type.
func MIMEForExt(ext string) (string, bool) {
	mime, ok := mimeByExt[strings.ToLower(ext)]

	return mime, ok
}

// ExtForMIME maps a MIME type onto the canonical key extension.
func ExtForMIME(mime string) (string, bool) {
	ext, ok := extByMIME[mime]

	return ext, ok
}
