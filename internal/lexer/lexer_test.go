package lexer

import (
	"testing"
)

// TestCharClassMatchesRawPredicates guards the table that the lexer's
// per-byte tests read against the plain range checks it was built from.
func TestCharClassMatchesRawPredicates(t *testing.T) {
	t.Parallel()
	for ch := range 256 {
		b := byte(ch)
		if got, want := charClass[ch]&classIdentStart != 0, isIdentStartRaw(b); got != want {
			t.Errorf("classIdentStart(%q) = %v, want %v", b, got, want)
		}
		if got, want := charClass[ch]&classIdentPart != 0, isIdentStartRaw(b) || isDigitRaw(b); got != want {
			t.Errorf("classIdentPart(%q) = %v, want %v", b, got, want)
		}
		if got, want := charClass[ch]&classDigit != 0, isDigitRaw(b); got != want {
			t.Errorf("classDigit(%q) = %v, want %v", b, got, want)
		}
		if got, want := charClass[ch]&classHexDigit != 0, isHexDigitRaw(b); got != want {
			t.Errorf("classHexDigit(%q) = %v, want %v", b, got, want)
		}
		wantSpace := b == ' ' || b == '\t' || b == '\r' || b == '\n'
		if got := charClass[ch]&classSpace != 0; got != wantSpace {
			t.Errorf("classSpace(%q) = %v, want %v", b, got, wantSpace)
		}
	}
}

// TestKeywordTable checks the open-addressed table the lexer probes against
// the two maps it is built from. The table is derived data, so a keyword added
// to a map has to show up in lookups.
func TestKeywordTable(t *testing.T) {
	t.Parallel()
	for k, want := range Keywords {
		tt, scalar, found := lookupKeyword([]byte(k))
		if !found {
			t.Errorf("lookupKeyword(%q) not found", k)
			continue
		}
		if tt != want {
			t.Errorf("lookupKeyword(%q) = %s, want %s", k, TokenName(tt), TokenName(want))
		}
		if _, isScalar := ScalarTypeNames[k]; isScalar != scalar {
			t.Errorf("lookupKeyword(%q) scalar = %v, want %v", k, scalar, isScalar)
		}
	}
	for k := range ScalarTypeNames {
		if _, _, found := lookupKeyword([]byte(k)); !found {
			t.Errorf("type name %q not found in the keyword table", k)
		}
	}
	for _, name := range []string{"", "x", "ANDX", "AND_", "_AND", "NOTAKEYWORD", "flags"} {
		if _, _, found := lookupKeyword([]byte(name)); found {
			t.Errorf("lookupKeyword(%q) unexpectedly found", name)
		}
	}
}

// TestMaxKeywordLenCoversTable guards the length the lexer uses to skip the
// uppercase copy: an identifier longer than this cannot be in either map.
func TestMaxKeywordLenCoversTable(t *testing.T) {
	t.Parallel()
	longest := 0
	for k := range Keywords {
		longest = max(longest, len(k))
	}
	for k := range ScalarTypeNames {
		longest = max(longest, len(k))
	}
	if maxKeywordLen < longest {
		t.Errorf("maxKeywordLen = %d, but the longest name is %d", maxKeywordLen, longest)
	}
}

// TestLexOffsets checks the field the formatter relies on: every token reports
// the byte index it starts at, so a token range can be copied from the source
// verbatim.
func TestLexOffsets(t *testing.T) {
	t.Parallel()
	const src = "PROGRAM p\nVAR\n\tx : INT; // note\nEND_VAR\nEND_PROGRAM"
	toks := Lex(src)
	for i, tok := range toks {
		if tok.Offset > len(src) {
			t.Fatalf("token %d offset %d past end of source (%d)", i, tok.Offset, len(src))
		}
		if tok.Literal != "" && src[tok.Offset:][:len(tok.Literal)] != tok.Literal {
			t.Errorf("token %d literal %q does not match source at offset %d",
				i, tok.Literal, tok.Offset)
		}
		if tok.Type != TokenEOF && i > 0 && tok.Offset < toks[i-1].Offset {
			t.Errorf("token %d offset %d is before the previous token's %d",
				i, tok.Offset, toks[i-1].Offset)
		}
	}
}
