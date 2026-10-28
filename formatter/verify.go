package formatter

import (
	"fmt"
	"strings"

	"github.com/ysmilda/stformat/internal/lexer"
)

// Verify reports an error unless after is a reformatting of before: the same
// tokens in the same order, where keywords may differ in case and comments in
// whitespace, and every other token is byte-identical. Whitespace between
// tokens, line endings and a leading byte order mark are free to change.
//
// Format runs the same check on its own output, so a formatter bug that drops,
// adds or reorders code makes the file fail instead of being saved. Verify is
// for output produced some other way, and for tests.
func Verify(before, after string) error {
	// A byte order mark at the start of the file is not code, but anywhere
	// else the same bytes are part of a token.
	if bom := strings.HasPrefix(before, utf8BOM); bom == strings.HasPrefix(after, utf8BOM) && bom {
		before, after = before[len(utf8BOM):], after[len(utf8BOM):]
	}
	return compareTokens(lexer.Lex(before), lexer.Lex(after))
}

// compareTokens reports the first difference between the token stream of a
// source and that of its formatted output that Verify does not accept. Both
// streams end with their EOF token.
func compareTokens(a, b []lexer.Token) error {
	for i := range min(len(a), len(b)) {
		switch {
		case sameToken(a[i], b[i]):
		case a[i].Type == lexer.TokenEOF:
			return fmt.Errorf("formatting would add %q; output discarded", b[i].Literal)
		case b[i].Type == lexer.TokenEOF:
			return fmt.Errorf("formatting would drop %q; output discarded", a[i].Literal)
		default:
			return fmt.Errorf("formatting would change %q into %q; output discarded", a[i].Literal, b[i].Literal)
		}
	}
	return nil
}

// sameToken reports whether b is an acceptable rewrite of a.
func sameToken(a, b lexer.Token) bool {
	if a.Type != b.Type {
		return false
	}
	switch {
	case a.Type == lexer.TokenLineComment || a.Type == lexer.TokenBlockComment:
		return withoutSpace(a.Literal) == withoutSpace(b.Literal)
	case lexer.KeywordSpelling(a.Type) != "":
		return strings.EqualFold(a.Literal, b.Literal)
	}
	// A line break inside a literal follows the line ending of the output,
	// so carriage returns do not count.
	return withoutCR(a.Literal) == withoutCR(b.Literal)
}

// withoutCR returns s with every carriage return removed.
func withoutCR(s string) string {
	return strings.ReplaceAll(s, "\r", "")
}

// withoutSpace returns s with all whitespace removed.
func withoutSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}
