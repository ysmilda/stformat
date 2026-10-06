package formatter

import (
	"strings"

	"github.com/ysmilda/stformat/internal/lexer"
)

// Formatting is switched off with a plain comment directive:
//
//	// stformat:ignore          leave the whole file untouched
//	// stformat:off             start an ignored section
//	// stformat:on              end the ignored section
//
// A directive may stand on its own line or trail code on a line. Inside an
// ignored section the source text is copied verbatim: neither the token pass
// nor the line-level post-processing may touch it. Directives themselves are
// never rewritten, so they keep their original spelling and case.
//
// "stformat:ignore" applies to the whole file, so it only counts in the
// comment block at the top of the file: once code has been seen it is an
// ordinary comment. That also makes it detectable from a stream, where the
// head of the input decides whether the input is parsed at all.
const (
	directiveOff    = "off"
	directiveOn     = "on"
	directiveIgnore = "ignore"

	directiveName = "stformat"
)

// tokenRun is a half-open range of token indexes [start, end) that is emitted
// verbatim instead of being formatted.
type tokenRun struct {
	start int
	end   int
}

// unterminated marks a run whose "stformat:on" has not been seen yet. It is
// negative so that it cannot be confused with a real token index.
const unterminated = -1

// parseDirective returns the directive carried by a comment literal, or "" for
// an ordinary comment. The name is matched case insensitively and may be
// written with spaces around the colon.
func parseDirective(lit string) string {
	body := strings.TrimSpace(lit)
	switch {
	case strings.HasPrefix(body, "//"):
		body = body[2:]
	case strings.HasPrefix(body, "(*"):
		body = strings.TrimSuffix(strings.TrimSpace(body[2:]), "*)")
	}
	i := strings.IndexByte(body, ':')
	if i < 0 {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(body[:i]), directiveName) {
		return ""
	}
	switch strings.TrimSpace(strings.ToLower(body[i+1:])) {
	case directiveOff:
		return directiveOff
	case directiveOn:
		return directiveOn
	case directiveIgnore:
		return directiveIgnore
	}
	return ""
}

// lineDirective returns the directive found in a formatted line, or "" when the
// line carries none. Both line comments and block comments are recognised, and
// a "//" or "(*" inside a string literal is not a comment.
func lineDirective(line string) string {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'', '"':
			i = skipQuoted(line, i) - 1
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				return parseDirective(line[i:])
			}
		case '(':
			if i+1 < len(line) && line[i+1] == '*' {
				end := strings.Index(line[i:], "*)")
				if end < 0 {
					return ""
				}
				if d := parseDirective(line[i : i+end+2]); d != "" {
					return d
				}
			}
		}
	}
	return ""
}

// scanDirectives finds the ignored sections in a token stream. A file-level
// "stformat:ignore" directive is reported through all, in which case runs must
// not be used. Only a directive in the comment block at the top of the stream
// counts as file-level; further down it is an ordinary comment.
func scanDirectives(tokens []lexer.Token) (all bool, runs []tokenRun) {
	leading := true
	for i, tok := range tokens {
		if tok.Type != lexer.TokenLineComment && tok.Type != lexer.TokenBlockComment {
			leading = false
			continue
		}
		switch parseDirective(tok.Literal) {
		case directiveIgnore:
			if leading {
				return true, nil
			}
		case directiveOff:
			runs = append(runs, tokenRun{start: i, end: unterminated})
		case directiveOn:
			if n := len(runs); n > 0 {
				runs[n-1].end = i
			}
		}
	}
	// An "off" without a matching "on" ignores the rest of the file.
	for i := range runs {
		if runs[i].end == unterminated {
			runs[i].end = len(tokens)
		}
	}
	return false, runs
}
