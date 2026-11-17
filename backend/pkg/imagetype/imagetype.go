// Package imagetype holds the single table of supported cover image formats.
package imagetype

import (
	"net/http"
	"strings"
)

// sniffLen is the number of leading bytes content sniffing considers.
const sniffLen = 512

// mimeByExt maps accepted filename extensions onto their MIME types.
var mimeByExt = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
	"gif":  "image/gif",
}

// extByMIME maps supported MIME types onto the canonical key extension.
var extByMIME = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
}

// Sniff reports the content type of data from its leading bytes.
func Sniff(data []byte) string {
	if len(data) > sniffLen {
		data = data[:sniffLen]
	}

	return http.DetectContentType(data)
}

// MIMEForExt maps a filename extension (case-insensitive) onto its MIME type.
func MIMEForExt(ext string) (string, bool) {
	mime, ok := mimeByExt[strings.ToLower(ext)]

	return mime, ok
}

// ExtForMIME maps a sniffed MIME type onto the canonical key extension.
func ExtForMIME(mime string) (string, bool) {
	ext, ok := extByMIME[mime]

	return ext, ok
}
