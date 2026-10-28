package formatter

import (
	"strings"
	"unicode/utf8"

	"github.com/ysmilda/stformat/internal/lexer"
)

// blockKind identifies the type of a block on the indentation stack.
type blockKind int

const (
	bPOU blockKind = iota
	bVar
	bType
	bStruct
	bIf
	bFor
	bWhile
	bRepeat
	bCase
	bCaseLabel
)

// printer formats a token stream into properly formatted ST source code.
// It uses an explicit block stack to compute indentation and a
// token-driven state machine to decide line breaks and spacing.
// Line breaks are deferred ("pending") so that indentation changes that
// apply to the next token are reflected before the line break is rendered.
type printer struct {
	tokens []lexer.Token
	// source holds the input with its line endings normalised to LF, so the
	// token offsets and the verbatim copies below line up. original is the
	// input as it was given, ending included.
	source   string
	original string
	// ending is the line ending to put in the output; the pass itself only
	// ever writes LF and run converts at the end.
	ending    string
	ignoreAll bool
	runs      []tokenRun
	runPos    int
	pos       int
	blocks    []blockKind
	// depth is the indentation level: the number of open blocks that are not
	// POU-like. It is maintained by push/pop instead of being recounted from
	// the stack on every line break.
	depth     int
	buf       strings.Builder
	lastCh    byte
	prevCh    byte
	lineStart bool
	pendingNL bool

	pendingBlank bool
	// inner lists, in ascending order, the output lines that continue a
	// multi-line string literal or comment: text that the line-level pass
	// must leave alone.
	inner []int
	// lastLit is the text of the last token attach wrote.
	lastLit string
	// glue makes the next token attach without the space it would otherwise
	// get, as the operand of a unary sign does. Any write clears it.
	glue     bool
	loopKw   lexer.TokenType // set by FOR/WHILE openers, consumed by DO
	caseSeen bool            // set by CASE opener, consumed by OF
	// bom is set when the input starts with a UTF-8 byte order mark, which
	// is kept out of the pass and put back in front of the output.
	bom bool
}

// utf8BOM is the byte order mark some editors put at the start of a UTF-8 file.
const utf8BOM = "\uFEFF"

func newPrinter(source, ending string) *printer {
	text, bom := strings.CutPrefix(source, utf8BOM)
	text = toLF(text)
	f := &printer{
		tokens:    lexer.Lex(text),
		source:    text,
		original:  source,
		ending:    ending,
		lineStart: true,
		lastCh:    '\n',
		bom:       bom,
	}
	// Formatting mostly preserves length and adds indentation and spacing, so
	// the input size is a good first guess. Sizing the builder up front avoids
	// the repeated grow-and-copy of the whole output.
	f.buf.Grow(len(text))
	if all, runs := scanDirectives(f.tokens); !all {
		f.runs = runs
	} else {
		// A file-level "stformat:ignore" leaves the file byte-for-byte alone.
		f.ignoreAll = true
	}
	return f
}

// Format formats the source, keeping the line ending it already uses.
func Format(source string) string {
	return newPrinter(source, LineEnding(source)).run()
}

// FormatWith formats the source and writes every line break as ending. Use it
// when the line ending must follow something other than the text being
// formatted, such as the rest of an XML file whose CDATA section holds the code.
// An ending other than "\r\n" or "\n" falls back to the one found in source.
func FormatWith(source, ending string) string {
	if ending != "\r\n" && ending != "\n" {
		ending = LineEnding(source)
	}
	return newPrinter(source, ending).run()
}

// run executes the formatting pass and returns the output string, using the
// line ending of the input.
func (f *printer) run() string {
	if f.ignoreAll {
		return f.original
	}
	for {
		if f.emitVerbatim() {
			continue
		}
		tok := f.peek()
		if tok.Type == lexer.TokenEOF {
			break
		}
		f.emitToken()
	}
	// A break still pending at the end would only write indentation. The file
	// ends with a single line break, unless the input ends inside a token
	// that never closes: a break written now would become part of its text.
	f.pendingNL, f.pendingBlank = false, false
	if f.lastCh != '\n' && !endsOpen(f.source, f.tokens) {
		f.writeCh('\n')
	}
	out := postProcess(f.buf.String(), f.inner)
	if f.ending != "\n" {
		out = strings.ReplaceAll(out, "\n", f.ending)
	}
	if f.bom {
		out = utf8BOM + out
	}
	return out
}

// emitVerbatim copies an ignored region from the source text and skips the
// tokens it covers. It returns false when the next token is not part of an
// ignored region.
func (f *printer) emitVerbatim() bool {
	if f.runPos >= len(f.runs) {
		return false
	}
	run := f.runs[f.runPos]
	if run.start != f.pos {
		return false
	}
	f.runPos++

	from := f.tokens[run.start].Offset
	to := len(f.source)
	if run.end < len(f.tokens) {
		to = f.tokens[run.end].Offset
	}
	if trailsCode(f.source, from) {
		// The directive trails code on its own source line: drop the
		// pending line break so it stays there.
		f.pendingNL = false
		f.pendingBlank = false
		f.flush()
		f.writeCh(' ')
		f.writeCh(' ')
	} else {
		// The directive stands on a line of its own in the source, even
		// when the code before it did not end its statement.
		f.startOwnLine()
		f.flush()
	}
	text := f.source[from:to]
	if to < len(f.source) || !endsOpen(f.source, f.tokens) {
		// Trailing blanks go, unless they belong to a literal that is
		// still open at the end of the input.
		text = strings.TrimRight(text, " \t")
	}
	f.markInner(text)
	f.writeRaw(text)
	if f.lastCh == '\n' {
		// The verbatim text ends its own line; indent what follows.
		f.requestNL()
	}
	f.pos = run.end
	return true
}

// endsOpen reports whether the source ends inside a string literal, comment
// or pragma that is never closed. Such a token runs to the end of the input,
// so a line break added after it would become part of its text. It is told
// apart from a closed token at the very end by lexing the tail again with a
// line break behind it: an unclosed token swallows the break.
func endsOpen(src string, tokens []lexer.Token) bool {
	if len(tokens) < 2 {
		return false
	}
	last := tokens[len(tokens)-2] // the one before EOF
	if last.Offset+len(last.Literal) != len(src) {
		return false
	}
	switch last.Type {
	case lexer.TokenString, lexer.TokenTypedLiteral, lexer.TokenBlockComment, lexer.TokenPragma:
		return lexer.Lex(last.Literal + "\n")[0].Literal != last.Literal
	}
	return false
}

// trailsCode reports whether the byte at index i is preceded, on the same line
// and without an intervening line break, by non-blank text.
func trailsCode(src string, i int) bool {
	for i--; i >= 0; i-- {
		switch src[i] {
		case '\n':
			return false
		case ' ', '\t', '\r':
			continue
		default:
			return true
		}
	}
	return false
}

func (f *printer) peek() lexer.Token {
	if f.pos >= len(f.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return f.tokens[f.pos]
}

func (f *printer) advance() lexer.Token {
	tok := f.peek()
	f.pos++
	return tok
}

// ------------- output primitives -------------

func (f *printer) writeCh(c byte) {
	f.prevCh = f.lastCh
	f.buf.WriteByte(c)
	f.lastCh = c
	f.lineStart = c == '\n'
}

// requestNL marks that the next content should begin on a new line.
// The actual newline is rendered by flush() using the then-current indent.
func (f *printer) requestNL() {
	f.pendingNL = true
}

// requestBlank marks that the next content should begin on a new line
// preceded by a blank line.
func (f *printer) requestBlank() {
	f.requestNL()
	f.pendingBlank = true
}

// flush renders a pending line break at the current indentation.
func (f *printer) flush() {
	if !f.pendingNL {
		return
	}
	f.pendingNL = false
	blank := f.pendingBlank
	f.pendingBlank = false
	if f.lastCh != '\n' {
		f.writeCh('\n')
	}
	if blank {
		f.writeCh('\n')
	}
	for range f.depth {
		f.writeCh('\t')
	}
	f.lineStart = true
}

// attach writes a string, flushing any pending line break first,
// without adding any leading whitespace.
func (f *printer) attach(s string) {
	f.flush()
	f.glue = false
	// Two tokens written back to back must not read as one: x and y become
	// xy, ( and * open a comment. They are kept apart whatever the layout
	// rules asked for.
	if s != "" && f.mergesWith(s) {
		f.writeCh(' ')
	}
	f.markInner(s)
	f.writeRaw(s)
	f.lastLit = s
}

// markInner records the output lines that s, about to be written, continues
// onto: the lines after its first one. They hold the inside of a multi-line
// token or an ignored region, which the line-level pass must leave alone.
func (f *printer) markInner(s string) {
	// A final line break ends the text; the line after it is not part of it.
	if n := strings.Count(strings.TrimSuffix(s, "\n"), "\n"); n > 0 {
		first := strings.Count(f.buf.String(), "\n") + 1
		for i := range n {
			f.inner = append(f.inner, first+i)
		}
	}
}

// mergesWith reports whether s, written right after the output so far, would
// lex together with what precedes it.
func (f *printer) mergesWith(s string) bool {
	prev, c := f.lastCh, s[0]
	if isBlank(prev) {
		return false
	}
	switch {
	case isWordByte(prev) && (isWordByte(c) || c == '#'):
		// Two words, or a word and '#' (T #5s would turn into T#5s).
		return true
	case prev == '.' && isDigit(c) && f.numberBeforeDot():
		// 1 . 5 would turn into the number 1.5 (but wValue.0 is bit access).
		return true
	case (isWordByte(c) || strings.IndexByte(".#_-+:*'\"", c) >= 0) && literalTail(f.lastLit):
		// A literal takes in characters that a rule of its own cannot
		// foresee (T# .0, %IX1 .5, 16#F F), so the pair is lexed again to see
		// whether it still splits where it did.
		return lexer.Lex(f.lastLit + s)[0].Literal != f.lastLit
	}
	switch string([]byte{prev, c}) {
	case "(*", "*)", "//", ":=", "=>", "<>", "<=", ">=", "..", "**":
		return true
	}
	return false
}

// numberBeforeDot reports whether the output ends in a number followed by a
// '.', such as "1_0.".
func (f *printer) numberBeforeDot() bool {
	out := f.buf.String()
	end := len(out) - 1 // the '.'
	i := end
	for i > 0 && isWordByte(out[i-1]) {
		i--
	}
	return i < end && isDigit(out[i])
}

// literalTail reports whether lit is a literal that may run on into the text
// after it: a number, an address, or a typed or time literal.
func literalTail(lit string) bool {
	return lit != "" && (isDigit(lit[0]) || lit[0] == '%' || strings.Contains(lit, "#"))
}

// isWordByte reports whether c can be part of an identifier, keyword or
// number, ASCII or not.
func isWordByte(c byte) bool {
	return isIdentChar(c) || c >= utf8.RuneSelf
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// writeRaw copies text from the source unchanged, newlines included.
//
// Only the tail goes through writeCh one byte at a time; the rest is handed to
// the builder in one copy, which is what writeRaw spends its time on (ident
// after a keyword, verbatim block, comment body).
func (f *printer) writeRaw(s string) {
	if len(s) <= tailTrackedLen {
		for i := range len(s) {
			f.writeCh(s[i])
		}
		return
	}
	n := len(s)
	f.buf.WriteString(s[:n-tailTrackedLen])
	for i := n - tailTrackedLen; i < n; i++ {
		f.writeCh(s[i])
	}
}

// tailTrackedLen is how many trailing bytes writeRaw still walks byte by byte.
// Callers inspect only the last two bytes written: lastCh decides spacing and
// lineStart, and prevCh is read by blankLineBeforeTop to spot a blank line.
// Three bytes go through writeCh because the boundary byte has to be tracked
// too, otherwise prevCh would hold a byte from before this call.
const tailTrackedLen = 3

// space writes a single space unless at line start or already spaced.
func (f *printer) space() {
	f.flush()
	if f.lineStart {
		return
	}
	if f.glue {
		f.glue = false
		return
	}
	switch f.lastCh {
	case ' ', '\t', '(', '[', '.', '\n':
		return
	}
	f.writeCh(' ')
}

// startOwnLine ensures a new line will begin at the current position.
func (f *printer) startOwnLine() {
	if !f.lineStart {
		f.requestNL()
	}
}

// push opens a block. POU-like blocks (FUNCTION/FUNCTION_BLOCK/PROGRAM/
// METHOD/...) do not indent their bodies: their statements sit at the POU
// keyword's column, so they do not raise the depth.
func (f *printer) push(b blockKind) {
	f.blocks = append(f.blocks, b)
	if b != bPOU {
		f.depth++
	}
}

func (f *printer) pop() blockKind {
	if len(f.blocks) == 0 {
		return bPOU
	}
	b := f.blocks[len(f.blocks)-1]
	f.blocks = f.blocks[:len(f.blocks)-1]
	if b != bPOU {
		f.depth--
	}
	return b
}

func (f *printer) top() blockKind {
	if len(f.blocks) == 0 {
		return bPOU
	}
	return f.blocks[len(f.blocks)-1]
}

func (f *printer) prevType() lexer.TokenType {
	if f.pos == 0 {
		return lexer.TokenEOF
	}
	return f.tokens[f.pos-1].Type
}

// afterOperand reports whether the last token before the current one, comments
// aside, ends an operand. A '+' or '-' behind one is a binary operator; behind
// anything else (an operator, '(', ',', ':=', a keyword) it is a sign.
func (f *printer) afterOperand() bool {
	for i := f.pos - 1; i >= 0; i-- {
		switch f.tokens[i].Type {
		case lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenPragma:
			continue
		case lexer.TokenIdent, lexer.TokenNumber, lexer.TokenBasedNumber, lexer.TokenString,
			lexer.TokenTimeLiteral, lexer.TokenTypedLiteral, lexer.TokenAddress,
			lexer.TokenTrue, lexer.TokenFalse,
			lexer.TokenRParen, lexer.TokenRBracket, lexer.TokenCaret:
			return true
		}
		return false
	}
	return false
}

// isBlank reports whether c is whitespace in the (LF-normalised) source.
func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

// endBreak requests the break that follows a closed block: a blank line
// unless the next token closes another block or the source ends.
func (f *printer) endBreak() {
	next := f.peek().Type
	if next != lexer.TokenEOF && !lexer.IsBlockEnd(next) {
		f.requestBlank()
	} else {
		f.requestNL()
	}
}

// ------------- token emission -------------

// emitToken lays out the token at the current position and moves past it,
// together with any tokens a header parser consumes along with it.
//
// The switch is a dispatch table over the token types, grouped by what they
// mean for layout; the banner comments name the groups.
func (f *printer) emitToken() {
	tok := f.peek()

	if lexer.IsComment(tok.Type) {
		f.emitComment(tok)
		return
	}

	lit := tokenSpelling(tok)

	switch tok.Type {
	// ------- POU declarations -------
	case lexer.TokenProgram, lexer.TokenFunction, lexer.TokenFunctionBlock,
		lexer.TokenClass, lexer.TokenInterface:
		f.blankLineBeforeTop()
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()
		f.parsePOUHeader(tok.Type)
		f.push(bPOU)
		f.requestNL()

	// METHOD / PROPERTY / ACTION are POU-like but appear inside an
	// enclosing POU, so they share the enclosing block's indent and no
	// blank line.
	case lexer.TokenMethod, lexer.TokenProperty, lexer.TokenAction:
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()
		f.parseMemberHeader(tok.Type)
		f.push(bPOU)
		f.requestNL()

	case lexer.TokenEndProgram, lexer.TokenEndFunction, lexer.TokenEndFunctionBlock,
		lexer.TokenEndAction, lexer.TokenEndMethod, lexer.TokenEndProperty,
		lexer.TokenEndClass, lexer.TokenEndInterface:
		f.popUntil(bPOU)
		f.startOwnLine()
		f.attach(lit)
		f.advance()

	// ------- TYPE blocks -------
	case lexer.TokenTypeKw:
		f.blankLineBeforeTop()
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.parseTypeHeader()
		f.push(bType)
		f.requestNL()

	case lexer.TokenEndType:
		f.popUntil(bType)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	// ------- VAR blocks -------
	case lexer.TokenVar, lexer.TokenVarInput, lexer.TokenVarOutput,
		lexer.TokenVarInOut, lexer.TokenVarTemp, lexer.TokenVarGlobal,
		lexer.TokenVarExternal, lexer.TokenVarConfig, lexer.TokenVarStat,
		lexer.TokenVarInst, lexer.TokenVarAccess:
		f.startOwnLine()
		f.attach(lit)
		f.advance()

	qualifiers:
		for {
			q := f.peek()
			switch q.Type {
			case lexer.TokenRetain, lexer.TokenConstant, lexer.TokenNonRetain, lexer.TokenPersistent:
				f.space()
				f.attach(keywordSpelling(q))
				f.advance()
			default:
				break qualifiers
			}
		}
		f.push(bVar)
		f.requestNL()

	case lexer.TokenEndVar:
		f.popUntil(bVar)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	// ------- STRUCT / UNION -------
	case lexer.TokenStruct, lexer.TokenUnion:
		if f.lineStart || f.lastCh == ' ' {
			f.attach(lit)
		} else {
			f.space()
			f.attach(lit)
		}
		f.advance()
		f.push(bStruct)
		f.requestNL()

	case lexer.TokenEndStruct, lexer.TokenEndUnion:
		f.popUntil(bStruct)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	// ------- Control flow openers -------
	case lexer.TokenIf, lexer.TokenFor, lexer.TokenWhile, lexer.TokenCase, lexer.TokenJmp:
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()
		f.loopKw = tok.Type
		if tok.Type == lexer.TokenCase {
			f.caseSeen = true
		}

	case lexer.TokenExit, lexer.TokenContinue, lexer.TokenReturn:
		if !f.lineStart && !f.pendingNL {
			f.startOwnLine()
		}
		f.attach(lit)
		f.advance()

	case lexer.TokenThen:
		f.space()
		f.attach(lit)
		f.advance()
		f.push(bIf)
		f.requestNL()

	case lexer.TokenDo:
		f.space()
		f.attach(lit)
		f.advance()
		if f.loopKw == lexer.TokenWhile {
			f.push(bWhile)
		} else {
			f.push(bFor)
		}
		f.requestNL()

	case lexer.TokenRepeat:
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.push(bRepeat)
		f.requestNL()

	case lexer.TokenOf:
		isCase := f.caseSeen
		f.caseSeen = false
		f.space()
		f.attach(lit)
		f.advance()
		if isCase {
			f.push(bCase)
			f.requestNL()
		}

	// ------- Control flow continuations -------
	case lexer.TokenElsif:
		f.popUntil(bIf)
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()

	case lexer.TokenElse:
		if f.top() == bCaseLabel {
			f.pop()
			f.startOwnLine()
			f.attach(lit)
			f.advance()
			f.push(bCaseLabel)
			f.requestNL()
		} else {
			f.popUntil(bIf)
			f.startOwnLine()
			f.attach(lit)
			f.advance()
			f.push(bIf)
			f.requestNL()
		}

	case lexer.TokenUntil:
		f.popUntil(bRepeat)
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()

	// ------- Control flow closers -------
	case lexer.TokenEndIf:
		f.popUntil(bIf)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	case lexer.TokenEndFor:
		f.popUntil(bFor)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	case lexer.TokenEndWhile:
		f.popUntil(bWhile)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	case lexer.TokenEndRepeat:
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	case lexer.TokenEndCase:
		for f.top() == bCaseLabel {
			f.pop()
		}
		f.popUntil(bCase)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

	// ------- Keywords that continue a line -------
	case lexer.TokenTo, lexer.TokenBy, lexer.TokenAt:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()

	case lexer.TokenArray:
		f.space()
		f.attach(lit)
		f.advance()

	case lexer.TokenRetain, lexer.TokenConstant, lexer.TokenNonRetain, lexer.TokenPersistent:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()

	// ------- Operators -------
	case lexer.TokenAssign:
		f.space()
		f.attach(":=")
		f.space()
		f.advance()

	case lexer.TokenOutputBind:
		f.space()
		f.attach("=>")
		f.space()
		f.advance()

	case lexer.TokenSAssign, lexer.TokenRAssign, lexer.TokenRefAssign:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()

	case lexer.TokenPlus, lexer.TokenMinus:
		unary := !f.afterOperand()
		f.space()
		f.attach(lit)
		if unary {
			// A sign binds to its operand: -1, a * -b, -(a + b).
			f.glue = true
		} else {
			f.space()
		}
		f.advance()

	case lexer.TokenIllegal:
		// Not ST, so there is no rule to space it by: it keeps the spacing
		// the source gave it on either side.
		if before := tok.Offset - 1; before >= 0 && isBlank(f.source[before]) {
			f.space()
		}
		f.attach(lit)
		if after := tok.Offset + len(lit); after < len(f.source) && !isBlank(f.source[after]) {
			f.glue = true
		}
		f.advance()

	case lexer.TokenEqual, lexer.TokenNotEqual, lexer.TokenLess, lexer.TokenLessEq,
		lexer.TokenGreater, lexer.TokenGreaterEq,
		lexer.TokenStar, lexer.TokenSlash, lexer.TokenPower, lexer.TokenMod,
		lexer.TokenAnd, lexer.TokenOr, lexer.TokenXor, lexer.TokenAndThen,
		lexer.TokenOrElse:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()

	case lexer.TokenNot:
		f.space()
		f.attach("NOT")
		f.advance()
		// NOT(x) reads as a call; NOT x needs the space.
		if f.peek().Type != lexer.TokenLParen {
			f.space()
		}

	// ------- Punctuation -------
	case lexer.TokenSemicolon:
		f.attach(";")
		f.advance()
		f.requestNL()

	case lexer.TokenComma:
		f.attach(",")
		f.space()
		f.advance()

	case lexer.TokenColon:
		f.handleColon()
		f.advance()

	case lexer.TokenDot:
		f.attach(".")
		f.advance()

	case lexer.TokenDotDot:
		f.attach("..")
		f.advance()

	case lexer.TokenLParen:
		f.handleLParen()
		f.advance()

	case lexer.TokenRParen:
		f.attach(")")
		f.advance()

	case lexer.TokenLBracket:
		f.attach("[")
		f.advance()

	case lexer.TokenRBracket:
		f.attach("]")
		f.advance()

	// ------- Identifiers and literals -------
	case lexer.TokenIdent:
		f.handleIdent(tok)
		f.advance()

	case lexer.TokenNumber, lexer.TokenBasedNumber, lexer.TokenString,
		lexer.TokenTimeLiteral, lexer.TokenTypedLiteral, lexer.TokenAddress:
		f.handleAtom(tok, "")
		f.advance()

	case lexer.TokenBoolType, lexer.TokenByteType, lexer.TokenWordType,
		lexer.TokenDwordType, lexer.TokenLwordType,
		lexer.TokenSintType, lexer.TokenIntType, lexer.TokenDintType, lexer.TokenLintType,
		lexer.TokenUsintType, lexer.TokenUintType, lexer.TokenUdintType, lexer.TokenUlintType,
		lexer.TokenRealType, lexer.TokenLrealType,
		lexer.TokenTimeType, lexer.TokenLtimeType, lexer.TokenDateType, lexer.TokenLdateType,
		lexer.TokenTodType, lexer.TokenLtodType, lexer.TokenDtType, lexer.TokenLdtType,
		lexer.TokenStringType, lexer.TokenWstringType, lexer.TokenCharType, lexer.TokenWcharType:
		f.handleAtom(tok, keywordSpelling(tok))
		f.advance()

	case lexer.TokenTrue, lexer.TokenFalse:
		f.handleAtom(tok, keywordSpelling(tok))
		f.advance()

	// ------- Misc keywords -------
	case lexer.TokenExtends, lexer.TokenImplements, lexer.TokenAbstract, lexer.TokenFinal,
		lexer.TokenInternal, lexer.TokenPrivate, lexer.TokenProtected, lexer.TokenPublic,
		lexer.TokenThis, lexer.TokenSuper,
		lexer.TokenReadOnly, lexer.TokenReadWrite, lexer.TokenParams:
		f.space()
		f.attach(lit)
		f.advance()

	default:
		f.attach(lit)
		f.advance()
	}
}

// ------------- specific handlers -------------

// blankLineBeforeTop inserts a blank line before a top-level declaration
// (POU, global VAR block, TYPE block) that follows other content.
func (f *printer) blankLineBeforeTop() {
	if len(f.blocks) > 0 || f.buf.Len() == 0 {
		return
	}
	f.flush()
	if f.lastCh != '\n' {
		f.writeCh('\n')
	}
	if f.lastCh != '\n' || f.prevCh != '\n' {
		f.writeCh('\n')
	}
	for range f.depth {
		f.writeCh('\t')
	}
	f.lineStart = true
}

// parsePOUHeader consumes the POU name and optional return type /
// EXTENDS / IMPLEMENTS clauses, emitting the ':' before a FUNCTION return
// type. The caller has already emitted the keyword and any block keyword
// that precedes it.
func (f *printer) parsePOUHeader(kind lexer.TokenType) {
	name := f.peek()
	if name.Type == lexer.TokenIdent {
		f.attach(name.Literal)
		f.advance()
	}

	if kind == lexer.TokenFunction {
		if f.peek().Type == lexer.TokenColon {
			f.advance()
			f.space()
			f.attach(":")
			f.space()
			rt := f.peek()
			if rt.Type != lexer.TokenEOF {
				f.attach(keywordSpelling(rt))
				f.advance()
			}
		}
	}

	for {
		switch f.peek().Type {
		case lexer.TokenExtends:
			f.advance()
			f.space()
			f.attach("EXTENDS")
			f.space()
			if n := f.peek(); n.Type == lexer.TokenIdent {
				f.attach(n.Literal)
				f.advance()
			}
		case lexer.TokenImplements:
			f.advance()
			f.space()
			f.attach("IMPLEMENTS")
			for {
				f.space()
				if n := f.peek(); n.Type == lexer.TokenIdent {
					f.attach(n.Literal)
					f.advance()
				}
				if f.peek().Type == lexer.TokenComma {
					f.attach(",")
					f.space()
					f.advance()
					continue
				}
				break
			}
		default:
			return
		}
	}
}

// parseTypeHeader consumes `name :` after a TYPE keyword.
func (f *printer) parseTypeHeader() {
	name := f.peek()
	if name.Type == lexer.TokenIdent {
		f.space()
		f.attach(name.Literal)
		f.advance()
	}
	// Consume optional colon (TYPE name : STRUCT)
	if f.peek().Type == lexer.TokenColon {
		f.space()
		f.attach(":")
		f.advance()
	}
}

// parseMemberHeader consumes access modifiers, name and optional
// `: return_type` after a METHOD / PROPERTY / ACTION keyword.
func (f *printer) parseMemberHeader(kind lexer.TokenType) {
	// Access modifiers come after the keyword: METHOD PRIVATE Foo
modifiers:
	for {
		q := f.peek()
		switch q.Type {
		case lexer.TokenPublic, lexer.TokenPrivate, lexer.TokenProtected, lexer.TokenInternal:
			f.attach(q.Literal)
			f.advance()
			f.space()
		case lexer.TokenAbstract, lexer.TokenFinal:
			f.attach(q.Literal)
			f.advance()
			f.space()
		default:
			break modifiers
		}
	}
	if n := f.peek(); n.Type == lexer.TokenIdent {
		f.attach(n.Literal)
		f.advance()
	}
	// METHOD / PROPERTY may have an explicit return type: NAME : TYPE
	colon := f.peek()
	if colon.Type == lexer.TokenColon && (kind == lexer.TokenMethod || kind == lexer.TokenProperty) {
		f.advance()
		f.space()
		f.attach(":")
		f.space()
		if rt := f.peek(); rt.Type != lexer.TokenEOF {
			f.attach(keywordSpelling(rt))
			f.advance()
		}
	}
}

func (f *printer) popUntil(kind blockKind) {
	for len(f.blocks) > 0 {
		b := f.pop()
		if b == kind {
			break
		}
	}
}

// handleColon handles ':' in its semantic contexts.
func (f *printer) handleColon() {
	switch f.top() {
	case bCase, bCaseLabel:
		f.attach(":")
		for f.top() == bCaseLabel {
			f.pop()
		}
		f.push(bCaseLabel)
		f.requestNL()
	default:
		if f.isLabel() {
			f.attach(":")
			return
		}
		f.space()
		f.attach(":")
		f.space()
	}
}

// isLabel reports whether the ':' at the current position ends a jump label
// (`lbl: x := 1;`): it follows a lone identifier at the start of a statement
// inside a POU body, where a declaration cannot stand.
func (f *printer) isLabel() bool {
	switch f.top() {
	case bVar, bType, bStruct:
		return false
	}
	if len(f.blocks) == 0 || f.pos < 2 || f.tokens[f.pos-1].Type != lexer.TokenIdent {
		return false
	}
	switch before := f.tokens[f.pos-2].Type; before {
	case lexer.TokenSemicolon, lexer.TokenThen, lexer.TokenDo, lexer.TokenElse, lexer.TokenRepeat,
		lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenPragma:
		return true
	default:
		return lexer.IsBlockEnd(before)
	}
}

// handleLParen writes '(' without a preceding space in call contexts.
func (f *printer) handleLParen() {
	prev := f.prevType()
	switch prev {
	case lexer.TokenIdent, lexer.TokenRParen, lexer.TokenRBracket, lexer.TokenNot:
		f.attach("(")
	case lexer.TokenNumber:
		// A repetition in an array initialiser: [10(0)].
		f.attach("(")
	default:
		if lexer.IsScalarTypeToken(prev) {
			f.attach("(")
		} else if f.lineStart || f.lastCh == ' ' {
			f.attach("(")
		} else {
			f.space()
			f.attach("(")
		}
	}
}

// emitCaseLabel writes a case label token at line start inside a CASE block.
// Returns true if the token was handled as a label. The first label after OF
// starts on the next line; any later label is preceded by a blank line.
func (f *printer) emitCaseLabel(lit string) bool {
	if f.top() != bCase && f.top() != bCaseLabel {
		return false
	}
	if !f.lineStart && !f.pendingNL && !f.startsWithNewStatement() {
		return false
	}
	if !f.looksLikeCaseLabel() {
		return false
	}
	if !f.lineStart {
		if f.top() == bCaseLabel {
			f.requestBlank()
		} else {
			f.requestNL()
		}
	}
	for f.top() == bCaseLabel {
		f.pop()
	}
	f.attach(lit)
	return true
}

// handleIdent decides whether an identifier begins a new statement line.
func (f *printer) handleIdent(tok lexer.Token) {
	if f.emitCaseLabel(tok.Literal) {
		return
	}

	f.flush()
	if f.lineStart {
		f.attach(tok.Literal)
		return
	}
	if f.startsWithNewStatement() {
		f.requestNL()
		f.flush()
		f.attach(tok.Literal)
		return
	}
	f.space()
	f.attach(tok.Literal)
}

// startsWithNewStatement reports whether the previous token ends a statement
// or block header, so the current identifier starts a new line.
func (f *printer) startsWithNewStatement() bool {
	switch f.prevType() {
	case lexer.TokenSemicolon, lexer.TokenThen, lexer.TokenDo, lexer.TokenRepeat,
		lexer.TokenElse:
		return true
	}
	// Any END_* keyword also ends at least one statement.
	if lexer.IsBlockEnd(f.prevType()) {
		return true
	}
	return false
}

// looksLikeCaseLabel reports whether tokens from the current position form
// a CASE label list (constants/ranges ending in ':').
func (f *printer) looksLikeCaseLabel() bool {
	i := f.pos
	for i < len(f.tokens) {
		t := f.tokens[i]
		switch t.Type {
		case lexer.TokenNumber, lexer.TokenBasedNumber, lexer.TokenIdent,
			lexer.TokenTrue, lexer.TokenFalse, lexer.TokenComma, lexer.TokenDotDot,
			lexer.TokenDot, lexer.TokenMinus:
			i++
		case lexer.TokenColon:
			return true
		default:
			return false
		}
	}
	return false
}

// tokenSpelling returns the text to emit for a token: the uppercase keyword
// spelling where there is one, the canonical spelling of a two-character
// operator, or the token's own literal.
func tokenSpelling(tok lexer.Token) string {
	if s := keywordSpelling(tok); s != "" {
		return s
	}
	switch tok.Type {
	case lexer.TokenAssign:
		return ":="
	case lexer.TokenOutputBind:
		return "=>"
	case lexer.TokenNotEqual:
		return "<>"
	case lexer.TokenLessEq:
		return "<="
	case lexer.TokenGreaterEq:
		return ">="
	case lexer.TokenPower:
		return "**"
	case lexer.TokenDotDot:
		return ".."
	}
	return tok.Literal
}

// keywordSpelling returns the spelling to emit for a keyword token. For the
// date/time type names that have an accepted abbreviation (TOD, LTOD, DT,
// LDT) the written form is preserved (uppercased) instead of expanding the
// alias to the canonical TIME_OF_DAY / DATE_AND_TIME spelling. For
// non-keyword tokens it returns tok.Literal.
func keywordSpelling(tok lexer.Token) string {
	switch tok.Type {
	case lexer.TokenTodType, lexer.TokenLtodType, lexer.TokenDtType, lexer.TokenLdtType:
		return strings.ToUpper(tok.Literal)
	}
	if s := lexer.KeywordSpelling(tok.Type); s != "" {
		return s
	}
	return tok.Literal
}

// handleAtom writes a numeric/literal token with appropriate spacing.
// Inside a CASE body a number at line start is treated as a label.
// If kw is non-empty it is used instead of tok.Literal (for keyword
// normalisation to UPPERCASE).
func (f *printer) handleAtom(tok lexer.Token, kw string) {
	lit := tok.Literal
	if kw != "" {
		lit = kw
	}

	if f.emitCaseLabel(lit) {
		return
	}

	f.flush()
	if f.lineStart {
		f.attach(lit)
		return
	}
	if f.startsWithNewStatement() {
		f.requestNL()
		f.flush()
		f.attach(lit)
		return
	}
	f.space()
	f.attach(lit)
}

// emitComment emits a comment. A comment that trails code in the source stays
// at the end of that line (two spaces separate it from the code); any other
// comment gets a line of its own. Directives keep their original spelling.
func (f *printer) emitComment(tok lexer.Token) {
	lit := tok.Literal
	if parseDirective(lit) == "" {
		lit = normalizeComment(lit)
	}
	if f.commentTrailsCode(tok) && !strings.Contains(lit, "\n") {
		f.pendingNL = false
		f.pendingBlank = false
		f.writeCh(' ')
		f.writeCh(' ')
	} else {
		// The comment stands on its own line in the source, so it keeps a line
		// of its own here too. The token before it may not have asked for one:
		// END_PROGRAM ends no statement, and a keyword that starts with a
		// space (PUBLIC) swallows the break that follows a POU header.
		f.startOwnLine()
	}
	f.attach(lit)
	f.requestNL()
	f.advance()
}

// commentTrailsCode reports whether the comment token follows code on the same
// source line.
func (f *printer) commentTrailsCode(tok lexer.Token) bool {
	return trailsCode(f.source, tok.Offset)
}

// normalizeComment puts a space between the comment opener and text that is
// glued to it ("//note" becomes "// note"). Nothing else in a comment is
// touched: its case, its indentation and its punctuation are content, so a
// banner such as "(*****)", a "///" doc comment and commented-out code all
// keep their exact text. A line comment holding only blanks is trimmed to
// "//". Pragmas (opening with "{") are left unchanged. Line endings need no
// attention here: the formatter works on LF internally and restores the ending
// of the input at the end.
func normalizeComment(lit string) string {
	switch {
	case strings.HasPrefix(lit, "//"):
		body := lit[2:]
		if trimBlank(body) == "" {
			return "//"
		}
		if startsWord(body) {
			return "// " + body
		}
	case strings.HasPrefix(lit, "(*") && strings.HasSuffix(lit, "*)") && len(lit) > 4:
		if startsWord(lit[2:]) {
			return "(* " + lit[2:]
		}
	}
	return lit
}

// startsWord reports whether s begins with a letter or a digit, ASCII or not.
func startsWord(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return (isIdentChar(c) && c != '_') || c >= utf8.RuneSelf
}
