package lexer

import (
	"strings"
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

// TestKeywordTable checks what is derived from the keywords table: every
// spelling is found by the lexer's lookup, every keyword token type has a
// canonical spelling, and non-keywords are not found.
func TestKeywordTable(t *testing.T) {
	t.Parallel()
	for _, k := range keywords {
		tt, found := lookupKeyword([]byte(k.spelling))
		if !found || tt != k.tt {
			t.Errorf("lookupKeyword(%q) = %s, %v; want %s", k.spelling, tt, found, k.tt)
		}
		if KeywordSpelling(k.tt) == "" {
			t.Errorf("KeywordSpelling(%s) is empty", k.tt)
		}
	}
	for tt := TokenProgram; tt <= TokenWcharType; tt++ {
		if KeywordSpelling(tt) == "" {
			t.Errorf("keyword token type %d has no spelling in the table", int(tt))
		}
	}
	for _, name := range []string{"", "x", "ANDX", "AND_", "_AND", "NOTAKEYWORD", "flags", "D", "LD"} {
		if _, found := lookupKeyword([]byte(name)); found {
			t.Errorf("lookupKeyword(%q) unexpectedly found", name)
		}
	}
}

// TestTokenTypeString checks that every token type has a name of its own.
func TestTokenTypeString(t *testing.T) {
	t.Parallel()
	seen := make(map[string]TokenType)
	for tt := TokenEOF; tt < tokenCount; tt++ {
		s := tt.String()
		if strings.HasPrefix(s, "TokenType(") {
			t.Errorf("token type %d has no name", int(tt))
		}
		// Aliased keywords share a type, never a name across types.
		if prev, dup := seen[s]; dup {
			t.Errorf("token types %d and %d are both named %q", int(prev), int(tt), s)
		}
		seen[s] = tt
	}
}

// TestMaxKeywordLenCoversTable guards the length the lexer uses to skip the
// uppercase copy: an identifier longer than this cannot be in the table.
func TestMaxKeywordLenCoversTable(t *testing.T) {
	t.Parallel()
	longest := 0
	for _, k := range keywords {
		longest = max(longest, len(k.spelling))
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

// TestLexTokenTypes pins the token types of inputs where the meaning depends
// on context: set/reset operators, keyword-spelled members and characters that
// start no token.
func TestLexTokenTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []TokenType
	}{
		{name: "set after target", in: "x S= y", want: []TokenType{TokenIdent, TokenSAssign, TokenIdent}},
		{name: "reset after index", in: "a[1] R= y", want: []TokenType{TokenIdent, TokenLBracket, TokenNumber, TokenRBracket, TokenRAssign, TokenIdent}},
		{name: "ref after deref", in: "p^ REF= y", want: []TokenType{TokenIdent, TokenCaret, TokenRefAssign, TokenIdent}},
		{name: "s compared after IF", in: "IF s=1", want: []TokenType{TokenIf, TokenIdent, TokenEqual, TokenNumber}},
		{name: "r compared after AND", in: "a AND r=2", want: []TokenType{TokenIdent, TokenAnd, TokenIdent, TokenEqual, TokenNumber}},
		{name: "ref compared after :=", in: "x := ref=3", want: []TokenType{TokenIdent, TokenAssign, TokenIdent, TokenEqual, TokenNumber}},
		{name: "s output binding", in: "(s=>q", want: []TokenType{TokenLParen, TokenIdent, TokenOutputBind, TokenIdent}},
		{name: "keyword member", in: "st.Type", want: []TokenType{TokenIdent, TokenDot, TokenIdent}},
		{name: "keyword method", in: "fb.Method()", want: []TokenType{TokenIdent, TokenDot, TokenIdent, TokenLParen, TokenRParen}},
		{name: "type member", in: "log.Time", want: []TokenType{TokenIdent, TokenDot, TokenIdent}},
		{name: "keyword after range stays keyword", in: "1..2 OF", want: []TokenType{TokenNumber, TokenDotDot, TokenNumber, TokenOf}},
		{name: "illegal character kept", in: "a ? b", want: []TokenType{TokenIdent, TokenIllegal, TokenIdent}},
		{name: "non-ascii identifier", in: "Größe := 1", want: []TokenType{TokenIdent, TokenAssign, TokenNumber}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			toks := Lex(c.in)
			toks = toks[:len(toks)-1] // EOF
			if len(toks) != len(c.want) {
				t.Fatalf("Lex(%q) = %d tokens, want %d: %v", c.in, len(toks), len(c.want), toks)
			}
			for i, tok := range toks {
				if tok.Type != c.want[i] {
					t.Errorf("Lex(%q) token %d (%q) = %s, want %s",
						c.in, i, tok.Literal, tok.Type, c.want[i])
				}
			}
		})
	}
}

// TestLexKeepsEveryByte checks that the tokens and the whitespace between them
// add up to the input: no character may be skipped, whatever it is.
func TestLexKeepsEveryByte(t *testing.T) {
	t.Parallel()
	const src = "x := y ? 1 & z @ `w` ~ ! | \\ ; Größe := 'a';"
	var b strings.Builder
	for _, tok := range Lex(src) {
		b.WriteString(tok.Literal)
	}
	if want := strings.Join(strings.Fields(src), ""); b.String() != want {
		t.Errorf("tokens spell %q, want %q", b.String(), want)
	}
}
