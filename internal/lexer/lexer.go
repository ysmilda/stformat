package lexer

import (
	"strings"
	"unicode"
)

// Lexer tokenizes IEC 61131-3 Structured Text source code.
type Lexer struct {
	input  string
	pos    int
	line   int
	col    int
	tokens []Token
}

// LexError represents a lexical error with source position.
type LexError struct {
	Line int
	Col  int
	Msg  string
}

func (e LexError) Error() string {
	return e.Msg
}

// Lex tokenizes the input string and returns all tokens.
func Lex(input string) []Token {
	l := &Lexer{
		input: input,
		line:  1,
		col:   1,
	}
	l.lexAll()
	return l.tokens
}

// LexErrors tokenizes the input string and returns tokens plus any errors.
func LexErrors(input string) ([]Token, []LexError) {
	l := &Lexer{
		input: input,
		line:  1,
		col:   1,
	}
	l.lexAll()
	return l.tokens, nil
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
		Line:    l.line,
		Col:     l.col,
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
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
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
		Line:    l.line,
		Col:     l.col,
	})
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
			continue
		}
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
	l.advanceN(2) // skip //
	for l.pos < len(l.input) && l.peek() != '\n' {
		l.advance()
	}
	l.tokens = append(l.tokens, Token{
		Type:    TokenLineComment,
		Literal: l.input[start:l.pos],
		Line:    l.line,
		Col:     l.col,
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
		Line:    l.line,
		Col:     l.col,
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
		Line:    l.line,
		Col:     l.col,
	})
}

func (l *Lexer) lex() {
	ch := l.peek()

	// Check for two-character operators first
	if l.pos+1 < len(l.input) {
		// REF= must be checked before R=.
		if l.pos+3 < len(l.input) && strings.ToUpper(l.input[l.pos:l.pos+4]) == "REF=" {
			l.advanceN(4)
			l.emit(TokenRefAssign, "REF=")
			return
		}
		two := strings.ToUpper(l.input[l.pos : l.pos+2])
		switch two {
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

// lexString lexes a STRING ('...') or WSTRING ("...") literal.
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
		if l.peek() == '\n' {
			// Unterminated string; bail out
			break
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

	for l.pos < len(l.input) && isIdentPart(l.peek()) {
		l.advance()
	}

	literal := l.input[start:l.pos]
	upper := strings.ToUpper(literal)

	// Check for typed literals: T#5s, TIME#5s, INT#42, BOOL#TRUE, STRING#'x'
	if l.pos < len(l.input) && l.peek() == '#' {
		timePrefix := upper == "T" || upper == "TIME" || upper == "LT" || upper == "LTIME"
		typePrefix := IsScalarType(upper)

		if timePrefix || typePrefix {
			l.advance() // skip #
			// Read the payload
			switch upper {
			case "STRING", "WSTRING":
				// String literal payload
				if l.peek() == '\'' {
					l.lexStringPayload()
				}
			case "BOOL":
				// Boolean: TRUE/FALSE
				for l.pos < len(l.input) && isIdentPart(l.peek()) {
					l.advance()
				}
			case "T", "TIME", "LT", "LTIME":
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
	if upper == "T" || upper == "TIME" || upper == "LT" || upper == "LTIME" {
		if l.pos < len(l.input) && l.peek() == '#' {
			l.advance() // skip #
			l.lexTimePayload()
			l.emit(TokenTimeLiteral, l.input[start:l.pos])
			return
		}
	}

	// Check keyword
	if tt, ok := Keywords[upper]; ok {
		l.emit(tt, literal)
		return
	}

	l.emit(TokenIdent, literal)
}

func (l *Lexer) lexTimePayload() {
	for l.pos < len(l.input) {
		ch := l.peek()
		if isTimeChar(ch) || isDigit(ch) || ch == '_' || ch == '+' || ch == '-' || ch == ':' || ch == '.' {
			l.advance()
		} else if ch == ' ' {
			// A space is only part of the literal if a time component
			// follows it (e.g. T#1h 30m 20s), so trailing whitespace
			// before the next token is not swallowed.
			p := l.pos + 1
			for p < len(l.input) && l.input[p] == ' ' {
				p++
			}
			if p < len(l.input) && (isDigit(l.input[p]) || isTimeChar(l.input[p]) || l.input[p] == ':' || l.input[p] == '-' || l.input[p] == '.' || l.input[p] == '+' || l.input[p] == '_') {
				l.advance()
			} else {
				break
			}
		} else {
			break
		}
	}
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

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
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
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

func isTimeChar(ch byte) bool {
	return strings.ContainsRune("dhmsnspDT", rune(ch)) || unicode.IsLetter(rune(ch))
}
