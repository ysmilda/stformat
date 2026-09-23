package lexer

// TokenType represents the type of a lexical token.
type TokenType int

const (
	TokenEOF TokenType = iota

	// Literals
	TokenIdent
	TokenNumber
	TokenBasedNumber
	TokenString
	TokenTimeLiteral
	TokenTypedLiteral
	TokenAddress

	// Keywords - POU declarations
	TokenProgram
	TokenEndProgram
	TokenFunction
	TokenEndFunction
	TokenFunctionBlock
	TokenEndFunctionBlock
	TokenAction
	TokenEndAction
	TokenMethod
	TokenEndMethod
	TokenProperty
	TokenEndProperty
	TokenClass
	TokenEndClass
	TokenInterface
	TokenEndInterface

	// Keywords - Variable blocks
	TokenVar
	TokenVarInput
	TokenVarOutput
	TokenVarInOut
	TokenVarTemp
	TokenVarGlobal
	TokenVarExternal
	TokenVarAccess
	TokenVarConfig
	TokenVarStat
	TokenVarInst
	TokenEndVar

	// Keywords - Control flow
	TokenIf
	TokenThen
	TokenElsif
	TokenElse
	TokenEndIf
	TokenCase
	TokenOf
	TokenEndCase
	TokenFor
	TokenTo
	TokenBy
	TokenDo
	TokenEndFor
	TokenWhile
	TokenEndWhile
	TokenRepeat
	TokenUntil
	TokenEndRepeat
	TokenReturn
	TokenExit
	TokenContinue
	TokenJmp

	// Keywords - Boolean literals
	TokenTrue
	TokenFalse

	// Keywords - Logical operators
	TokenAnd
	TokenOr
	TokenNot
	TokenXor
	TokenMod
	TokenAndThen
	TokenOrElse

	// Keywords - Type system
	TokenTypeKw
	TokenEndType
	TokenStruct
	TokenEndStruct
	TokenUnion
	TokenEndUnion
	TokenArray
	TokenRetain
	TokenNonRetain
	TokenPersistent
	TokenConstant
	TokenAt

	// Keywords - OO (TwinCAT/IEC Ed3)
	TokenExtends
	TokenImplements
	TokenAbstract
	TokenFinal
	TokenInternal
	TokenPrivate
	TokenProtected
	TokenPublic
	TokenThis
	TokenSuper

	// Keywords - Pointer/Reference
	TokenPointer
	TokenReference
	TokenRefTo

	// Keywords - Access
	TokenReadOnly
	TokenReadWrite
	TokenParams

	// Scalar type keywords
	TokenBoolType
	TokenByteType
	TokenWordType
	TokenDwordType
	TokenLwordType
	TokenSintType
	TokenIntType
	TokenDintType
	TokenLintType
	TokenUsintType
	TokenUintType
	TokenUdintType
	TokenUlintType
	TokenRealType
	TokenLrealType
	TokenTimeType
	TokenLtimeType
	TokenDateType
	TokenLdateType
	TokenTodType
	TokenLtodType
	TokenDtType
	TokenLdtType
	TokenStringType
	TokenWstringType
	TokenCharType
	TokenWcharType

	// Operators
	TokenAssign     // :=
	TokenOutputBind // =>
	TokenEqual      // =
	TokenNotEqual   // <>
	TokenLess       // <
	TokenLessEq     // <=
	TokenGreater    // >
	TokenGreaterEq  // >=
	TokenPlus       // +
	TokenMinus      // -
	TokenStar       // *
	TokenSlash      // /
	TokenPower      // **

	// Delimiters
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenLBrace    // {
	TokenRBrace    // }
	TokenSemicolon // ;
	TokenColon     // :
	TokenComma     // ,
	TokenDot       // .
	TokenDotDot    // ..
	TokenHash      // #
	TokenCaret     // ^
	TokenPercent   // %

	// Special
	TokenSAssign      // S= (TwinCAT set)
	TokenRAssign      // R= (TwinCAT reset)
	TokenRefAssign    // REF= (TwinCAT reference)
	TokenLineComment  // // ...
	TokenBlockComment // (* ... *)
)

// Token represents a lexical token with its type, literal text, and source position.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

// Keywords maps uppercase keyword strings to their token types.
var Keywords = map[string]TokenType{
	// POU declarations
	"PROGRAM":            TokenProgram,
	"END_PROGRAM":        TokenEndProgram,
	"FUNCTION":           TokenFunction,
	"END_FUNCTION":       TokenEndFunction,
	"FUNCTION_BLOCK":     TokenFunctionBlock,
	"END_FUNCTION_BLOCK": TokenEndFunctionBlock,
	"ACTION":             TokenAction,
	"END_ACTION":         TokenEndAction,
	"METHOD":             TokenMethod,
	"END_METHOD":         TokenEndMethod,
	"PROPERTY":           TokenProperty,
	"END_PROPERTY":       TokenEndProperty,
	"CLASS":              TokenClass,
	"END_CLASS":          TokenEndClass,
	"INTERFACE":          TokenInterface,
	"END_INTERFACE":      TokenEndInterface,

	// Variable blocks
	"VAR":          TokenVar,
	"VAR_INPUT":    TokenVarInput,
	"VAR_OUTPUT":   TokenVarOutput,
	"VAR_IN_OUT":   TokenVarInOut,
	"VAR_TEMP":     TokenVarTemp,
	"VAR_GLOBAL":   TokenVarGlobal,
	"VAR_EXTERNAL": TokenVarExternal,
	"VAR_ACCESS":   TokenVarAccess,
	"VAR_CONFIG":   TokenVarConfig,
	"VAR_STAT":     TokenVarStat,
	"VAR_INST":     TokenVarInst,
	"END_VAR":      TokenEndVar,

	// Control flow
	"IF":         TokenIf,
	"THEN":       TokenThen,
	"ELSIF":      TokenElsif,
	"ELSE":       TokenElse,
	"END_IF":     TokenEndIf,
	"CASE":       TokenCase,
	"OF":         TokenOf,
	"END_CASE":   TokenEndCase,
	"FOR":        TokenFor,
	"TO":         TokenTo,
	"BY":         TokenBy,
	"DO":         TokenDo,
	"END_FOR":    TokenEndFor,
	"WHILE":      TokenWhile,
	"END_WHILE":  TokenEndWhile,
	"REPEAT":     TokenRepeat,
	"UNTIL":      TokenUntil,
	"END_REPEAT": TokenEndRepeat,
	"RETURN":     TokenReturn,
	"EXIT":       TokenExit,
	"CONTINUE":   TokenContinue,
	"JMP":        TokenJmp,

	// Boolean literals
	"TRUE":  TokenTrue,
	"FALSE": TokenFalse,

	// Logical operators
	"AND":      TokenAnd,
	"OR":       TokenOr,
	"NOT":      TokenNot,
	"XOR":      TokenXor,
	"MOD":      TokenMod,
	"AND_THEN": TokenAndThen,
	"OR_ELSE":  TokenOrElse,

	// Type system
	"TYPE":       TokenTypeKw,
	"END_TYPE":   TokenEndType,
	"STRUCT":     TokenStruct,
	"END_STRUCT": TokenEndStruct,
	"UNION":      TokenUnion,
	"END_UNION":  TokenEndUnion,
	"ARRAY":      TokenArray,
	"RETAIN":     TokenRetain,
	"NON_RETAIN": TokenNonRetain,
	"PERSISTENT": TokenPersistent,
	"CONSTANT":   TokenConstant,
	"AT":         TokenAt,

	// OO
	"EXTENDS":    TokenExtends,
	"IMPLEMENTS": TokenImplements,
	"ABSTRACT":   TokenAbstract,
	"FINAL":      TokenFinal,
	"INTERNAL":   TokenInternal,
	"PRIVATE":    TokenPrivate,
	"PROTECTED":  TokenProtected,
	"PUBLIC":     TokenPublic,
	"THIS":       TokenThis,
	"SUPER":      TokenSuper,

	// Pointer/Reference
	"POINTER":   TokenPointer,
	"REFERENCE": TokenReference,
	"REF_TO":    TokenRefTo,

	// Access
	"READ_ONLY":  TokenReadOnly,
	"READ_WRITE": TokenReadWrite,
	"PARAMS":     TokenParams,

	// Scalar types
	"BOOL":           TokenBoolType,
	"BYTE":           TokenByteType,
	"WORD":           TokenWordType,
	"DWORD":          TokenDwordType,
	"LWORD":          TokenLwordType,
	"SINT":           TokenSintType,
	"INT":            TokenIntType,
	"DINT":           TokenDintType,
	"LINT":           TokenLintType,
	"USINT":          TokenUsintType,
	"UINT":           TokenUintType,
	"UDINT":          TokenUdintType,
	"ULINT":          TokenUlintType,
	"REAL":           TokenRealType,
	"LREAL":          TokenLrealType,
	"TIME":           TokenTimeType,
	"LTIME":          TokenLtimeType,
	"DATE":           TokenDateType,
	"LDATE":          TokenLdateType,
	"TOD":            TokenTodType,
	"LTOD":           TokenLtodType,
	"DT":             TokenDtType,
	"LDT":            TokenLdtType,
	"STRING":         TokenStringType,
	"WSTRING":        TokenWstringType,
	"CHAR":           TokenCharType,
	"WCHAR":          TokenWcharType,
	"TIME_OF_DAY":    TokenTodType,
	"DATE_AND_TIME":  TokenDtType,
	"LTIME_OF_DAY":   TokenLtodType,
	"LDATE_AND_TIME": TokenLdtType,
}

// ScalarTypeNames is the set of all IEC 61131-3 elementary type names.
var ScalarTypeNames = map[string]struct{}{
	"BOOL":           {},
	"BYTE":           {},
	"WORD":           {},
	"DWORD":          {},
	"LWORD":          {},
	"SINT":           {},
	"INT":            {},
	"DINT":           {},
	"LINT":           {},
	"USINT":          {},
	"UINT":           {},
	"UDINT":          {},
	"ULINT":          {},
	"REAL":           {},
	"LREAL":          {},
	"TIME":           {},
	"LTIME":          {},
	"DATE":           {},
	"LDATE":          {},
	"TOD":            {},
	"LTOD":           {},
	"DT":             {},
	"LDT":            {},
	"STRING":         {},
	"WSTRING":        {},
	"CHAR":           {},
	"WCHAR":          {},
	"TIME_OF_DAY":    {},
	"DATE_AND_TIME":  {},
	"LTIME_OF_DAY":   {},
	"LDATE_AND_TIME": {},
}

// IsScalarType returns true if the name is an IEC 61131-3 elementary type.
func IsScalarType(name string) bool {
	_, ok := ScalarTypeNames[name]
	return ok
}

// IsScalarTypeToken returns true if the token type is an IEC 61131-3 elementary
// type keyword token (STRING, INT, BOOL, etc.).
func IsScalarTypeToken(tt TokenType) bool {
	switch tt {
	case TokenBoolType, TokenByteType, TokenWordType, TokenDwordType, TokenLwordType,
		TokenSintType, TokenIntType, TokenDintType, TokenLintType,
		TokenUsintType, TokenUintType, TokenUdintType, TokenUlintType,
		TokenRealType, TokenLrealType,
		TokenTimeType, TokenLtimeType, TokenDateType, TokenLdateType,
		TokenTodType, TokenLtodType, TokenDtType, TokenLdtType,
		TokenStringType, TokenWstringType, TokenCharType, TokenWcharType:
		return true
	}
	return false
}

// IsBlockEnd returns true if the token type is an END_* block terminator.
func IsBlockEnd(tt TokenType) bool {
	switch tt {
	case TokenEndProgram, TokenEndFunction, TokenEndFunctionBlock,
		TokenEndAction, TokenEndMethod, TokenEndProperty, TokenEndClass,
		TokenEndInterface, TokenEndVar, TokenEndIf, TokenEndCase, TokenEndFor,
		TokenEndWhile, TokenEndRepeat, TokenEndType, TokenEndStruct, TokenEndUnion:
		return true
	}
	return false
}

// KeywordSpelling returns the canonical UPPERCASE spelling of a keyword token.
// It returns an empty string for non-keyword tokens.
func KeywordSpelling(tt TokenType) string {
	switch tt {
	case TokenProgram:
		return "PROGRAM"
	case TokenEndProgram:
		return "END_PROGRAM"
	case TokenFunction:
		return "FUNCTION"
	case TokenEndFunction:
		return "END_FUNCTION"
	case TokenFunctionBlock:
		return "FUNCTION_BLOCK"
	case TokenEndFunctionBlock:
		return "END_FUNCTION_BLOCK"
	case TokenAction:
		return "ACTION"
	case TokenEndAction:
		return "END_ACTION"
	case TokenMethod:
		return "METHOD"
	case TokenEndMethod:
		return "END_METHOD"
	case TokenProperty:
		return "PROPERTY"
	case TokenEndProperty:
		return "END_PROPERTY"
	case TokenClass:
		return "CLASS"
	case TokenEndClass:
		return "END_CLASS"
	case TokenInterface:
		return "INTERFACE"
	case TokenEndInterface:
		return "END_INTERFACE"
	case TokenVar:
		return "VAR"
	case TokenVarInput:
		return "VAR_INPUT"
	case TokenVarOutput:
		return "VAR_OUTPUT"
	case TokenVarInOut:
		return "VAR_IN_OUT"
	case TokenVarTemp:
		return "VAR_TEMP"
	case TokenVarGlobal:
		return "VAR_GLOBAL"
	case TokenVarExternal:
		return "VAR_EXTERNAL"
	case TokenVarAccess:
		return "VAR_ACCESS"
	case TokenVarConfig:
		return "VAR_CONFIG"
	case TokenVarStat:
		return "VAR_STAT"
	case TokenVarInst:
		return "VAR_INST"
	case TokenEndVar:
		return "END_VAR"
	case TokenIf:
		return "IF"
	case TokenThen:
		return "THEN"
	case TokenElsif:
		return "ELSIF"
	case TokenElse:
		return "ELSE"
	case TokenEndIf:
		return "END_IF"
	case TokenCase:
		return "CASE"
	case TokenOf:
		return "OF"
	case TokenEndCase:
		return "END_CASE"
	case TokenFor:
		return "FOR"
	case TokenTo:
		return "TO"
	case TokenBy:
		return "BY"
	case TokenDo:
		return "DO"
	case TokenEndFor:
		return "END_FOR"
	case TokenWhile:
		return "WHILE"
	case TokenEndWhile:
		return "END_WHILE"
	case TokenRepeat:
		return "REPEAT"
	case TokenUntil:
		return "UNTIL"
	case TokenEndRepeat:
		return "END_REPEAT"
	case TokenReturn:
		return "RETURN"
	case TokenExit:
		return "EXIT"
	case TokenContinue:
		return "CONTINUE"
	case TokenJmp:
		return "JMP"
	case TokenTrue:
		return "TRUE"
	case TokenFalse:
		return "FALSE"
	case TokenAnd:
		return "AND"
	case TokenOr:
		return "OR"
	case TokenNot:
		return "NOT"
	case TokenXor:
		return "XOR"
	case TokenMod:
		return "MOD"
	case TokenAndThen:
		return "AND_THEN"
	case TokenOrElse:
		return "OR_ELSE"
	case TokenTypeKw:
		return "TYPE"
	case TokenEndType:
		return "END_TYPE"
	case TokenStruct:
		return "STRUCT"
	case TokenEndStruct:
		return "END_STRUCT"
	case TokenUnion:
		return "UNION"
	case TokenEndUnion:
		return "END_UNION"
	case TokenArray:
		return "ARRAY"
	case TokenRetain:
		return "RETAIN"
	case TokenNonRetain:
		return "NON_RETAIN"
	case TokenPersistent:
		return "PERSISTENT"
	case TokenConstant:
		return "CONSTANT"
	case TokenAt:
		return "AT"
	case TokenExtends:
		return "EXTENDS"
	case TokenImplements:
		return "IMPLEMENTS"
	case TokenAbstract:
		return "ABSTRACT"
	case TokenFinal:
		return "FINAL"
	case TokenInternal:
		return "INTERNAL"
	case TokenPrivate:
		return "PRIVATE"
	case TokenProtected:
		return "PROTECTED"
	case TokenPublic:
		return "PUBLIC"
	case TokenThis:
		return "THIS"
	case TokenSuper:
		return "SUPER"
	case TokenPointer:
		return "POINTER"
	case TokenReference:
		return "REFERENCE"
	case TokenRefTo:
		return "REF_TO"
	case TokenReadOnly:
		return "READ_ONLY"
	case TokenReadWrite:
		return "READ_WRITE"
	case TokenParams:
		return "PARAMS"
	case TokenBoolType:
		return "BOOL"
	case TokenByteType:
		return "BYTE"
	case TokenWordType:
		return "WORD"
	case TokenDwordType:
		return "DWORD"
	case TokenLwordType:
		return "LWORD"
	case TokenSintType:
		return "SINT"
	case TokenIntType:
		return "INT"
	case TokenDintType:
		return "DINT"
	case TokenLintType:
		return "LINT"
	case TokenUsintType:
		return "USINT"
	case TokenUintType:
		return "UINT"
	case TokenUdintType:
		return "UDINT"
	case TokenUlintType:
		return "ULINT"
	case TokenRealType:
		return "REAL"
	case TokenLrealType:
		return "LREAL"
	case TokenTimeType:
		return "TIME"
	case TokenLtimeType:
		return "LTIME"
	case TokenDateType:
		return "DATE"
	case TokenLdateType:
		return "LDATE"
	case TokenTodType:
		return "TIME_OF_DAY"
	case TokenLtodType:
		return "LTIME_OF_DAY"
	case TokenDtType:
		return "DATE_AND_TIME"
	case TokenLdtType:
		return "LDATE_AND_TIME"
	case TokenStringType:
		return "STRING"
	case TokenWstringType:
		return "WSTRING"
	case TokenCharType:
		return "CHAR"
	case TokenWcharType:
		return "WCHAR"
	}
	return ""
}

// TokenName returns the string representation of a token type for diagnostics.
func TokenName(tt TokenType) string {
	switch tt {
	case TokenEOF:
		return "EOF"
	case TokenIdent:
		return "IDENT"
	case TokenNumber:
		return "NUMBER"
	case TokenBasedNumber:
		return "BASED_NUMBER"
	case TokenString:
		return "STRING"
	case TokenTimeLiteral:
		return "TIME_LITERAL"
	case TokenTypedLiteral:
		return "TYPED_LITERAL"
	case TokenAddress:
		return "ADDRESS"
	case TokenAssign:
		return ":="
	case TokenOutputBind:
		return "=>"
	case TokenEqual:
		return "="
	case TokenNotEqual:
		return "<>"
	case TokenLess:
		return "<"
	case TokenLessEq:
		return "<="
	case TokenGreater:
		return ">"
	case TokenGreaterEq:
		return ">="
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenStar:
		return "*"
	case TokenSlash:
		return "/"
	case TokenPower:
		return "**"
	case TokenSemicolon:
		return ";"
	case TokenColon:
		return ":"
	case TokenComma:
		return ","
	case TokenDot:
		return "."
	case TokenDotDot:
		return ".."
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBracket:
		return "["
	case TokenRBracket:
		return "]"
	case TokenLBrace:
		return "{"
	case TokenRBrace:
		return "}"
	case TokenHash:
		return "#"
	case TokenCaret:
		return "^"
	case TokenPercent:
		return "%"
	case TokenSAssign:
		return "S="
	case TokenRAssign:
		return "R="
	case TokenRefAssign:
		return "REF="
	case TokenLineComment:
		return "LINE_COMMENT"
	case TokenBlockComment:
		return "BLOCK_COMMENT"
	default:
		if s := KeywordSpelling(tt); s != "" {
			return s
		}
		return "UNKNOWN"
	}
}
