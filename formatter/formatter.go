package formatter

import (
	"fmt"
	"slices"
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
	bNamespace
	bConfig
	bResource
	bStep
	bTransition
)

// blockNames names the blocks in error messages, by the keyword that opens
// them.
var blockNames = [...]string{
	bPOU:        "POU",
	bVar:        "VAR",
	bType:       "TYPE",
	bStruct:     "STRUCT",
	bIf:         "IF",
	bFor:        "FOR",
	bWhile:      "WHILE",
	bRepeat:     "REPEAT",
	bCase:       "CASE",
	bCaseLabel:  "CASE label",
	bNamespace:  "NAMESPACE",
	bConfig:     "CONFIGURATION",
	bResource:   "RESOURCE",
	bStep:       "STEP",
	bTransition: "TRANSITION",
}

func (b blockKind) String() string { return blockNames[b] }

// indents reports whether the body of a block is indented. A POU's statements
// sit at its keyword's column, and a namespace holds POUs at the top level.
func (b blockKind) indents() bool {
	return b != bPOU && b != bNamespace
}

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
	// blockAt holds, for each open block, the source offset of the token
	// that opened it, so an error can name the line of an unclosed block.
	blockAt []int
	// brackets holds the open '(' and '[' with their source offsets.
	brackets []openBracket
	// openers maps the index of an identifier that opens a block (NAMESPACE,
	// STEP, GET, ...) to that block; see findOpeners.
	openers map[int]blockKind
	// checked is set when the structure is checked: not for a file with
	// ignored regions, whose tokens are partly skipped. err holds the first
	// problem found.
	checked bool
	err     error
	// depth is the indentation level: the number of open blocks that indent
	// their body. It is maintained by push/pop instead of being recounted
	// from the stack on every line break.
	depth     int
	buf       []byte
	lastCh    byte
	prevCh    byte
	lineStart bool
	pendingNL bool

	pendingBlank bool
	// lineBegin is where the current output line starts in buf, and spans
	// are the tokens written on it so far, which endLine lays out once the
	// line is complete. frozen marks a line that holds verbatim text and is
	// kept as it is.
	lineBegin int
	spans     []span
	frozen    bool
	// carry is how many brackets earlier lines left open.
	carry int
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
	f.buf = make([]byte, 0, len(text)+len(text)/8)
	if all, runs := scanDirectives(f.tokens); !all {
		f.runs = runs
		f.checked = len(runs) == 0
		f.openers = findOpeners(f.tokens)
	} else {
		// A file-level "stformat:ignore" leaves the file byte-for-byte alone.
		f.ignoreAll = true
	}
	return f
}

// Format formats the source, keeping the line ending it already uses.
//
// It returns an error, and no output, when the source is not well formed (a
// block or bracket that is opened and never closed, or closed and never
// opened; reported as a *SyntaxError) or when the output would not hold the
// same tokens as the source, which Verify describes. A file with ignored
// regions is not checked for structure, since part of it is never read.
func Format(source string) (string, error) {
	return format(source, LineEnding(source))
}

// FormatWith is Format writing every line break as ending. Use it when the
// line ending must follow something other than the text being formatted, such
// as the rest of an XML file whose CDATA section holds the code. An ending
// other than "\r\n" or "\n" falls back to the one found in source.
func FormatWith(source, ending string) (string, error) {
	if ending != "\r\n" && ending != "\n" {
		ending = LineEnding(source)
	}
	return format(source, ending)
}

func format(source, ending string) (string, error) {
	f := newPrinter(source, ending)
	out := f.run()
	if f.ignoreAll {
		return out, nil
	}
	if f.err != nil {
		return "", f.err
	}
	// The source tokens are at hand, so only the output is lexed again.
	text := toLF(out)
	if f.bom {
		text = strings.TrimPrefix(text, utf8BOM)
	}
	if err := compareTokens(f.tokens, lexer.Lex(text)); err != nil {
		return "", err
	}
	return out, nil
}

// SyntaxError reports source that is not well formed enough to format.
type SyntaxError struct {
	Line int // line of the offending token, counted from 1
	Msg  string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// fail records a structural problem at a source offset. Only the first one is
// kept, and nothing is recorded when the structure is not checked.
func (f *printer) fail(offset int, format string, args ...any) {
	if !f.checked || f.err != nil {
		return
	}
	f.err = &SyntaxError{
		Line: f.line(offset),
		Msg:  fmt.Sprintf(format, args...),
	}
}

// line returns the source line, counted from 1, of an offset.
func (f *printer) line(offset int) int {
	return 1 + strings.Count(f.source[:offset], "\n")
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
	f.checkClosed()
	// A break still pending at the end would only write indentation. The file
	// ends with a single line break, unless the input ends inside a token
	// that never closes: a break written now would become part of its text.
	f.pendingNL, f.pendingBlank = false, false
	f.endLine()
	if f.lastCh != '\n' && !endsOpen(f.source, f.tokens) {
		f.writeCh('\n')
	}
	out := string(f.buf)
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
	f.frozen = true
	f.writeRaw(text)
	if f.lastCh == '\n' {
		// The verbatim text ends its own line; indent what follows.
		f.endLine()
		f.lineBegin = len(f.buf)
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
	f.buf = append(f.buf, c)
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
		f.newline()
	}
	if blank {
		f.newline()
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
	// Every token is written while it is the current one, so the position
	// says which token of the input the text is.
	start := len(f.buf)
	f.writeRaw(s)
	f.spans = append(f.spans, span{start: start, end: len(f.buf), tok: f.pos})
	f.lastLit = s
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
	out := f.buf
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

func isIdentChar(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || isDigit(b)
}

// trimBlank removes the spaces and tabs around s. strings.TrimSpace would also
// remove characters such as a form feed, which are tokens of the source that
// the pass must keep.
func trimBlank(s string) string {
	return strings.Trim(s, " \t")
}

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
	f.buf = append(f.buf, s[:n-tailTrackedLen]...)
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

// push opens a block. A block that does not indent its body (a POU, a
// namespace) does not raise the depth. The block is taken to be opened by the
// token just consumed, which is where an error about it points.
func (f *printer) push(b blockKind) {
	f.blocks = append(f.blocks, b)
	f.blockAt = append(f.blockAt, f.tokens[max(f.pos-1, 0)].Offset)
	if b.indents() {
		f.depth++
	}
}

func (f *printer) pop() blockKind {
	if len(f.blocks) == 0 {
		return bPOU
	}
	b := f.blocks[len(f.blocks)-1]
	f.blocks = f.blocks[:len(f.blocks)-1]
	f.blockAt = f.blockAt[:len(f.blockAt)-1]
	if b.indents() {
		f.depth--
	}
	return b
}

// openBracket is a '(' or '[' waiting for its closing bracket.
type openBracket struct {
	ch     byte
	offset int
}

// openBracketAt records a '(' or '[' at the current token.
func (f *printer) openBracketAt(ch byte) {
	f.brackets = append(f.brackets, openBracket{ch, f.peek().Offset})
}

// closeBracketAt matches a ')' or ']' at the current token against the
// innermost open bracket.
func (f *printer) closeBracketAt(ch byte) {
	want := byte('(')
	if ch == ']' {
		want = '['
	}
	n := len(f.brackets)
	switch {
	case n == 0:
		f.fail(f.peek().Offset, "%c without a matching %c", ch, want)
	case f.brackets[n-1].ch != want:
		f.fail(f.peek().Offset, "%c closes the %c from line %d", ch, f.brackets[n-1].ch, f.line(f.brackets[n-1].offset))
		f.brackets = f.brackets[:n-1]
	default:
		f.brackets = f.brackets[:n-1]
	}
}

// checkClosed reports a block or bracket still open at the end of the input.
// A POU may stay open: TwinCAT stores a POU's declaration without its END_
// keyword, and a namespace may hold such POUs.
func (f *printer) checkClosed() {
	for i, b := range f.blocks {
		if b != bPOU && b != bNamespace && b != bCaseLabel {
			f.fail(f.blockAt[i], "%s is never closed", b)
			return
		}
	}
	if len(f.brackets) > 0 {
		b := f.brackets[0]
		f.fail(b.offset, "%c is never closed", b.ch)
	}
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
	return f.afterOperandAt(f.pos)
}

// afterOperandAt is afterOperand for the token at index at.
func (f *printer) afterOperandAt(at int) bool {
	for i := at - 1; i >= 0; i-- {
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

	if kind, ok := f.openers[f.pos]; ok {
		f.emitOpener(kind)
		return
	}

	lit := tokenSpelling(tok)

	switch tok.Type {
	// ------- POU declarations -------
	case lexer.TokenProgram, lexer.TokenFunction, lexer.TokenFunctionBlock,
		lexer.TokenClass, lexer.TokenInterface:
		if tok.Type == lexer.TokenProgram && (f.top() == bConfig || f.top() == bResource) {
			// A program instance in a configuration (PROGRAM inst WITH task :
			// Prog;) is a declaration, not a POU.
			f.startOwnLine()
			f.attach(lit)
			f.advance()
			return
		}
		f.blankLineBeforeTop()
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.parseHeader()
		f.push(bPOU)
		f.requestNL()

	// METHOD / PROPERTY / ACTION are POU-like but appear inside an
	// enclosing POU, so they share the enclosing block's indent and no
	// blank line.
	case lexer.TokenMethod, lexer.TokenProperty, lexer.TokenAction:
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.parseHeader()
		f.push(bPOU)
		f.requestNL()

	case lexer.TokenEndProgram, lexer.TokenEndFunction, lexer.TokenEndFunctionBlock,
		lexer.TokenEndAction, lexer.TokenEndMethod, lexer.TokenEndProperty,
		lexer.TokenEndClass, lexer.TokenEndInterface, lexer.TokenEndGet, lexer.TokenEndSet:
		f.popUntil(bPOU)
		f.startOwnLine()
		f.attach(lit)
		f.advance()

	// ------- Namespaces, configurations and SFC -------
	case lexer.TokenInitialStep:
		f.emitOpener(bStep)

	case lexer.TokenEndNamespace, lexer.TokenEndConfiguration, lexer.TokenEndResource,
		lexer.TokenEndStep, lexer.TokenEndTransition:
		f.popUntil(closedBy[tok.Type])
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()

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
		lexer.TokenVarInst, lexer.TokenVarAccess, lexer.TokenVarGeneric:
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
		if f.top() == bCaseLabel || f.top() == bCase {
			// The ELSE branch of a CASE, after its labels or in place of them.
			if f.top() == bCaseLabel {
				f.pop()
			}
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
		f.openBracketAt('(')
		f.handleLParen()
		f.advance()

	case lexer.TokenRParen:
		f.closeBracketAt(')')
		f.attach(")")
		f.advance()

	case lexer.TokenLBracket:
		f.openBracketAt('[')
		f.attach("[")
		f.advance()

	case lexer.TokenRBracket:
		f.closeBracketAt(']')
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
	if len(f.buf) == 0 || slices.ContainsFunc(f.blocks, func(b blockKind) bool { return b != bNamespace }) {
		return
	}
	f.flush()
	if f.lastCh != '\n' {
		f.newline()
	}
	if f.lastCh != '\n' || f.prevCh != '\n' {
		f.newline()
	}
	for range f.depth {
		f.writeCh('\t')
	}
	f.lineStart = true
}

// parseHeader writes what follows a POU or member keyword (FUNCTION_BLOCK,
// METHOD, ...): access and inheritance modifiers, the name, a return type after
// ':' and EXTENDS / IMPLEMENTS lists, in whatever order the source has them. It
// stops at the first token that is none of these (a second name included), so
// the first VAR block or statement starts on a line of its own.
func (f *printer) parseHeader() {
	named := false
	for {
		tok := f.peek()
		switch tok.Type {
		case lexer.TokenPublic, lexer.TokenPrivate, lexer.TokenProtected, lexer.TokenInternal,
			lexer.TokenAbstract, lexer.TokenFinal:
			f.space()
			f.attach(tokenSpelling(tok))
			f.advance()
		case lexer.TokenIdent:
			if named {
				return
			}
			named = true
			f.space()
			f.attachName()
		case lexer.TokenColon:
			f.space()
			f.attach(":")
			f.advance()
			f.parseType()
		case lexer.TokenExtends, lexer.TokenImplements:
			f.space()
			f.attach(tokenSpelling(tok))
			f.advance()
			f.parseNameList()
		default:
			return
		}
	}
}

// parseType writes a type: a name (INT, Ns.ST_Data, STRING(80), STRING[80]),
// POINTER TO, REFERENCE TO or REF_TO a type, or ARRAY[...] OF a type.
func (f *printer) parseType() {
	tok := f.peek()
	switch {
	case tok.Type == lexer.TokenPointer || tok.Type == lexer.TokenReference:
		f.space()
		f.attach(tokenSpelling(tok))
		f.advance()
		if f.peek().Type == lexer.TokenTo {
			f.space()
			f.attach("TO")
			f.advance()
		}
		f.parseType()
	case tok.Type == lexer.TokenRefTo:
		f.space()
		f.attach(tokenSpelling(tok))
		f.advance()
		f.parseType()
	case tok.Type == lexer.TokenArray:
		f.space()
		f.attach(tokenSpelling(tok))
		f.advance()
		if f.peek().Type == lexer.TokenLBracket {
			f.parseGroup()
		}
		if f.peek().Type == lexer.TokenOf {
			f.space()
			f.attach("OF")
			f.advance()
			f.parseType()
		}
	case tok.Type == lexer.TokenIdent || lexer.IsScalarTypeToken(tok.Type):
		f.space()
		f.attachName()
		if t := f.peek().Type; t == lexer.TokenLParen || t == lexer.TokenLBracket {
			f.parseGroup()
		}
	}
}

// attachName writes the name at the current position, with the members of a
// dotted name (Ns.ST_Data) behind it, and moves past it. A type keyword keeps
// the spelling keywordSpelling gives it.
func (f *printer) attachName() {
	f.attach(keywordSpelling(f.peek()))
	f.advance()
	for f.peek().Type == lexer.TokenDot && f.pos+1 < len(f.tokens) && f.tokens[f.pos+1].Type == lexer.TokenIdent {
		f.attach(".")
		f.advance()
		f.attach(f.peek().Literal)
		f.advance()
	}
}

// parseGroup writes a bracketed group ([1..10, 0..1] or (80)) token by token
// up to and including its closing bracket.
func (f *printer) parseGroup() {
	depth := 0
	for {
		tok := f.peek()
		switch tok.Type {
		case lexer.TokenEOF:
			return
		case lexer.TokenLParen, lexer.TokenLBracket:
			depth++
		case lexer.TokenRParen, lexer.TokenRBracket:
			depth--
		}
		f.emitToken()
		if depth == 0 {
			return
		}
	}
}

// parseNameList writes the comma separated names after EXTENDS or IMPLEMENTS.
func (f *printer) parseNameList() {
	for f.peek().Type == lexer.TokenIdent {
		f.space()
		f.attachName()
		if f.peek().Type != lexer.TokenComma {
			return
		}
		f.attach(",")
		f.advance()
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

// emitOpener writes a block opener that the keyword switch does not handle
// (INITIAL_STEP, or a contextual opener such as NAMESPACE, STEP or GET) with
// the rest of its header line, and opens its block.
func (f *printer) emitOpener(kind blockKind) {
	tok := f.peek()
	if kind == bNamespace || kind == bConfig {
		f.blankLineBeforeTop()
	}
	f.startOwnLine()
	// A contextual opener is an identifier, so it keeps its spelling.
	f.attach(tokenSpelling(tok))
	f.advance()
	f.restOfHeader(kind)
	f.push(kind)
	f.requestNL()
}

// restOfHeader writes the header that follows a block opener: the name of a
// NAMESPACE, CONFIGURATION or STEP (with its ':'), the name and ON target of a
// RESOURCE, or the FROM ... TO ... := condition; of a TRANSITION. An accessor
// (GET, SET) has none. The header is told by its tokens, never by where the
// source breaks its lines: a long header line may be broken by the layout, and
// formatting the result again must find the same header.
func (f *printer) restOfHeader(kind blockKind) {
	switch kind {
	case bPOU:
		return
	case bTransition:
		for {
			tok := f.peek()
			if !headerToken(tok.Type) || f.opensBlock() {
				return
			}
			f.emitToken()
			if tok.Type == lexer.TokenSemicolon {
				return
			}
		}
	}
	if f.peek().Type != lexer.TokenIdent || f.opensBlock() {
		return
	}
	f.space()
	f.attachName()
	switch kind {
	case bStep:
		// STEP s1: — the colon belongs to the step name; the step's actions
		// follow in its body.
		if f.peek().Type == lexer.TokenColon {
			f.attach(":")
			f.advance()
		}
	case bResource:
		// RESOURCE r ON target
		if t := f.peek(); t.Type == lexer.TokenIdent && strings.EqualFold(t.Literal, "ON") &&
			f.pos+1 < len(f.tokens) && f.tokens[f.pos+1].Type == lexer.TokenIdent {
			f.space()
			f.attach(t.Literal)
			f.advance()
			f.space()
			f.attachName()
		}
	}
}

// opensBlock reports whether the token at the current position opens a block
// of its own, so that it is not part of the header before it.
func (f *printer) opensBlock() bool {
	_, ok := f.openers[f.pos]
	return ok
}

// headerToken reports whether a token can be part of a block opener's header
// line: anything but a comment, the end of input, or a keyword that has a
// layout of its own (a block opener or closer, VAR, ...). Operators spelled as
// keywords, TO and type names are fine.
func headerToken(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenEOF, lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenPragma:
		return false
	case lexer.TokenTo, lexer.TokenTrue, lexer.TokenFalse, lexer.TokenAnd, lexer.TokenOr,
		lexer.TokenXor, lexer.TokenNot, lexer.TokenMod, lexer.TokenAndThen, lexer.TokenOrElse:
		return true
	}
	return lexer.KeywordSpelling(tt) == "" || lexer.IsScalarTypeToken(tt)
}

// contextualOpener is a block opener that the lexer leaves an identifier,
// because the word is common as a name in PLC code (a Step counter, a Set
// method). It only opens a block where findOpeners finds it closed.
type contextualOpener struct {
	word string          // uppercase spelling
	end  lexer.TokenType // the keyword that closes the block
	kind blockKind
}

var contextualOpeners = []contextualOpener{
	{"NAMESPACE", lexer.TokenEndNamespace, bNamespace},
	{"CONFIGURATION", lexer.TokenEndConfiguration, bConfig},
	{"RESOURCE", lexer.TokenEndResource, bResource},
	{"STEP", lexer.TokenEndStep, bStep},
	{"TRANSITION", lexer.TokenEndTransition, bTransition},
	{"GET", lexer.TokenEndGet, bPOU},
	{"SET", lexer.TokenEndSet, bPOU},
}

// closedBy maps a closing keyword to the block it closes, for the blocks that
// the keyword switch closes through it.
var closedBy = map[lexer.TokenType]blockKind{
	lexer.TokenEndNamespace:     bNamespace,
	lexer.TokenEndConfiguration: bConfig,
	lexer.TokenEndResource:      bResource,
	lexer.TokenEndStep:          bStep,
	lexer.TokenEndTransition:    bTransition,
}

// lookupOpener returns the contextual opener an identifier is spelled like.
func lookupOpener(lit string) (contextualOpener, bool) {
	for _, o := range contextualOpeners {
		if len(lit) == len(o.word) && strings.EqualFold(lit, o.word) {
			return o, true
		}
	}
	return contextualOpener{}, false
}

// findOpeners returns the identifiers that open a block, by token index. Such
// an identifier is spelled like a contextual opener, starts a statement, is
// not used as a value (assigned, called, indexed, dereferenced or followed
// by ';') and is closed by its keyword further on. Openers of one kind nest, so
// each closing keyword pairs with the nearest open candidate before it.
func findOpeners(toks []lexer.Token) map[int]blockKind {
	var openers map[int]blockKind
	var pending map[lexer.TokenType][]int
	for i, tok := range toks {
		if tok.Type == lexer.TokenIdent {
			o, ok := lookupOpener(tok.Literal)
			if ok && startsStatement(toks, i) && !usedAsValue(toks, i, o.kind == bTransition) {
				if pending == nil {
					pending = make(map[lexer.TokenType][]int)
				}
				pending[o.end] = append(pending[o.end], i)
			}
			continue
		}
		stack := pending[tok.Type]
		if len(stack) == 0 {
			continue
		}
		at := stack[len(stack)-1]
		pending[tok.Type] = stack[:len(stack)-1]
		if openers == nil {
			openers = make(map[int]blockKind)
		}
		o, _ := lookupOpener(toks[at].Literal)
		openers[at] = o.kind
	}
	return openers
}

// startsStatement reports whether the identifier at index i comes where a
// statement or declaration starts: at the start of the input or after a ';',
// THEN, DO, ELSE, REPEAT or END_ keyword, or right after a POU or member
// header (PROGRAM Chart, then STEP; PROPERTY P : INT, then GET). It looks only
// at tokens, never at line breaks, so formatting cannot change its answer.
func startsStatement(toks []lexer.Token, i int) bool {
	j := i - 1
	for j >= 0 && lexer.IsComment(toks[j].Type) {
		j--
	}
	if j < 0 {
		return true
	}
	switch tt := toks[j].Type; tt {
	case lexer.TokenSemicolon, lexer.TokenThen, lexer.TokenDo, lexer.TokenElse, lexer.TokenRepeat:
		return true
	default:
		if lexer.IsBlockEnd(tt) {
			return true
		}
	}
	// A header: the keyword, modifiers, a name, a return type (a dotted name,
	// POINTER TO / REFERENCE TO a type, or ARRAY[...] OF a type) and
	// EXTENDS / IMPLEMENTS lists.
	for ; j >= 0; j-- {
		switch toks[j].Type {
		case lexer.TokenProgram, lexer.TokenFunction, lexer.TokenFunctionBlock, lexer.TokenClass,
			lexer.TokenInterface, lexer.TokenMethod, lexer.TokenProperty, lexer.TokenAction:
			return true
		case lexer.TokenExtends, lexer.TokenImplements,
			lexer.TokenIdent, lexer.TokenColon, lexer.TokenDot, lexer.TokenComma,
			lexer.TokenNumber, lexer.TokenDotDot, lexer.TokenLParen, lexer.TokenRParen,
			lexer.TokenLBracket, lexer.TokenRBracket, lexer.TokenPointer, lexer.TokenReference,
			lexer.TokenRefTo, lexer.TokenTo, lexer.TokenArray, lexer.TokenOf,
			lexer.TokenPublic, lexer.TokenPrivate, lexer.TokenProtected, lexer.TokenInternal,
			lexer.TokenAbstract, lexer.TokenFinal,
			lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenPragma:
		default:
			if !lexer.IsScalarTypeToken(toks[j].Type) {
				return false
			}
		}
	}
	return false
}

// usedAsValue reports whether the identifier at index i is used as a variable
// or called, judging by the token after it. A TRANSITION may be followed by
// its (PRIORITY := n).
func usedAsValue(toks []lexer.Token, i int, parenOK bool) bool {
	for _, t := range toks[i+1:] {
		switch t.Type {
		case lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenPragma:
			continue
		case lexer.TokenLParen:
			return !parenOK
		case lexer.TokenAssign, lexer.TokenSAssign, lexer.TokenRAssign, lexer.TokenRefAssign,
			lexer.TokenOutputBind, lexer.TokenDot, lexer.TokenLBracket, lexer.TokenCaret,
			lexer.TokenComma, lexer.TokenSemicolon, lexer.TokenColon, lexer.TokenRParen,
			lexer.TokenEqual, lexer.TokenNotEqual, lexer.TokenLess, lexer.TokenLessEq,
			lexer.TokenGreater, lexer.TokenGreaterEq, lexer.TokenPlus, lexer.TokenMinus,
			lexer.TokenStar, lexer.TokenSlash, lexer.TokenAnd, lexer.TokenOr, lexer.TokenXor:
			return true
		}
		return false
	}
	return false
}

// popUntil closes the innermost open block of the given kind, which the
// current token ends, together with any block opened inside it. A block left
// open inside it, or no such block at all, is a structural error; layout
// carries on either way.
func (f *printer) popUntil(kind blockKind) {
	tok := f.peek()
	for len(f.blocks) > 0 {
		at := f.blockAt[len(f.blockAt)-1]
		b := f.pop()
		if b == kind {
			return
		}
		if b != bCaseLabel {
			f.fail(tok.Offset, "%s while the %s from line %d is still open", tokenSpelling(tok), b, f.line(at))
		}
	}
	f.fail(tok.Offset, "%s without a matching %s", tokenSpelling(tok), kind)
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
