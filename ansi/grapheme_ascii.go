package ansi

import "unicode/utf8"

// asciiMayStartGrapheme keeps the last ASCII byte with any following Unicode
// combining marks. Printable ASCII with an ASCII successor retains the byte
// fast path; only the boundary before Unicode needs grapheme segmentation.
func asciiMayStartGrapheme[T string | []byte](s T, i int) bool {
	return i+1 < len(s) && s[i+1] >= utf8.RuneSelf
}
