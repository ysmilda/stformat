package formatter

import "strings"

// LineEnding reports the line ending src uses: "\r\n" or "\n". The ending
// that occurs most often wins; a file without line breaks, or one that uses
// both equally, reports "\n".
func LineEnding(src string) string {
	// Counting the substrings delegates to the vectorised bytealg search
	// instead of a byte-at-a-time loop over the whole file.
	crlf := strings.Count(src, "\r\n")
	lf := strings.Count(src, "\n") - crlf
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
