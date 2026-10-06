package formatter

import "strings"

// LineEnding reports the line ending src uses: "\r\n" or "\n". The ending
// that occurs most often wins; a file without line breaks, or one that uses
// both equally, reports "\n".
func LineEnding(src string) string {
	var crlf, lf int
	for i := range len(src) {
		if src[i] != '\n' {
			continue
		}
		if i > 0 && src[i-1] == '\r' {
			crlf++
		} else {
			lf++
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

// toLF normalises line endings so the formatter only ever handles "\n".
func toLF(src string) string {
	if !strings.Contains(src, "\r\n") {
		return src
	}
	return strings.ReplaceAll(src, "\r\n", "\n")
}
