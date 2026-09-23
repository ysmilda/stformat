package parser

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ysmilda/stformat/internal/lexer"
)

// Parser builds an AST from a token stream.
type Parser struct {
	tokens []lexer.Token
	pos    int
}

// Parse parses the input source code and returns an AST.
func Parse(source string) (*Program, error) {
	tokens := lexer.Lex(source)
	p := &Parser{tokens: tokens}
	return p.parseProgram()
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekAt(offset int) lexer.Token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[idx]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if tok.Type != lexer.TokenEOF {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return tok, fmt.Errorf("line %d: expected %s, got %s (%q)", tok.Line, lexer.TokenName(tt), lexer.TokenName(tok.Type), tok.Literal)
	}
	return p.advance(), nil
}

func (p *Parser) match(tt lexer.TokenType) bool {
	if p.peek().Type == tt {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) skipComments() {
	for p.peek().Type == lexer.TokenLineComment || p.peek().Type == lexer.TokenBlockComment {
		p.advance()
	}
}

// parseProgram parses the top-level program structure.
func (p *Parser) parseProgram() (*Program, error) {
	prog := &Program{}

	p.skipComments()

	// Parse type blocks, global VAR blocks, and POUs
	for p.peek().Type != lexer.TokenEOF {
		p.skipComments()
		tok := p.peek()

		switch tok.Type {
		case lexer.TokenProgram:
			pou, err := p.parsePOU(lexer.TokenProgram)
			if err != nil {
				return nil, err
			}
			prog.Name = pou.Name
			prog.POUs = append(prog.POUs, *pou)

		case lexer.TokenFunction:
			pou, err := p.parsePOU(lexer.TokenFunction)
			if err != nil {
				return nil, err
			}
			prog.POUs = append(prog.POUs, *pou)

		case lexer.TokenFunctionBlock:
			pou, err := p.parsePOU(lexer.TokenFunctionBlock)
			if err != nil {
				return nil, err
			}
			prog.POUs = append(prog.POUs, *pou)

		case lexer.TokenTypeKw:
			tb, err := p.parseTypeBlock()
			if err != nil {
				return nil, err
			}
			prog.TypeBlocks = append(prog.TypeBlocks, *tb)

		case lexer.TokenVar, lexer.TokenVarInput, lexer.TokenVarOutput,
			lexer.TokenVarInOut, lexer.TokenVarTemp, lexer.TokenVarGlobal,
			lexer.TokenVarExternal, lexer.TokenVarAccess, lexer.TokenVarConfig,
			lexer.TokenVarStat, lexer.TokenVarInst:
			vb, err := p.parseVarBlock()
			if err != nil {
				return nil, err
			}
			prog.Globals = append(prog.Globals, *vb)

		default:
			// Skip unknown tokens
			p.advance()
		}
	}

	return prog, nil
}

// parsePOU parses a PROGRAM, FUNCTION, or FUNCTION_BLOCK declaration.
func (p *Parser) parsePOU(kind lexer.TokenType) (*POU, error) {
	pou := &POU{Kind: kind}
	pou.Pos = p.posToPos()

	// Skip opening keyword
	p.advance()

	// Parse name
	nameTok, err := p.expect(lexer.TokenIdent)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", kindName(kind), err)
	}
	pou.Name = nameTok.Literal

	// For FUNCTION, parse return type
	if kind == lexer.TokenFunction {
		if _, err := p.expect(lexer.TokenColon); err != nil {
			return nil, fmt.Errorf("parsing FUNCTION %s: %w", pou.Name, err)
		}
		retType, err := p.parseTypeExpr()
		if err != nil {
			return nil, fmt.Errorf("parsing FUNCTION %s return type: %w", pou.Name, err)
		}
		pou.RetType = &retType
	}

	p.skipComments()

	// Parse VAR blocks
	for {
		tok := p.peek()
		if !isVarBlockStart(tok.Type) {
			break
		}
		vb, err := p.parseVarBlock()
		if err != nil {
			return nil, fmt.Errorf("parsing %s %s: %w", kindName(kind), pou.Name, err)
		}
		pou.VarBlocks = append(pou.VarBlocks, *vb)
		p.skipComments()
	}

	// Parse body statements
	endTok := endTokenFor(kind)
	body, err := p.parseStatementBlock(endTok)
	if err != nil {
		return nil, fmt.Errorf("parsing %s %s body: %w", kindName(kind), pou.Name, err)
	}
	pou.Body = body

	// Expect end keyword
	if _, err := p.expect(endTok); err != nil {
		return nil, fmt.Errorf("parsing %s %s: %w", kindName(kind), pou.Name, err)
	}

	// Expect semicolon after end keyword
	p.match(lexer.TokenSemicolon)

	return pou, nil
}

// parseTypeBlock parses TYPE ... END_TYPE.
func (p *Parser) parseTypeBlock() (*TypeBlock, error) {
	tb := &TypeBlock{}
	tb.Pos = p.posToPos()

	p.advance() // skip TYPE
	p.skipComments()

	for {
		p.skipComments()
		if p.peek().Type == lexer.TokenEndType {
			break
		}

		decl, err := p.parseTypeDecl()
		if err != nil {
			return nil, err
		}
		tb.Decls = append(tb.Decls, *decl)
		p.skipComments()
	}

	p.advance() // skip END_TYPE
	p.match(lexer.TokenSemicolon)

	return tb, nil
}

// parseTypeDecl parses a single type declaration.
func (p *Parser) parseTypeDecl() (*TypeDecl, error) {
	td := &TypeDecl{}
	td.Pos = p.posToPos()

	nameTok, err := p.expect(lexer.TokenIdent)
	if err != nil {
		return nil, err
	}
	td.Name = nameTok.Literal

	if _, err := p.expect(lexer.TokenColon); err != nil {
		return nil, err
	}

	tp, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	td.Type = tp

	p.match(lexer.TokenSemicolon)
	return td, nil
}

// parseVarBlock parses a VAR ... END_VAR block.
func (p *Parser) parseVarBlock() (*VarBlock, error) {
	vb := &VarBlock{}
	vb.Pos = p.posToPos()

	tok := p.advance()
	vb.Kind = tok.Type

	// Check for qualifier (RETAIN, CONSTANT, etc.)
	p.skipComments()
	if p.peek().Type == lexer.TokenRetain {
		vb.Qualifier = lexer.TokenRetain
		p.advance()
	} else if p.peek().Type == lexer.TokenConstant {
		vb.Qualifier = lexer.TokenConstant
		p.advance()
	} else if p.peek().Type == lexer.TokenNonRetain {
		vb.Qualifier = lexer.TokenNonRetain
		p.advance()
	} else if p.peek().Type == lexer.TokenPersistent {
		vb.Qualifier = lexer.TokenPersistent
		p.advance()
	}

	// Parse variable declarations
	for {
		p.skipComments()
		if p.peek().Type == lexer.TokenEndVar {
			break
		}

		decls, err := p.parseVarDeclList()
		if err != nil {
			return nil, err
		}
		vb.Variables = append(vb.Variables, decls...)
		p.skipComments()
	}

	p.advance() // skip END_VAR
	p.match(lexer.TokenSemicolon)

	return vb, nil
}

// parseVarDeclList parses a comma-separated list of variable declarations.
func (p *Parser) parseVarDeclList() ([]VarDecl, error) {
	var decls []VarDecl

	// Collect names
	var names []string
	for {
		p.skipComments()

		// Check if this is an AT address declaration
		if p.peek().Type == lexer.TokenAt {
			break
		}

		// Check for end of declaration list
		if p.peek().Type == lexer.TokenColon {
			break
		}

		nameTok, err := p.expect(lexer.TokenIdent)
		if err != nil {
			return nil, err
		}
		names = append(names, nameTok.Literal)

		p.skipComments()
		if p.peek().Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}

	// Parse optional AT address
	var addr string
	if p.peek().Type == lexer.TokenAt {
		p.advance()
		addrTok := p.advance()
		addr = addrTok.Literal
		p.skipComments()
	}

	// Parse type
	if _, err := p.expect(lexer.TokenColon); err != nil {
		return nil, err
	}

	tp, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}

	// Parse optional initializer
	var init Expression
	if p.peek().Type == lexer.TokenAssign {
		p.advance()
		init, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	p.match(lexer.TokenSemicolon)

	// Create one VarDecl per name
	for _, name := range names {
		decl := VarDecl{
			Names: []string{name},
			Type:  tp,
			Addr:  addr,
			Init:  init,
			Pos:   p.posToPos(),
		}
		decls = append(decls, decl)
	}

	return decls, nil
}

// parseTypeExpr parses a type expression.
func (p *Parser) parseTypeExpr() (TypeExpr, error) {
	tok := p.peek()

	switch tok.Type {
	case lexer.TokenArray:
		return p.parseArrayType()
	case lexer.TokenStruct:
		return p.parseStructType()
	case lexer.TokenStringType, lexer.TokenWstringType:
		return p.parseStringType()
	case lexer.TokenPointer:
		p.advance()
		if _, err := p.expect(lexer.TokenTo); err != nil {
			return nil, err
		}
		base, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return &PointerType{Base: base}, nil
	case lexer.TokenReference:
		p.advance()
		if _, err := p.expect(lexer.TokenTo); err != nil {
			return nil, err
		}
		base, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return &PointerType{Base: base}, nil
	default:
		// Scalar or named type
		p.advance()
		name := strings.ToUpper(tok.Literal)
		if lexer.IsScalarType(name) {
			return &ScalarTypeRef{Name: name}, nil
		}
		return &NamedTypeRef{Name: tok.Literal}, nil
	}
}

// parseArrayType parses ARRAY [lo..hi] OF type.
func (p *Parser) parseArrayType() (*ArrayType, error) {
	at := &ArrayType{}

	p.advance() // skip ARRAY
	if _, err := p.expect(lexer.TokenLBracket); err != nil {
		return nil, err
	}

	lo, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	at.Lo = lo

	if _, err := p.expect(lexer.TokenDotDot); err != nil {
		return nil, err
	}

	hi, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	at.Hi = hi

	if _, err := p.expect(lexer.TokenRBracket); err != nil {
		return nil, err
	}

	if _, err := p.expect(lexer.TokenOf); err != nil {
		return nil, err
	}

	elem, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	at.Elem = elem

	return at, nil
}

// parseStructType parses STRUCT ... END_STRUCT.
func (p *Parser) parseStructType() (*StructType, error) {
	st := &StructType{}

	p.advance() // skip STRUCT
	p.skipComments()

	for {
		p.skipComments()
		if p.peek().Type == lexer.TokenEndStruct {
			break
		}

		decls, err := p.parseVarDeclList()
		if err != nil {
			return nil, err
		}
		st.Fields = append(st.Fields, decls...)
	}

	p.advance() // skip END_STRUCT
	return st, nil
}

// parseStringType parses STRING(n) or WSTRING(n).
func (p *Parser) parseStringType() (*StringType, error) {
	st := &StringType{}
	tok := p.advance()
	st.Wide = tok.Type == lexer.TokenWstringType

	if p.peek().Type == lexer.TokenLParen {
		p.advance()
		lenTok, err := p.expect(lexer.TokenNumber)
		if err != nil {
			return nil, err
		}
		n, _ := strconv.Atoi(lenTok.Literal)
		st.MaxLen = n
		if _, err := p.expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
	}

	return st, nil
}

// parseStatementBlock parses statements until a terminator token.
func (p *Parser) parseStatementBlock(terminators ...lexer.TokenType) ([]Statement, error) {
	var stmts []Statement

	for {
		p.skipComments()
		tok := p.peek()

		// Check for terminator
		if slices.Contains(terminators, tok.Type) {
			return stmts, nil
		}
		if tok.Type == lexer.TokenEOF {
			return stmts, nil
		}

		// Check for var blocks inside POU body (rare but valid)
		if isVarBlockStart(tok.Type) {
			break
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	return stmts, nil
}

// parseStatement parses a single statement.
func (p *Parser) parseStatement() (Statement, error) {
	p.skipComments()
	tok := p.peek()

	switch tok.Type {
	case lexer.TokenIf:
		return p.parseIfStmt()
	case lexer.TokenFor:
		return p.parseForStmt()
	case lexer.TokenWhile:
		return p.parseWhileStmt()
	case lexer.TokenRepeat:
		return p.parseRepeatStmt()
	case lexer.TokenCase:
		return p.parseCaseStmt()
	case lexer.TokenReturn:
		p.advance()
		p.match(lexer.TokenSemicolon)
		return &ReturnStmt{Pos: p.posToPos()}, nil
	case lexer.TokenExit:
		p.advance()
		p.match(lexer.TokenSemicolon)
		return &ExitStmt{Pos: p.posToPos()}, nil
	case lexer.TokenContinue:
		p.advance()
		p.match(lexer.TokenSemicolon)
		return &ContinueStmt{Pos: p.posToPos()}, nil
	case lexer.TokenIdent:
		return p.parseAssignOrCall()
	default:
		// Skip unknown tokens
		p.advance()
		return nil, nil
	}
}

// parseIfStmt parses IF ... THEN ... ELSIF ... ELSE ... END_IF.
func (p *Parser) parseIfStmt() (*IfStmt, error) {
	stmt := &IfStmt{}
	stmt.Pos = p.posToPos()

	p.advance() // skip IF

	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.Condition = cond

	if _, err := p.expect(lexer.TokenThen); err != nil {
		return nil, err
	}

	body, err := p.parseStatementBlock(lexer.TokenElsif, lexer.TokenElse, lexer.TokenEndIf)
	if err != nil {
		return nil, err
	}
	stmt.Then = body

	// Parse ELSIF clauses
	for p.peek().Type == lexer.TokenElsif {
		elsif := ElsifClause{}
		elsif.Pos = p.posToPos()

		p.advance() // skip ELSIF

		cond, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		elsif.Condition = cond

		if _, err := p.expect(lexer.TokenThen); err != nil {
			return nil, err
		}

		body, err := p.parseStatementBlock(lexer.TokenElsif, lexer.TokenElse, lexer.TokenEndIf)
		if err != nil {
			return nil, err
		}
		elsif.Body = body

		stmt.Elsifs = append(stmt.Elsifs, elsif)
	}

	// Parse ELSE
	if p.peek().Type == lexer.TokenElse {
		p.advance()
		body, err := p.parseStatementBlock(lexer.TokenEndIf)
		if err != nil {
			return nil, err
		}
		stmt.Else = body
	}

	if _, err := p.expect(lexer.TokenEndIf); err != nil {
		return nil, err
	}
	p.match(lexer.TokenSemicolon)

	return stmt, nil
}

// parseCaseStmt parses CASE ... OF ... END_CASE.
func (p *Parser) parseCaseStmt() (*CaseStmt, error) {
	stmt := &CaseStmt{}
	stmt.Pos = p.posToPos()

	p.advance() // skip CASE

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.Expression = expr

	if _, err := p.expect(lexer.TokenOf); err != nil {
		return nil, err
	}

	// Parse case clauses
	for {
		p.skipComments()
		tok := p.peek()

		if tok.Type == lexer.TokenElse || tok.Type == lexer.TokenEndCase {
			break
		}

		labels, err := p.parseCaseLabels()
		if err != nil {
			return nil, err
		}

		if _, err := p.expect(lexer.TokenColon); err != nil {
			return nil, err
		}

		body, err := p.parseStatementBlock(lexer.TokenElse, lexer.TokenEndCase)
		if err != nil {
			return nil, err
		}

		stmt.Cases = append(stmt.Cases, CaseClause{
			Labels: labels,
			Body:   body,
		})
	}

	// Parse ELSE
	if p.peek().Type == lexer.TokenElse {
		p.advance()
		body, err := p.parseStatementBlock(lexer.TokenEndCase)
		if err != nil {
			return nil, err
		}
		stmt.Else = body
	}

	if _, err := p.expect(lexer.TokenEndCase); err != nil {
		return nil, err
	}
	p.match(lexer.TokenSemicolon)

	return stmt, nil
}

// parseCaseLabels parses a comma-separated list of case labels.
func (p *Parser) parseCaseLabels() ([]CaseLabel, error) {
	var labels []CaseLabel

	for {
		label, err := p.parseCaseLabel()
		if err != nil {
			return nil, err
		}
		labels = append(labels, *label)

		if p.peek().Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}

	return labels, nil
}

// parseCaseLabel parses a single case label (constant or range).
func (p *Parser) parseCaseLabel() (*CaseLabel, error) {
	label := &CaseLabel{}

	constExpr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	label.Const = constExpr

	// Check for range
	if p.peek().Type == lexer.TokenDotDot {
		p.advance()
		hi, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		label.Hi = hi
	}

	return label, nil
}

// parseForStmt parses FOR ... TO ... [BY ...] DO ... END_FOR.
func (p *Parser) parseForStmt() (*ForStmt, error) {
	stmt := &ForStmt{}
	stmt.Pos = p.posToPos()

	p.advance() // skip FOR

	varTok, err := p.expect(lexer.TokenIdent)
	if err != nil {
		return nil, err
	}
	stmt.Variable = varTok.Literal

	if _, err := p.expect(lexer.TokenAssign); err != nil {
		return nil, err
	}

	start, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.Start = start

	if _, err := p.expect(lexer.TokenTo); err != nil {
		return nil, err
	}

	end, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.End = end

	// Optional BY
	if p.peek().Type == lexer.TokenBy {
		p.advance()
		step, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Step = step
	}

	if _, err := p.expect(lexer.TokenDo); err != nil {
		return nil, err
	}

	body, err := p.parseStatementBlock(lexer.TokenEndFor)
	if err != nil {
		return nil, err
	}
	stmt.Body = body

	if _, err := p.expect(lexer.TokenEndFor); err != nil {
		return nil, err
	}
	p.match(lexer.TokenSemicolon)

	return stmt, nil
}

// parseWhileStmt parses WHILE ... DO ... END_WHILE.
func (p *Parser) parseWhileStmt() (*WhileStmt, error) {
	stmt := &WhileStmt{}
	stmt.Pos = p.posToPos()

	p.advance() // skip WHILE

	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.Condition = cond

	if _, err := p.expect(lexer.TokenDo); err != nil {
		return nil, err
	}

	body, err := p.parseStatementBlock(lexer.TokenEndWhile)
	if err != nil {
		return nil, err
	}
	stmt.Body = body

	if _, err := p.expect(lexer.TokenEndWhile); err != nil {
		return nil, err
	}
	p.match(lexer.TokenSemicolon)

	return stmt, nil
}

// parseRepeatStmt parses REPEAT ... UNTIL ... END_REPEAT.
func (p *Parser) parseRepeatStmt() (*RepeatStmt, error) {
	stmt := &RepeatStmt{}
	stmt.Pos = p.posToPos()

	p.advance() // skip REPEAT

	body, err := p.parseStatementBlock(lexer.TokenUntil)
	if err != nil {
		return nil, err
	}
	stmt.Body = body

	if _, err := p.expect(lexer.TokenUntil); err != nil {
		return nil, err
	}

	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	stmt.Condition = cond

	if _, err := p.expect(lexer.TokenEndRepeat); err != nil {
		return nil, err
	}
	p.match(lexer.TokenSemicolon)

	return stmt, nil
}

// parseAssignOrCall parses an assignment or function/FB call statement.
func (p *Parser) parseAssignOrCall() (Statement, error) {
	lhs, err := p.parsePostfixExpr()
	if err != nil {
		return nil, err
	}

	// Check for assignment
	if p.peek().Type == lexer.TokenAssign {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.match(lexer.TokenSemicolon)
		return &AssignStmt{
			Target: lhs,
			Value:  val,
			Pos:    p.posToPos(),
		}, nil
	}

	// Otherwise it's a call statement
	call, ok := lhs.(*CallExpr)
	if !ok {
		// Not a call, skip
		p.match(lexer.TokenSemicolon)
		return nil, nil
	}

	p.match(lexer.TokenSemicolon)
	return &CallStmt{
		Call: call,
		Pos:  call.Pos,
	}, nil
}

// --- Expression parsing (precedence climbing) ---

func (p *Parser) parseExpression() (Expression, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expression, error) {
	left, err := p.parseXor()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenOr || tok.Type == lexer.TokenOrElse {
			p.advance()
			right, err := p.parseXor()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: strings.ToUpper(tok.Literal), Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseXor() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for {
		if p.peek().Type == lexer.TokenXor {
			p.advance()
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "XOR", Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseAnd() (Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenAnd || tok.Type == lexer.TokenAndThen {
			p.advance()
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "AND", Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseComparison() (Expression, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenEqual, lexer.TokenNotEqual,
			lexer.TokenLess, lexer.TokenLessEq,
			lexer.TokenGreater, lexer.TokenGreaterEq:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: tok.Literal, Left: left, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseAddition() (Expression, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenPlus || tok.Type == lexer.TokenMinus {
			p.advance()
			right, err := p.parseMultiplication()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: tok.Literal, Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseMultiplication() (Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenStar || tok.Type == lexer.TokenSlash || tok.Type == lexer.TokenMod {
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: strings.ToUpper(tok.Literal), Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *Parser) parseUnary() (Expression, error) {
	tok := p.peek()

	if tok.Type == lexer.TokenNot {
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Expr: expr}, nil
	}

	if tok.Type == lexer.TokenMinus {
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Expr: expr}, nil
	}

	if tok.Type == lexer.TokenPlus {
		p.advance()
		return p.parseUnary()
	}

	return p.parsePostfixExpr()
}

func (p *Parser) parsePostfixExpr() (Expression, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	// Handle postfix: .member, [index], (args)
	for {
		switch p.peek().Type {
		case lexer.TokenDot:
			p.advance()
			member, err := p.expect(lexer.TokenIdent)
			if err != nil {
				return nil, err
			}
			expr = &MemberExpr{
				Object: expr,
				Member: member.Literal,
				Pos:    p.posToPos(),
			}
		case lexer.TokenLBracket:
			p.advance()
			idx, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenRBracket); err != nil {
				return nil, err
			}
			expr = &IndexExpr{
				Array: expr,
				Index: idx,
				Pos:   p.posToPos(),
			}
		case lexer.TokenLParen:
			// Check if this is a call
			ident, ok := expr.(*IdentExpr)
			if !ok {
				return expr, nil
			}
			p.advance()
			call := &CallExpr{
				Name: ident.Name,
				Pos:  ident.Pos,
			}
			if p.peek().Type != lexer.TokenRParen {
				args, named, output, err := p.parseCallArgs()
				if err != nil {
					return nil, err
				}
				call.Args = args
				call.Named = named
				call.Output = output
			}
			if _, err := p.expect(lexer.TokenRParen); err != nil {
				return nil, err
			}
			expr = call
		default:
			return expr, nil
		}
	}
}

func (p *Parser) parseCallArgs() ([]Expression, []NamedArg, []OutputBinding, error) {
	var args []Expression
	var named []NamedArg
	var output []OutputBinding

	for {
		p.skipComments()

		// Check if this is a named arg (name := value) or output binding (name => target)
		if p.peek().Type == lexer.TokenIdent {
			// Look ahead: ident := expr or ident => expr
			name := p.peek().Literal
			if p.peekAt(1).Type == lexer.TokenAssign {
				p.advance() // skip name
				p.advance() // skip :=
				val, err := p.parseExpression()
				if err != nil {
					return nil, nil, nil, err
				}
				named = append(named, NamedArg{Name: name, Value: val, Pos: p.posToPos()})
			} else if p.peekAt(1).Type == lexer.TokenOutputBind {
				p.advance() // skip name
				p.advance() // skip =>
				target, err := p.parseExpression()
				if err != nil {
					return nil, nil, nil, err
				}
				output = append(output, OutputBinding{Name: name, Target: target, Pos: p.posToPos()})
			} else {
				expr, err := p.parseExpression()
				if err != nil {
					return nil, nil, nil, err
				}
				args = append(args, expr)
			}
		} else {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, nil, nil, err
			}
			args = append(args, expr)
		}

		if p.peek().Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}

	return args, named, output, nil
}

func (p *Parser) parsePrimary() (Expression, error) {
	tok := p.peek()

	switch tok.Type {
	case lexer.TokenNumber:
		p.advance()
		return &NumberLit{Value: tok.Literal, Base: 10}, nil

	case lexer.TokenBasedNumber:
		p.advance()
		return &NumberLit{Value: tok.Literal, Base: extractBase(tok.Literal)}, nil

	case lexer.TokenString:
		p.advance()
		return &StringLit{Value: tok.Literal, Wide: false}, nil

	case lexer.TokenTimeLiteral:
		p.advance()
		return &TimeLit{Value: tok.Literal}, nil

	case lexer.TokenTypedLiteral:
		p.advance()
		return p.parseTypedLit(tok.Literal)

	case lexer.TokenTrue:
		p.advance()
		return &BoolLit{Value: true}, nil

	case lexer.TokenFalse:
		p.advance()
		return &BoolLit{Value: false}, nil

	case lexer.TokenAddress:
		p.advance()
		return &AddressExpr{Address: tok.Literal, Pos: p.posToPos()}, nil

	case lexer.TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return &ParenExpr{Expr: expr, Pos: p.posToPos()}, nil

	case lexer.TokenIdent:
		p.advance()
		return &IdentExpr{Name: tok.Literal, Pos: p.posToPos()}, nil

	default:
		p.advance()
		return nil, fmt.Errorf("line %d: unexpected token %s (%q)", tok.Line, lexer.TokenName(tok.Type), tok.Literal)
	}
}

func (p *Parser) parseTypedLit(literal string) (Expression, error) {
	before, after, ok := strings.Cut(literal, "#")
	if !ok {
		return &IdentExpr{Name: literal}, nil
	}

	typeName := before
	payload := after

	// Try to parse the payload as an expression
	inner := &StringLit{Value: payload}
	return &TypedLit{Type: typeName, Value: inner}, nil
}

// --- Helpers ---

func (p *Parser) posToPos() Pos {
	if p.pos < len(p.tokens) {
		return Pos{
			Line: p.tokens[p.pos].Line,
			Col:  p.tokens[p.pos].Col,
		}
	}
	return Pos{}
}

func isVarBlockStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenVar, lexer.TokenVarInput, lexer.TokenVarOutput,
		lexer.TokenVarInOut, lexer.TokenVarTemp, lexer.TokenVarGlobal,
		lexer.TokenVarExternal, lexer.TokenVarAccess, lexer.TokenVarConfig,
		lexer.TokenVarStat, lexer.TokenVarInst:
		return true
	}
	return false
}

func endTokenFor(kind lexer.TokenType) lexer.TokenType {
	switch kind {
	case lexer.TokenProgram:
		return lexer.TokenEndProgram
	case lexer.TokenFunction:
		return lexer.TokenEndFunction
	case lexer.TokenFunctionBlock:
		return lexer.TokenEndFunctionBlock
	}
	return lexer.TokenEOF
}

func kindName(kind lexer.TokenType) string {
	switch kind {
	case lexer.TokenProgram:
		return "PROGRAM"
	case lexer.TokenFunction:
		return "FUNCTION"
	case lexer.TokenFunctionBlock:
		return "FUNCTION_BLOCK"
	}
	return "UNKNOWN"
}

func extractBase(literal string) int {
	before, _, ok := strings.Cut(literal, "#")
	if !ok {
		return 10
	}
	base := before
	switch base {
	case "2":
		return 2
	case "8":
		return 8
	case "16":
		return 16
	}
	return 10
}
