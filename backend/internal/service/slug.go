package service

import (
	"strings"
)

// ukToLatin maps Cyrillic letters to their latin transliteration.
var ukToLatin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "h", 'ґ': "g", 'д': "d",
	'е': "e", 'є': "ie", 'ж': "zh", 'з': "z", 'и': "y", 'і': "i",
	'ї': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ь': "", 'ю': "iu", 'я': "ia",
	'ы': "y", 'э': "e", 'ё': "e", 'ъ': "",
}

// slugify converts a title into a URL-safe slug.
// Letters are transliterated, everything else becomes a dash.
func slugify(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if mapped, ok := ukToLatin[r]; ok {
			b.WriteString(mapped)
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}

	slug := collapseDashes(b.String())

	return slug
}

// collapseDashes squeezes dash runs into one and trims the ends.
func collapseDashes(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		if r == '-' {
			if dash {
				continue
			}
			dash = true
		} else {
			dash = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "-")
}
