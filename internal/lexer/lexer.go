package lexer

import (
	"strings"
)

// Lexer tokenizes IEC 61131-3 Structured Text source code.
type Lexer struct {
	input  string
	pos    int
	start  int // byte offset of the token currently being lexed
	tokens []Token
}

// Lex tokenizes the input string and returns all tokens.
func Lex(input string) []Token {
	l := &Lexer{
		input: input,
		// Sizing the slice up front keeps the append in emit from
		// reallocating and copying the whole token stream over and over,
		// which otherwise dominates both the allocation volume and the GC
		// scan work.
		tokens: make([]Token, 0, estimateTokens(len(input))),
	}
	l.lexAll()
	return l.tokens
}

// estimateTokens returns how many tokens a source of n bytes is expected to
// hold. ST measures around six bytes per token, so n/5 over-estimates slightly;
// the slack is worth more than the few bytes of unused slice, because falling
// short means reallocating and copying the whole token stream.
func estimateTokens(n int) int {
	return n / 5
}

func (l *Lexer) lexAll() {
	for l.pos < len(l.input) {
		l.skipWhitespaceAndComments()
		if l.pos >= len(l.input) {
			break
		}
		l.lex()
	}
	l.tokens = append(l.tokens, Token{
		Type:    TokenEOF,
		Literal: "",
		Offset:  l.pos,
	})
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) peekAt(offset int) byte {
	p := l.pos + offset
	if p >= len(l.input) {
		return 0
	}
	return l.input[p]
}

func (l *Lexer) advance() byte {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *Lexer) advanceN(n int) {
	for range n {
		l.advance()
	}
}

func (l *Lexer) emit(tt TokenType, literal string) {
	l.tokens = append(l.tokens, Token{
		Type:    tt,
		Literal: literal,
		Offset:  l.start,
	})
}

func (l *Lexer) skipWhitespaceAndComments() {
	// Runs of blanks are skipped in one go rather than a byte at a time.
	for l.pos < len(l.input) {
		if charClass[l.input[l.pos]]&classSpace != 0 {
			i := l.pos
			for i < len(l.input) && charClass[l.input[i]]&classSpace != 0 {
				i++
			}
			l.pos = i
			continue
		}
		ch := l.peek()
		if ch == '/' && l.peekAt(1) == '/' {
			l.skipLineComment()
			continue
		}
		if ch == '(' && l.peekAt(1) == '*' {
			l.skipBlockComment()
			continue
		}
		if ch == '{' {
			l.skipPragma()
			continue
		}
		break
	}
}

func (l *Lexer) skipLineComment() {
	start := l.pos
	// The newline that ends the comment is not part of it, so the scan stops
	// at the first one.
	body := l.input[l.pos+2:]
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		l.pos += 2 + i
	} else {
		l.pos = len(l.input)
	}
	// A CR from a CRLF file must be dropped too, or it survives into the
	// output and the formatted file ends up with mixed line endings.
	end := l.pos
	if end > start && l.input[end-1] == '\r' {
		end--
	}
	l.tokens = append(l.tokens, Token{
		Type:    TokenLineComment,
		Literal: l.input[start:end],
		Offset:  start,
	})
}

func (l *Lexer) skipBlockComment() {
	start := l.pos
	depth := 1
	l.advanceN(2) // skip (*
	for l.pos < len(l.input) && depth > 0 {
		if l.peek() == '(' && l.peekAt(1) == '*' {
			depth++
			l.advanceN(2)
			continue
		}
		if l.peek() == '*' && l.peekAt(1) == ')' {
			depth--
			l.advanceN(2)
			continue
		}
		l.advance()
	}
	l.tokens = append(l.tokens, Token{
		Type:    TokenBlockComment,
		Literal: l.input[start:l.pos],
		Offset:  start,
	})
}

func (l *Lexer) skipPragma() {
	start := l.pos
	depth := 1
	l.advance() // skip {
	for l.pos < len(l.input) && depth > 0 {
		if l.peek() == '{' {
			depth++
			l.advance()
			continue
		}
		if l.peek() == '}' {
			depth--
			l.advance()
			continue
		}
		l.advance()
	}
	l.tokens = append(l.tokens, Token{
		Type:    TokenBlockComment, // Pragmas preserved as comments
		Literal: l.input[start:l.pos],
		Offset:  start,
	})
}

func (l *Lexer) lex() {
	l.start = l.pos
	ch := l.peek()

	// Check for two-character operators first. The window is uppercased in
	// place so no substring is allocated: this runs for every token.
	if l.pos+1 < len(l.input) {
		// REF= must be checked before R=.
		if l.pos+3 < len(l.input) && equalFoldASCII(l.input[l.pos:l.pos+4], "REF=") {
			l.advanceN(4)
			l.emit(TokenRefAssign, "REF=")
			return
		}
		two := [2]byte{toUpperASCII(l.input[l.pos]), toUpperASCII(l.input[l.pos+1])}
		switch string(two[:]) {
		case ":=":
			l.advanceN(2)
			l.emit(TokenAssign, ":=")
			return
		case "=>":
			l.advanceN(2)
			l.emit(TokenOutputBind, "=>")
			return
		case "<>":
			l.advanceN(2)
			l.emit(TokenNotEqual, "<>")
			return
		case "<=":
			l.advanceN(2)
			l.emit(TokenLessEq, "<=")
			return
		case ">=":
			l.advanceN(2)
			l.emit(TokenGreaterEq, ">=")
			return
		case "..":
			l.advanceN(2)
			l.emit(TokenDotDot, "..")
			return
		case "**":
			l.advanceN(2)
			l.emit(TokenPower, "**")
			return
		case "S=":
			l.advanceN(2)
			l.emit(TokenSAssign, "S=")
			return
		case "R=":
			l.advanceN(2)
			l.emit(TokenRAssign, "R=")
			return
		}
	}

	// Single character operators and delimiters
	switch ch {
	case '+':
		l.advance()
		l.emit(TokenPlus, "+")
	case '-':
		l.advance()
		l.emit(TokenMinus, "-")
	case '*':
		l.advance()
		l.emit(TokenStar, "*")
	case '/':
		l.advance()
		l.emit(TokenSlash, "/")
	case '=':
		l.advance()
		l.emit(TokenEqual, "=")
	case '<':
		l.advance()
		l.emit(TokenLess, "<")
	case '>':
		l.advance()
		l.emit(TokenGreater, ">")
	case '(':
		l.advance()
		l.emit(TokenLParen, "(")
	case ')':
		l.advance()
		l.emit(TokenRParen, ")")
	case '[':
		l.advance()
		l.emit(TokenLBracket, "[")
	case ']':
		l.advance()
		l.emit(TokenRBracket, "]")
	case '{':
		l.advance()
		l.emit(TokenLBrace, "{")
	case '}':
		l.advance()
		l.emit(TokenRBrace, "}")
	case ';':
		l.advance()
		l.emit(TokenSemicolon, ";")
	case ':':
		l.advance()
		l.emit(TokenColon, ":")
	case ',':
		l.advance()
		l.emit(TokenComma, ",")
	case '.':
		l.advance()
		l.emit(TokenDot, ".")
	case '#':
		l.advance()
		l.emit(TokenHash, "#")
	case '^':
		l.advance()
		l.emit(TokenCaret, "^")
	case '%':
		l.lexAddress()
	case '\'':
		l.lexString(false)
	case '"':
		l.lexString(true)
	default:
		// Numbers
		if isDigit(ch) || (ch == '.' && isDigit(l.peekAt(1))) {
			l.lexNumber()
			return
		}
		// Identifiers and keywords
		if isIdentStart(ch) {
			l.lexIdentOrKeyword()
			return
		}
		// Unknown character - skip it
		l.advance()
	}
}

// lexString lexes a STRING ('...') or WSTRING ("...") literal. A literal may
// span several lines: the content of such a literal is kept byte-for-byte, so
// bailing out at the newline would silently rewrite the string. An
// unterminated literal therefore swallows the rest of the input, which the
// formatter copies through unchanged.
func (l *Lexer) lexString(wide bool) {
	start := l.pos
	quote := byte('\'')
	if wide {
		quote = '"'
	}
	l.advance() // skip opening quote
	for l.pos < len(l.input) && l.peek() != quote {
		if l.peek() == '$' {
			l.advance() // skip $
			if l.pos < len(l.input) {
				l.advance() // skip escape char
			}
			continue
		}
		l.advance()
	}
	if l.pos < len(l.input) {
		l.advance() // skip closing quote
	}
	l.emit(TokenString, l.input[start:l.pos])
}

func (l *Lexer) lexNumber() {
	start := l.pos

	// Check for based number: 2#1010, 8#777, 16#FF, 16#ABCD
	if l.pos+1 < len(l.input) && l.input[l.pos+1] == '#' {
		switch l.input[l.pos] {
		case '2':
			l.advanceN(2)
			l.lexBasedDigits(2)
			l.emit(TokenBasedNumber, l.input[start:l.pos])
			return
		case '8':
			l.advanceN(2)
			l.lexBasedDigits(8)
			l.emit(TokenBasedNumber, l.input[start:l.pos])
			return
		}
	}
	if l.pos+2 < len(l.input) && l.input[l.pos] == '1' && l.input[l.pos+1] == '6' && l.input[l.pos+2] == '#' {
		l.advanceN(3)
		l.lexBasedDigits(16)
		l.emit(TokenBasedNumber, l.input[start:l.pos])
		return
	}

	// Decimal number
	for l.pos < len(l.input) && (isDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}

	// Check for real number (fraction)
	if l.peek() == '.' && l.peekAt(1) != '.' && isDigit(l.peekAt(1)) {
		l.advance() // skip .
		for l.pos < len(l.input) && (isDigit(l.peek()) || l.peek() == '_') {
			l.advance()
		}
	}

	// Check for exponent
	if l.peek() == 'e' || l.peek() == 'E' {
		p := l.pos + 1
		if p < len(l.input) && (l.input[p] == '+' || l.input[p] == '-') {
			p++
		}
		if p < len(l.input) && isDigit(l.input[p]) {
			l.advance() // skip e/E
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
			for l.pos < len(l.input) && isDigit(l.peek()) {
				l.advance()
			}
		}
	}

	l.emit(TokenNumber, l.input[start:l.pos])
}

func (l *Lexer) lexBasedDigits(base int) {
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '#' || ch == '_' {
			l.advance()
			continue
		}
		if isBaseDigit(ch, base) {
			l.advance()
			continue
		}
		break
	}
}

func (l *Lexer) lexAddress() {
	start := l.pos
	l.advance() // skip %
	// Read I, Q, M, or * (incomplete)
	if l.pos < len(l.input) && (l.peek() == 'I' || l.peek() == 'i' || l.peek() == 'Q' || l.peek() == 'q' || l.peek() == 'M' || l.peek() == 'm' || l.peek() == '*') {
		l.advance()
	}
	// Read optional size modifier X, B, W, D, L
	if l.pos < len(l.input) {
		ch := l.peek()
		switch ch {
		case 'X', 'x', 'B', 'b', 'W', 'w', 'D', 'd', 'L', 'l':
			// Only consume if followed by a digit
			if l.pos+1 < len(l.input) && isDigit(l.peekAt(1)) {
				l.advance()
			}
		}
	}
	// Pin marker replaces the offset: %I*, %Q* (runtime-configured I/O)
	if l.pos < len(l.input) && l.peek() == '*' {
		l.advance()
	}
	// Read digits and optional .digits
	for l.pos < len(l.input) && (isDigit(l.peek()) || l.peek() == '.') {
		l.advance()
	}
	l.emit(TokenAddress, l.input[start:l.pos])
}

func (l *Lexer) lexIdentOrKeyword() {
	start := l.pos

	// Walk the input directly instead of through peek/advance: this loop runs
	// once per identifier byte, and the bounds check it needs is the only one
	// that matters.
	i := start
	for i < len(l.input) && charClass[l.input[i]]&classIdentPart != 0 {
		i++
	}
	l.pos = i

	literal := l.input[start:l.pos]
	// An identifier longer than maxKeywordLen cannot be a keyword or an
	// elementary type name, so it skips the uppercase copy and the lookups.
	// That matters because the copy is otherwise repeated for every
	// identifier in the file, and most identifiers are not keywords.
	if len(literal) > maxKeywordLen {
		l.emit(TokenIdent, literal)
		return
	}

	// Uppercase into a stack buffer rather than calling strings.ToUpper, which
	// allocates whenever the identifier holds a lowercase letter. The keyword
	// lookup hashes the uppercase form, so it needs no second pass.
	var scratch [maxKeywordLen]byte
	upper, h := upperASCII(scratch[:], literal)
	tt, scalar, isKeyword := lookupKeywordHashed(upper, h)

	// Check for typed literals: T#5s, TIME#5s, INT#42, BOOL#TRUE, STRING#'x'
	if l.pos < len(l.input) && l.peek() == '#' {
		timePrefix := isTimePrefix(upper)

		if timePrefix || scalar {
			l.advance() // skip #
			// Read the payload
			switch {
			case string(upper) == "STRING" || string(upper) == "WSTRING":
				// String literal payload
				if l.peek() == '\'' {
					l.lexStringPayload()
				}
			case string(upper) == "BOOL":
				// Boolean: TRUE/FALSE
				for l.pos < len(l.input) && isIdentPart(l.peek()) {
					l.advance()
				}
			case timePrefix:
				l.lexTimePayload()
			default:
				// Numeric payload
				for l.pos < len(l.input) && (isDigit(l.peek()) || isHexDigit(l.peek()) || l.peek() == '#' || l.peek() == '_' || l.peek() == '+' || l.peek() == '-' || l.peek() == ':' || l.peek() == '.') {
					l.advance()
				}
				// Optional exponent
				if l.pos < len(l.input) && (l.peek() == 'e' || l.peek() == 'E') {
					p := l.pos + 1
					if p < len(l.input) && (l.input[p] == '+' || l.input[p] == '-') {
						p++
					}
					if p < len(l.input) && isDigit(l.input[p]) {
						l.pos = p + 1
						for l.pos < len(l.input) && isDigit(l.peek()) {
							l.pos++
						}
					}
				}
			}
			l.emit(TokenTypedLiteral, l.input[start:l.pos])
			return
		}
	}

	// Check for time literals: T#5s, TIME#5s, LT#5s
	if isTimePrefix(upper) {
		if l.pos < len(l.input) && l.peek() == '#' {
			l.advance() // skip #
			l.lexTimePayload()
			l.emit(TokenTimeLiteral, l.input[start:l.pos])
			return
		}
	}

	// Check keyword
	if isKeyword {
		l.emit(tt, literal)
		return
	}

	l.emit(TokenIdent, literal)
}

// maxKeywordLen is the length of the longest keyword and elementary type name
// in Keywords ("END_FUNCTION_BLOCK"). An identifier longer than this cannot be
// either, so it needs no uppercase copy or lookup at all.
const maxKeywordLen = len("END_FUNCTION_BLOCK")

// upperASCII writes the uppercase form of s into dst and returns it together
// with its hash, which is what the keyword table is probed with. s must not be
// longer than dst; the caller guarantees that with maxKeywordLen.
func upperASCII(dst []byte, s string) ([]byte, uint32) {
	h := uint32(2166136261)
	for i := range len(s) {
		c := toUpperASCII(s[i])
		dst[i] = c
		h = (h ^ uint32(c)) * 16777619
	}
	return dst[:len(s)], h
}

// isTimePrefix reports whether an uppercased identifier prefixes a duration
// literal (T#5s, TIME#5s, LT#5s, LTIME#5s).
func isTimePrefix(upper []byte) bool {
	switch string(upper) {
	case "T", "TIME", "LT", "LTIME":
		return true
	}
	return false
}

// lexTimePayload consumes the components of a duration literal such as
// T#1h30m20s or T#1d 12h 30m. A space only continues the literal when another
// component follows, and a component never starts with an arbitrary letter:
// that would swallow the code following the literal (for example
// `T#0MS THEN x := 1`).
func (l *Lexer) lexTimePayload() {
	for l.pos < len(l.input) {
		ch := l.peek()
		if isDigit(ch) || isTimeUnitChar(ch) || ch == '_' || ch == '-' ||
			ch == '+' || ch == ':' || ch == '.' {
			l.advance()
			continue
		}
		if ch == ' ' {
			// A space is part of the literal only when another component
			// follows (e.g. `T#1h 30m 20s`), so trailing whitespace
			// before the next token is not swallowed.
			if startsTimeComponent(l.input, l.pos) {
				l.advance()
				continue
			}
			break
		}
		break
	}
}

// startsTimeComponent reports whether the space at index i is followed by
// another duration component: a number, or a unit that is itself followed by a
// number (`T#1h 30m`).
func startsTimeComponent(s string, i int) bool {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return false
	}
	if isDigit(s[i]) {
		return true
	}
	if !isTimeUnitChar(s[i]) {
		return false
	}
	for i++; i < len(s) && (isTimeUnitChar(s[i]) || isDigit(s[i])); i++ {
	}
	return i < len(s) && isDigit(s[i])
}

func (l *Lexer) lexStringPayload() {
	for l.pos < len(l.input) && l.peek() != '\'' {
		if l.peek() == '$' {
			l.advance() // skip $
			if l.pos < len(l.input) {
				l.advance() // skip escape char
			}
		} else {
			l.advance()
		}
	}
	if l.pos < len(l.input) {
		l.advance() // skip closing '
	}
}

// toUpperASCII uppercases a single byte. ST identifiers and operators are
// ASCII, so this is enough and it avoids the allocation strings.ToUpper makes
// whenever the input has a lowercase letter.
func toUpperASCII(ch byte) byte {
	if ch >= 'a' && ch <= 'z' {
		return ch - 'a' + 'A'
	}
	return ch
}

// equalFoldASCII reports whether s equals upper, comparing ASCII letters
// case-insensitively. s must already have len(upper) bytes.
func equalFoldASCII(s string, upper string) bool {
	for i := range len(upper) {
		if toUpperASCII(s[i]) != upper[i] {
			return false
		}
	}
	return true
}

func isDigit(ch byte) bool {
	return charClass[ch]&classDigit != 0
}

func isHexDigit(ch byte) bool {
	return charClass[ch]&classHexDigit != 0
}

// The Raw variants hold the plain range tests that build charClass, and are
// the reference the table is built from.
func isDigitRaw(ch byte) bool { return ch >= '0' && ch <= '9' }

func isHexDigitRaw(ch byte) bool {
	return isDigitRaw(ch) || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
}

func isIdentStartRaw(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func isBaseDigit(ch byte, base int) bool {
	switch base {
	case 2:
		return ch == '0' || ch == '1'
	case 8:
		return ch >= '0' && ch <= '7'
	case 10:
		return isDigit(ch)
	case 16:
		return isHexDigit(ch)
	}
	return false
}

func isIdentStart(ch byte) bool {
	return charClass[ch]&classIdentStart != 0
}

func isIdentPart(ch byte) bool {
	return charClass[ch]&classIdentPart != 0
}

// Character classes, as bits in charClass. The lexer tests these once per
// source byte, so a table lookup replaces the range comparisons.
const (
	classIdentStart = 1 << iota
	classIdentPart
	classDigit
	classHexDigit
	classSpace
)

var charClass = buildCharClass()

func buildCharClass() [256]uint8 {
	var t [256]uint8
	for ch := range 256 {
		b := byte(ch)
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			t[ch] |= classSpace
		}
		if isDigitRaw(b) {
			t[ch] |= classDigit | classIdentPart
		}
		if isHexDigitRaw(b) {
			t[ch] |= classHexDigit
		}
		if isIdentStartRaw(b) {
			t[ch] |= classIdentStart | classIdentPart
		}
	}
	return t
}

// isTimeUnitChar reports whether ch may appear in the unit part of a duration
// literal (T#1h30m, LT#2d12h, TIME#500ms). Units are single letters with
// optional prefixes: d, h, m, s, ms, us, ns. Literals are case insensitive.
func isTimeUnitChar(ch byte) bool {
	return strings.IndexByte("dhmsunDHMSUN", ch) >= 0
}
