package parser

import "github.com/ysmilda/stformat/internal/lexer"

// Pos represents a source position.
type Pos struct {
	Line int
	Col  int
}

// Node is the base interface for all AST nodes.
type Node interface {
	nodeType() string
}

// Statement is the interface for statement nodes.
type Statement interface {
	Node
	stmtNode()
}

// Expression is the interface for expression nodes.
type Expression interface {
	Node
	exprNode()
}

// TypeExpr is the interface for type expression nodes.
type TypeExpr interface {
	Node
	typeExprNode()
}

// Program represents a complete ST source file.
type Program struct {
	Name       string
	POUs       []POU
	TypeBlocks []TypeBlock
	Globals    []VarBlock
}

func (p *Program) nodeType() string { return "Program" }

// POU represents a Program Organisation Unit (FUNCTION, FUNCTION_BLOCK, PROGRAM).
type POU struct {
	Kind      lexer.TokenType // TokenFunction, TokenFunctionBlock, TokenProgram
	Name      string
	RetType   *TypeExpr // Return type (FUNCTION only)
	VarBlocks []VarBlock
	Body      []Statement
	Pos       Pos
}

func (p *POU) nodeType() string { return "POU" }

// TypeBlock represents a TYPE ... END_TYPE declaration.
type TypeBlock struct {
	Decls []TypeDecl
	Pos   Pos
}

func (t *TypeBlock) nodeType() string { return "TypeBlock" }

// TypeDecl represents a single type declaration inside a TYPE block.
type TypeDecl struct {
	Name string
	Type TypeExpr
	Pos  Pos
}

func (t *TypeDecl) nodeType() string { return "TypeDecl" }

// VarBlock represents a VAR ... END_VAR block.
type VarBlock struct {
	Kind      lexer.TokenType // TokenVar, TokenVarInput, etc.
	Qualifier lexer.TokenType // TokenRetain, TokenConstant, or 0
	Variables []VarDecl
	Pos       Pos
}

func (v *VarBlock) nodeType() string { return "VarBlock" }

// VarDecl represents a single variable declaration.
type VarDecl struct {
	Names []string // Multiple names: a, b : INT;
	Type  TypeExpr
	Addr  string     // AT %IX0.0, optional
	Init  Expression // Initial value, optional
	Pos   Pos
}

func (v *VarDecl) nodeType() string { return "VarDecl" }

// --- Type Expressions ---

// ScalarTypeRef refers to a built-in scalar type (BOOL, INT, REAL, etc.).
type ScalarTypeRef struct {
	Name string
}

func (s *ScalarTypeRef) nodeType() string { return "ScalarTypeRef" }
func (s *ScalarTypeRef) typeExprNode()    {}
func (s *ScalarTypeRef) String() string   { return s.Name }

// NamedTypeRef refers to a user-defined type by name.
type NamedTypeRef struct {
	Name string
}

func (n *NamedTypeRef) nodeType() string { return "NamedTypeRef" }
func (n *NamedTypeRef) typeExprNode()    {}
func (n *NamedTypeRef) String() string   { return n.Name }

// ArrayType represents ARRAY [lo..hi] OF type.
type ArrayType struct {
	Lo   Expression
	Hi   Expression
	Elem TypeExpr
}

func (a *ArrayType) nodeType() string { return "ArrayType" }
func (a *ArrayType) typeExprNode()    {}
func (a *ArrayType) String() string   { return "ARRAY[..] OF ..." }

// StructType represents STRUCT ... END_STRUCT.
type StructType struct {
	Fields []VarDecl
}

func (s *StructType) nodeType() string { return "StructType" }
func (s *StructType) typeExprNode()    {}
func (s *StructType) String() string   { return "STRUCT ... END_STRUCT" }

// StringType represents STRING(n) or WSTRING(n).
type StringType struct {
	MaxLen int
	Wide   bool
}

func (s *StringType) nodeType() string { return "StringType" }
func (s *StringType) typeExprNode()    {}
func (s *StringType) String() string   { return "STRING" }

// PointerType represents POINTER TO type.
type PointerType struct {
	Base TypeExpr
}

func (p *PointerType) nodeType() string { return "PointerType" }
func (p *PointerType) typeExprNode()    {}
func (p *PointerType) String() string   { return "POINTER TO ..." }

// --- Statements ---

// AssignStmt represents variable := expression;
type AssignStmt struct {
	Target Expression
	Value  Expression
	Pos    Pos
}

func (a *AssignStmt) nodeType() string { return "AssignStmt" }
func (a *AssignStmt) stmtNode()        {}

// CallStmt represents a function/FB call statement.
type CallStmt struct {
	Call *CallExpr
	Pos  Pos
}

func (c *CallStmt) nodeType() string { return "CallStmt" }
func (c *CallStmt) stmtNode()        {}

// IfStmt represents IF ... THEN ... ELSIF ... ELSE ... END_IF.
type IfStmt struct {
	Condition Expression
	Then      []Statement
	Elsifs    []ElsifClause
	Else      []Statement
	Pos       Pos
}

func (i *IfStmt) nodeType() string { return "IfStmt" }
func (i *IfStmt) stmtNode()        {}

// ElsifClause represents an ELSIF ... THEN ... clause.
type ElsifClause struct {
	Condition Expression
	Body      []Statement
	Pos       Pos
}

// CaseStmt represents CASE ... OF ... END_CASE.
type CaseStmt struct {
	Expression Expression
	Cases      []CaseClause
	Else       []Statement
	Pos        Pos
}

func (c *CaseStmt) nodeType() string { return "CaseStmt" }
func (c *CaseStmt) stmtNode()        {}

// CaseClause represents a single case label with its body.
type CaseClause struct {
	Labels []CaseLabel
	Body   []Statement
}

// CaseLabel represents a case label (constant or range).
type CaseLabel struct {
	Const Expression
	Hi    Expression // nil if not a range
}

// ForStmt represents FOR ... TO ... [BY ...] DO ... END_FOR.
type ForStmt struct {
	Variable string
	Start    Expression
	End      Expression
	Step     Expression // nil if no BY clause
	Body     []Statement
	Pos      Pos
}

func (f *ForStmt) nodeType() string { return "ForStmt" }
func (f *ForStmt) stmtNode()        {}

// WhileStmt represents WHILE ... DO ... END_WHILE.
type WhileStmt struct {
	Condition Expression
	Body      []Statement
	Pos       Pos
}

func (w *WhileStmt) nodeType() string { return "WhileStmt" }
func (w *WhileStmt) stmtNode()        {}

// RepeatStmt represents REPEAT ... UNTIL ... END_REPEAT.
type RepeatStmt struct {
	Body      []Statement
	Condition Expression
	Pos       Pos
}

func (r *RepeatStmt) nodeType() string { return "RepeatStmt" }
func (r *RepeatStmt) stmtNode()        {}

// ReturnStmt represents RETURN;
type ReturnStmt struct{ Pos Pos }

func (r *ReturnStmt) nodeType() string { return "ReturnStmt" }
func (r *ReturnStmt) stmtNode()        {}

// ExitStmt represents EXIT;
type ExitStmt struct{ Pos Pos }

func (e *ExitStmt) nodeType() string { return "ExitStmt" }
func (e *ExitStmt) stmtNode()        {}

// ContinueStmt represents CONTINUE;
type ContinueStmt struct{ Pos Pos }

func (c *ContinueStmt) nodeType() string { return "ContinueStmt" }
func (c *ContinueStmt) stmtNode()        {}

// --- Expressions ---

// NumberLit represents a numeric literal.
type NumberLit struct {
	Value string
	Base  int // 10 for decimal, 16 for hex, etc.
}

func (n *NumberLit) nodeType() string { return "NumberLit" }
func (n *NumberLit) exprNode()        {}

// StringLit represents a string literal.
type StringLit struct {
	Value string
	Wide  bool
}

func (s *StringLit) nodeType() string { return "StringLit" }
func (s *StringLit) exprNode()        {}

// BoolLit represents TRUE or FALSE.
type BoolLit struct {
	Value bool
}

func (b *BoolLit) nodeType() string { return "BoolLit" }
func (b *BoolLit) exprNode()        {}

// TimeLit represents a time literal (T#5s, TIME#1h30m, etc.).
type TimeLit struct {
	Value string
}

func (t *TimeLit) nodeType() string { return "TimeLit" }
func (t *TimeLit) exprNode()        {}

// TypedLit represents a typed literal (INT#42, REAL#3.14, etc.).
type TypedLit struct {
	Type  string
	Value Expression
}

func (t *TypedLit) nodeType() string { return "TypedLit" }
func (t *TypedLit) exprNode()        {}

// IdentExpr represents a variable reference.
type IdentExpr struct {
	Name string
	Pos  Pos
}

func (i *IdentExpr) nodeType() string { return "IdentExpr" }
func (i *IdentExpr) exprNode()        {}

// MemberExpr represents dot access (obj.field).
type MemberExpr struct {
	Object Expression
	Member string
	Pos    Pos
}

func (m *MemberExpr) nodeType() string { return "MemberExpr" }
func (m *MemberExpr) exprNode()        {}

// IndexExpr represents array indexing (arr[idx]).
type IndexExpr struct {
	Array Expression
	Index Expression
	Pos   Pos
}

func (i *IndexExpr) nodeType() string { return "IndexExpr" }
func (i *IndexExpr) exprNode()        {}

// CallExpr represents a function/FB call.
type CallExpr struct {
	Name   string
	Args   []Expression
	Named  []NamedArg
	Output []OutputBinding
	Pos    Pos
}

func (c *CallExpr) nodeType() string { return "CallExpr" }
func (c *CallExpr) exprNode()        {}

// NamedArg represents a named argument in a call (name := value).
type NamedArg struct {
	Name  string
	Value Expression
	Pos   Pos
}

// OutputBinding represents an output binding in a call (name => target).
type OutputBinding struct {
	Name   string
	Target Expression
	Pos    Pos
}

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Op    string // +, -, *, /, AND, OR, XOR, MOD, =, <>, <, >, <=, >=
	Left  Expression
	Right Expression
}

func (b *BinaryExpr) nodeType() string { return "BinaryExpr" }
func (b *BinaryExpr) exprNode()        {}

// UnaryExpr represents a unary operation (NOT, -).
type UnaryExpr struct {
	Op   string // NOT, -
	Expr Expression
}

func (u *UnaryExpr) nodeType() string { return "UnaryExpr" }
func (u *UnaryExpr) exprNode()        {}

// ParenExpr represents a parenthesized expression.
type ParenExpr struct {
	Expr Expression
	Pos  Pos
}

func (p *ParenExpr) nodeType() string { return "ParenExpr" }
func (p *ParenExpr) exprNode()        {}

// AddressExpr represents a located variable (%IX0.0).
type AddressExpr struct {
	Address string
	Pos     Pos
}

func (a *AddressExpr) nodeType() string { return "AddressExpr" }
func (a *AddressExpr) exprNode()        {}
