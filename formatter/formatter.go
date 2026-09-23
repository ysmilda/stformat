package formatter

import (
	"strings"

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

// Formatter formats a token stream into properly formatted ST source code.
// It uses an explicit block stack to compute indentation and a
// token-driven state machine to decide line breaks and spacing.
// Line breaks are deferred ("pending") so that indentation changes that
// apply to the next token are reflected before the line break is rendered.
type Formatter struct {
	tokens       []lexer.Token
	pos          int
	blocks       []blockKind
	buf          strings.Builder
	lastCh       byte
	prevCh       byte
	lineStart    bool
	pendingNL    bool
	pendingBlank bool
	loopKw       lexer.TokenType // set by FOR/WHILE openers, consumed by DO
	caseSeen     bool            // set by CASE opener, consumed by OF
	maxLine      int
}

// New creates a formatter from a token stream.
func New(tokens []lexer.Token) *Formatter {
	return &Formatter{
		tokens:    tokens,
		lastCh:    '\n',
		lineStart: true,
		maxLine:   120,
	}
}

// Format lexes the source and formats it.
func Format(source string) string {
	tokens := lexer.Lex(source)
	f := New(tokens)
	return f.Run()
}

// FormatWithCheck formats and returns whether it changed.
func FormatWithCheck(source string) (string, bool) {
	result := Format(source)
	return result, result != source
}

// Run executes the formatting pass and returns the output string.
func (f *Formatter) Run() string {
	for {
		tok := f.peek()
		if tok.Type == lexer.TokenEOF {
			break
		}
		if !f.emitToken() {
			f.pos++
		}
	}
	f.flush()
	if f.lastCh != '\n' {
		f.writeCh('\n')
	}
	return PostProcess(f.buf.String())
}

func (f *Formatter) peek() lexer.Token {
	if f.pos >= len(f.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return f.tokens[f.pos]
}

func (f *Formatter) advance() lexer.Token {
	tok := f.peek()
	f.pos++
	return tok
}

// ------------- output primitives -------------

func (f *Formatter) writeCh(c byte) {
	f.prevCh = f.lastCh
	f.buf.WriteByte(c)
	f.lastCh = c
	f.lineStart = c == '\n'
}

func (f *Formatter) indent() int {
	// POU-like blocks (FUNCTION/FUNCTION_BLOCK/PROGRAM/METHOD/...) do not
	// indent their bodies: body statements sit at the POU keyword's column.
	n := 0
	for _, b := range f.blocks {
		if b != bPOU {
			n++
		}
	}
	return n
}

// requestNL marks that the next content should begin on a new line.
// The actual newline is rendered by flush() using the then-current indent.
func (f *Formatter) requestNL() {
	f.pendingNL = true
}

// requestBlank marks that the next content should begin on a new line
// preceded by a blank line.
func (f *Formatter) requestBlank() {
	f.requestNL()
	f.pendingBlank = true
}

// flush renders a pending line break at the current indentation.
func (f *Formatter) flush() {
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
	for range f.indent() {
		f.writeCh('\t')
	}
	f.lineStart = true
}

// attach writes a string, flushing any pending line break first,
// without adding any leading whitespace.
func (f *Formatter) attach(s string) {
	f.flush()
	for i := range len(s) {
		f.writeCh(s[i])
	}
}

// space writes a single space unless at line start or already spaced.
func (f *Formatter) space() {
	f.flush()
	if f.lineStart {
		return
	}
	switch f.lastCh {
	case ' ', '\t', '(', '[', '.', '\n':
		return
	}
	f.writeCh(' ')
}

// startOwnLine ensures a new line will begin at the current position.
func (f *Formatter) startOwnLine() {
	if !f.lineStart {
		f.requestNL()
	}
}

func (f *Formatter) push(b blockKind) {
	f.blocks = append(f.blocks, b)
}

func (f *Formatter) pop() blockKind {
	if len(f.blocks) == 0 {
		return bPOU
	}
	b := f.blocks[len(f.blocks)-1]
	f.blocks = f.blocks[:len(f.blocks)-1]
	return b
}

func (f *Formatter) top() blockKind {
	if len(f.blocks) == 0 {
		return bPOU
	}
	return f.blocks[len(f.blocks)-1]
}

func (f *Formatter) prevType() lexer.TokenType {
	if f.pos == 0 {
		return lexer.TokenEOF
	}
	return f.tokens[f.pos-1].Type
}

// endBreak requests the break that follows a closed block: a blank line
// unless the next token closes another block or the source ends.
func (f *Formatter) endBreak() {
	next := f.peek().Type
	if next != lexer.TokenEOF && !lexer.IsBlockEnd(next) {
		f.requestBlank()
	} else {
		f.requestNL()
	}
}

// ------------- token emission -------------

// emitToken processes the token at the current position. It returns false
// if the token was not consumed (caller advances).
func (f *Formatter) emitToken() bool {
	tok := f.peek()

	if tok.Type == lexer.TokenLineComment || tok.Type == lexer.TokenBlockComment {
		f.emitComment(tok)
		return true
	}

	lit := tok.Literal
	if s := keywordSpelling(tok); s != "" {
		lit = s
	} else {
		switch tok.Type {
		case lexer.TokenAssign:
			lit = ":="
		case lexer.TokenOutputBind:
			lit = "=>"
		case lexer.TokenNotEqual:
			lit = "<>"
		case lexer.TokenLessEq:
			lit = "<="
		case lexer.TokenGreaterEq:
			lit = ">="
		case lexer.TokenPower:
			lit = "**"
		case lexer.TokenDotDot:
			lit = ".."
		case lexer.TokenSAssign:
			lit = "S="
		case lexer.TokenRAssign:
			lit = "R="
		case lexer.TokenRefAssign:
			lit = "REF="
		}
	}

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
		return true

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
		return true

	case lexer.TokenEndProgram, lexer.TokenEndFunction, lexer.TokenEndFunctionBlock,
		lexer.TokenEndAction, lexer.TokenEndMethod, lexer.TokenEndProperty,
		lexer.TokenEndClass, lexer.TokenEndInterface:
		f.popUntil(bPOU)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		return true

	// ------- TYPE blocks -------
	case lexer.TokenTypeKw:
		f.blankLineBeforeTop()
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.parseTypeHeader()
		f.push(bType)
		f.requestNL()
		return true

	case lexer.TokenEndType:
		f.popUntil(bType)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	// ------- VAR blocks -------
	case lexer.TokenVar, lexer.TokenVarInput, lexer.TokenVarOutput,
		lexer.TokenVarInOut, lexer.TokenVarTemp, lexer.TokenVarGlobal,
		lexer.TokenVarExternal, lexer.TokenVarConfig, lexer.TokenVarStat,
		lexer.TokenVarInst, lexer.TokenVarAccess:
		f.startOwnLine()
		f.attach(lit)
		f.advance()

		for {
			q := f.peek()
			switch q.Type {
			case lexer.TokenRetain, lexer.TokenConstant, lexer.TokenNonRetain, lexer.TokenPersistent:
				f.space()
				f.attach(keywordSpelling(q))
				f.advance()
			default:
				goto qualifiersDone
			}
		}
	qualifiersDone:
		f.push(bVar)
		f.requestNL()
		return true

	case lexer.TokenEndVar:
		f.popUntil(bVar)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

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
		return true

	case lexer.TokenEndStruct, lexer.TokenEndUnion:
		f.popUntil(bStruct)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

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
		return true

	case lexer.TokenExit, lexer.TokenContinue, lexer.TokenReturn:
		if !f.lineStart && !f.pendingNL {
			f.startOwnLine()
		}
		f.attach(lit)
		f.advance()
		return true

	case lexer.TokenThen:
		f.space()
		f.attach(lit)
		f.advance()
		f.push(bIf)
		f.requestNL()
		return true

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
		return true

	case lexer.TokenRepeat:
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.push(bRepeat)
		f.requestNL()
		return true

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
		return true

	// ------- Control flow continuations -------
	case lexer.TokenElsif:
		f.popUntil(bIf)
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()
		return true

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
		return true

	case lexer.TokenUntil:
		f.popUntil(bRepeat)
		f.startOwnLine()
		f.attach(lit)
		f.space()
		f.advance()
		return true

	// ------- Control flow closers -------
	case lexer.TokenEndIf:
		f.popUntil(bIf)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	case lexer.TokenEndFor:
		f.popUntil(bFor)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	case lexer.TokenEndWhile:
		f.popUntil(bWhile)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	case lexer.TokenEndRepeat:
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	case lexer.TokenEndCase:
		for f.top() == bCaseLabel {
			f.pop()
		}
		f.popUntil(bCase)
		f.startOwnLine()
		f.attach(lit)
		f.advance()
		f.endBreak()
		return true

	// ------- Keywords that continue a line -------
	case lexer.TokenTo, lexer.TokenBy, lexer.TokenAt:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()
		return true

	case lexer.TokenArray:
		f.space()
		f.attach(lit)
		f.advance()
		return true

	case lexer.TokenRetain, lexer.TokenConstant, lexer.TokenNonRetain, lexer.TokenPersistent:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()
		return true

	// ------- Operators -------
	case lexer.TokenAssign:
		f.space()
		f.attach(":=")
		f.space()
		f.advance()
		return true

	case lexer.TokenOutputBind:
		f.space()
		f.attach("=>")
		f.space()
		f.advance()
		return true

	case lexer.TokenSAssign, lexer.TokenRAssign, lexer.TokenRefAssign:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()
		return true

	case lexer.TokenEqual, lexer.TokenNotEqual, lexer.TokenLess, lexer.TokenLessEq,
		lexer.TokenGreater, lexer.TokenGreaterEq, lexer.TokenPlus, lexer.TokenMinus,
		lexer.TokenStar, lexer.TokenSlash, lexer.TokenPower, lexer.TokenMod,
		lexer.TokenAnd, lexer.TokenOr, lexer.TokenXor, lexer.TokenAndThen,
		lexer.TokenOrElse:
		f.space()
		f.attach(lit)
		f.space()
		f.advance()
		return true

	case lexer.TokenNot:
		f.space()
		f.attach("NOT")
		f.space()
		f.advance()
		return true

	// ------- Punctuation -------
	case lexer.TokenSemicolon:
		f.attach(";")
		f.advance()
		f.requestNL()
		return true

	case lexer.TokenComma:
		f.attach(",")
		f.space()
		f.advance()
		return true

	case lexer.TokenColon:
		f.handleColon()
		f.advance()
		return true

	case lexer.TokenDot:
		f.attach(".")
		f.advance()
		return true

	case lexer.TokenDotDot:
		f.attach("..")
		f.advance()
		return true

	case lexer.TokenLParen:
		f.handleLParen()
		f.advance()
		return true

	case lexer.TokenRParen:
		f.attach(")")
		f.advance()
		return true

	case lexer.TokenLBracket:
		f.attach("[")
		f.advance()
		return true

	case lexer.TokenRBracket:
		f.attach("]")
		f.advance()
		return true

	// ------- Identifiers and literals -------
	case lexer.TokenIdent:
		f.handleIdent(tok)
		f.advance()
		return true

	case lexer.TokenNumber, lexer.TokenBasedNumber, lexer.TokenString,
		lexer.TokenTimeLiteral, lexer.TokenTypedLiteral, lexer.TokenAddress:
		f.handleAtom(tok, "")
		f.advance()
		return true

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
		return true

	case lexer.TokenTrue, lexer.TokenFalse:
		f.handleAtom(tok, keywordSpelling(tok))
		f.advance()
		return true

	// ------- Misc keywords -------
	case lexer.TokenExtends, lexer.TokenImplements, lexer.TokenAbstract, lexer.TokenFinal,
		lexer.TokenInternal, lexer.TokenPrivate, lexer.TokenProtected, lexer.TokenPublic,
		lexer.TokenThis, lexer.TokenSuper,
		lexer.TokenReadOnly, lexer.TokenReadWrite, lexer.TokenParams:
		f.space()
		f.attach(lit)
		f.advance()
		return true

	default:
		f.attach(lit)
		f.advance()
		return true
	}
}

// ------------- specific handlers -------------

// blankLineBeforeTop inserts a blank line before a top-level declaration
// (POU, global VAR block, TYPE block) that follows other content.
func (f *Formatter) blankLineBeforeTop() {
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
	for range f.indent() {
		f.writeCh('\t')
	}
	f.lineStart = true
}

// parsePOUHeader consumes the POU name and optional return type /
// EXTENDS / IMPLEMENTS clauses, emitting the ':' before a FUNCTION return
// type. The caller has already emitted the keyword and any block keyword
// that precedes it.
func (f *Formatter) parsePOUHeader(kind lexer.TokenType) {
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
func (f *Formatter) parseTypeHeader() {
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
func (f *Formatter) parseMemberHeader(kind lexer.TokenType) {
	// Access modifiers come after the keyword: METHOD PRIVATE Foo
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
			goto modifiersDone
		}
	}
modifiersDone:
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

func (f *Formatter) popUntil(kind blockKind) {
	for len(f.blocks) > 0 {
		b := f.pop()
		if b == kind {
			break
		}
	}
}

// handleColon handles ':' in its semantic contexts.
func (f *Formatter) handleColon() {
	switch f.top() {
	case bCase, bCaseLabel:
		f.attach(":")
		for f.top() == bCaseLabel {
			f.pop()
		}
		f.push(bCaseLabel)
		f.requestNL()
	default:
		f.space()
		f.attach(":")
		f.space()
	}
}

// handleLParen writes '(' without a preceding space in call contexts.
func (f *Formatter) handleLParen() {
	prev := f.prevType()
	switch prev {
	case lexer.TokenIdent, lexer.TokenRParen, lexer.TokenRBracket:
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
func (f *Formatter) emitCaseLabel(lit string) bool {
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
func (f *Formatter) handleIdent(tok lexer.Token) {
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
func (f *Formatter) startsWithNewStatement() bool {
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
func (f *Formatter) looksLikeCaseLabel() bool {
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
func (f *Formatter) handleAtom(tok lexer.Token, kw string) {
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

// emitComment emits a comment token on its own line.
func (f *Formatter) emitComment(tok lexer.Token) {
	f.flush()
	if !f.lineStart {
		f.requestNL()
		f.flush()
	}
	f.attach(normalizeComment(tok.Literal))
	f.requestNL()
	f.advance()
}

// normalizeComment ensures a comment starts with a space and a capital letter.
// Pragmas (opening with "{") are left unchanged.
func normalizeComment(lit string) string {
	switch {
	case strings.HasPrefix(lit, "//"):
		if len(lit) <= 2 {
			return lit
		}
		rest := strings.TrimLeft(lit[2:], " \t")
		if len(rest) == 0 {
			return "//"
		}
		if c := rest[0]; c >= 'a' && c <= 'z' {
			rest = string(c-'a'+'A') + rest[1:]
		}
		return "// " + rest
	case strings.HasPrefix(lit, "(*"):
		if len(lit) <= 4 {
			return lit
		}
		end := strings.LastIndex(lit, "*)")
		if end < 0 {
			return lit
		}
		inner := strings.TrimLeft(lit[2:end], " \t")
		if len(inner) == 0 {
			return lit
		}
		if c := inner[0]; c >= 'a' && c <= 'z' {
			inner = string(c-'a'+'A') + inner[1:]
		}
		return "(* " + inner + "*)"
	default:
		return lit
	}
}
