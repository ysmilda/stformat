package lexer

import "fmt"

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
	TokenEndGet // END_GET of a property accessor
	TokenEndSet // END_SET of a property accessor
	TokenEndNamespace
	TokenEndConfiguration
	TokenEndResource
	TokenInitialStep // INITIAL_STEP, the SFC step a chart starts in
	TokenEndStep
	TokenEndTransition

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
	TokenVarGeneric
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
	TokenSemicolon // ;
	TokenColon     // :
	TokenComma     // ,
	TokenDot       // .
	TokenDotDot    // ..
	TokenHash      // #
	TokenCaret     // ^

	// Special
	TokenSAssign      // S= (TwinCAT set)
	TokenRAssign      // R= (TwinCAT reset)
	TokenRefAssign    // REF= (TwinCAT reference)
	TokenLineComment  // // ...
	TokenBlockComment // (* ... *)
	TokenPragma       // {attribute ...}
	TokenIllegal      // a character that cannot start any token

	tokenCount // number of token types; not a token
)

// Token represents a lexical token with its type, literal text, and source position.
type Token struct {
	Type    TokenType
	Literal string
	// Offset is the byte index of the first character of the token in the
	// source. It lets callers copy the original text of a token range
	// verbatim. The end of a token is implied by the Offset of the next one,
	// so no line/column bookkeeping is kept: nothing outside the lexer needs
	// it, and dropping it shrinks Token by a third.
	Offset int
}

// keywords lists every keyword and elementary type name with its token type.
// It is the single source of truth: the lookup the lexer uses, the canonical
// spelling of each keyword token and the set of type names are all derived
// from it. A token type listed more than once has aliases; the first spelling
// is the canonical one (TIME_OF_DAY, with TOD as its short form).
var keywords = []struct {
	spelling string
	tt       TokenType
}{
	// POU declarations
	{"PROGRAM", TokenProgram},
	{"END_PROGRAM", TokenEndProgram},
	{"FUNCTION", TokenFunction},
	{"END_FUNCTION", TokenEndFunction},
	{"FUNCTION_BLOCK", TokenFunctionBlock},
	{"END_FUNCTION_BLOCK", TokenEndFunctionBlock},
	{"ACTION", TokenAction},
	{"END_ACTION", TokenEndAction},
	{"METHOD", TokenMethod},
	{"END_METHOD", TokenEndMethod},
	{"PROPERTY", TokenProperty},
	{"END_PROPERTY", TokenEndProperty},
	{"CLASS", TokenClass},
	{"END_CLASS", TokenEndClass},
	{"INTERFACE", TokenInterface},
	{"END_INTERFACE", TokenEndInterface},
	{"END_GET", TokenEndGet},
	{"END_SET", TokenEndSet},

	// Namespaces, configurations and SFC. Their openers (NAMESPACE,
	// CONFIGURATION, RESOURCE, STEP, TRANSITION, GET, SET) are common
	// identifiers in PLC code, so the lexer leaves them identifiers and the
	// formatter recognises them by the closing keyword that follows.
	{"END_NAMESPACE", TokenEndNamespace},
	{"END_CONFIGURATION", TokenEndConfiguration},
	{"END_RESOURCE", TokenEndResource},
	{"INITIAL_STEP", TokenInitialStep},
	{"END_STEP", TokenEndStep},
	{"END_TRANSITION", TokenEndTransition},

	// Variable blocks
	{"VAR", TokenVar},
	{"VAR_INPUT", TokenVarInput},
	{"VAR_OUTPUT", TokenVarOutput},
	{"VAR_IN_OUT", TokenVarInOut},
	{"VAR_TEMP", TokenVarTemp},
	{"VAR_GLOBAL", TokenVarGlobal},
	{"VAR_EXTERNAL", TokenVarExternal},
	{"VAR_ACCESS", TokenVarAccess},
	{"VAR_CONFIG", TokenVarConfig},
	{"VAR_STAT", TokenVarStat},
	{"VAR_INST", TokenVarInst},
	{"VAR_GENERIC", TokenVarGeneric},
	{"END_VAR", TokenEndVar},

	// Control flow
	{"IF", TokenIf},
	{"THEN", TokenThen},
	{"ELSIF", TokenElsif},
	{"ELSE", TokenElse},
	{"END_IF", TokenEndIf},
	{"CASE", TokenCase},
	{"OF", TokenOf},
	{"END_CASE", TokenEndCase},
	{"FOR", TokenFor},
	{"TO", TokenTo},
	{"BY", TokenBy},
	{"DO", TokenDo},
	{"END_FOR", TokenEndFor},
	{"WHILE", TokenWhile},
	{"END_WHILE", TokenEndWhile},
	{"REPEAT", TokenRepeat},
	{"UNTIL", TokenUntil},
	{"END_REPEAT", TokenEndRepeat},
	{"RETURN", TokenReturn},
	{"EXIT", TokenExit},
	{"CONTINUE", TokenContinue},
	{"JMP", TokenJmp},

	// Boolean literals
	{"TRUE", TokenTrue},
	{"FALSE", TokenFalse},

	// Logical and arithmetic operators
	{"AND", TokenAnd},
	{"OR", TokenOr},
	{"NOT", TokenNot},
	{"XOR", TokenXor},
	{"MOD", TokenMod},
	{"AND_THEN", TokenAndThen},
	{"OR_ELSE", TokenOrElse},

	// Type system
	{"TYPE", TokenTypeKw},
	{"END_TYPE", TokenEndType},
	{"STRUCT", TokenStruct},
	{"END_STRUCT", TokenEndStruct},
	{"UNION", TokenUnion},
	{"END_UNION", TokenEndUnion},
	{"ARRAY", TokenArray},
	{"RETAIN", TokenRetain},
	{"NON_RETAIN", TokenNonRetain},
	{"PERSISTENT", TokenPersistent},
	{"CONSTANT", TokenConstant},
	{"AT", TokenAt},

	// OO
	{"EXTENDS", TokenExtends},
	{"IMPLEMENTS", TokenImplements},
	{"ABSTRACT", TokenAbstract},
	{"FINAL", TokenFinal},
	{"INTERNAL", TokenInternal},
	{"PRIVATE", TokenPrivate},
	{"PROTECTED", TokenProtected},
	{"PUBLIC", TokenPublic},
	{"THIS", TokenThis},
	{"SUPER", TokenSuper},

	// Pointer/Reference
	{"POINTER", TokenPointer},
	{"REFERENCE", TokenReference},
	{"REF_TO", TokenRefTo},

	// Access
	{"READ_ONLY", TokenReadOnly},
	{"READ_WRITE", TokenReadWrite},
	{"PARAMS", TokenParams},

	// Elementary types
	{"BOOL", TokenBoolType},
	{"BYTE", TokenByteType},
	{"WORD", TokenWordType},
	{"DWORD", TokenDwordType},
	{"LWORD", TokenLwordType},
	{"SINT", TokenSintType},
	{"INT", TokenIntType},
	{"DINT", TokenDintType},
	{"LINT", TokenLintType},
	{"USINT", TokenUsintType},
	{"UINT", TokenUintType},
	{"UDINT", TokenUdintType},
	{"ULINT", TokenUlintType},
	{"REAL", TokenRealType},
	{"LREAL", TokenLrealType},
	{"TIME", TokenTimeType},
	{"LTIME", TokenLtimeType},
	{"DATE", TokenDateType},
	{"LDATE", TokenLdateType},
	{"TIME_OF_DAY", TokenTodType},
	{"TOD", TokenTodType},
	{"LTIME_OF_DAY", TokenLtodType},
	{"LTOD", TokenLtodType},
	{"DATE_AND_TIME", TokenDtType},
	{"DT", TokenDtType},
	{"LDATE_AND_TIME", TokenLdtType},
	{"LDT", TokenLdtType},
	{"STRING", TokenStringType},
	{"WSTRING", TokenWstringType},
	{"CHAR", TokenCharType},
	{"WCHAR", TokenWcharType},
}

// keywordTypes maps the uppercase spelling of a keyword or type name to its
// token type, and keywordSpellings holds the canonical spelling of each
// keyword token type. Both are built from keywords.
var (
	keywordTypes     = make(map[string]TokenType, len(keywords))
	keywordSpellings [tokenCount]string
)

func init() {
	for _, k := range keywords {
		keywordTypes[k.spelling] = k.tt
		if keywordSpellings[k.tt] == "" {
			keywordSpellings[k.tt] = k.spelling
		}
	}
}

// lookupKeyword returns the token type of an uppercased identifier, and
// whether it is a keyword or an elementary type name at all. Indexing the map
// with a converted []byte does not allocate.
func lookupKeyword(upper []byte) (TokenType, bool) {
	tt, ok := keywordTypes[string(upper)]
	return tt, ok
}

// IsScalarTypeToken reports whether tt is an IEC 61131-3 elementary type
// (STRING, INT, BOOL, etc.).
func IsScalarTypeToken(tt TokenType) bool {
	return tt >= TokenBoolType && tt <= TokenWcharType
}

// IsBlockEnd reports whether tt is an END_* block terminator.
func IsBlockEnd(tt TokenType) bool {
	switch tt {
	case TokenEndProgram, TokenEndFunction, TokenEndFunctionBlock,
		TokenEndAction, TokenEndMethod, TokenEndProperty, TokenEndClass,
		TokenEndInterface, TokenEndGet, TokenEndSet, TokenEndNamespace,
		TokenEndConfiguration, TokenEndResource, TokenEndStep, TokenEndTransition,
		TokenEndVar, TokenEndIf, TokenEndCase, TokenEndFor,
		TokenEndWhile, TokenEndRepeat, TokenEndType, TokenEndStruct, TokenEndUnion:
		return true
	}
	return false
}

// IsComment reports whether tt is a comment or a pragma: text that is not
// read as a statement.
func IsComment(tt TokenType) bool {
	return tt == TokenLineComment || tt == TokenBlockComment || tt == TokenPragma
}

// KeywordSpelling returns the canonical uppercase spelling of a keyword token,
// or "" for a token that is not a keyword.
func KeywordSpelling(tt TokenType) string {
	if tt < 0 || tt >= tokenCount {
		return ""
	}
	return keywordSpellings[tt]
}

// tokenNames names the token types that are not keywords.
var tokenNames = [tokenCount]string{
	TokenEOF:          "EOF",
	TokenIdent:        "IDENT",
	TokenNumber:       "NUMBER",
	TokenBasedNumber:  "BASED_NUMBER",
	TokenString:       "STRING_LITERAL",
	TokenTimeLiteral:  "TIME_LITERAL",
	TokenTypedLiteral: "TYPED_LITERAL",
	TokenAddress:      "ADDRESS",
	TokenAssign:       ":=",
	TokenOutputBind:   "=>",
	TokenEqual:        "=",
	TokenNotEqual:     "<>",
	TokenLess:         "<",
	TokenLessEq:       "<=",
	TokenGreater:      ">",
	TokenGreaterEq:    ">=",
	TokenPlus:         "+",
	TokenMinus:        "-",
	TokenStar:         "*",
	TokenSlash:        "/",
	TokenPower:        "**",
	TokenLParen:       "(",
	TokenRParen:       ")",
	TokenLBracket:     "[",
	TokenRBracket:     "]",
	TokenSemicolon:    ";",
	TokenColon:        ":",
	TokenComma:        ",",
	TokenDot:          ".",
	TokenDotDot:       "..",
	TokenHash:         "#",
	TokenCaret:        "^",
	TokenSAssign:      "S=",
	TokenRAssign:      "R=",
	TokenRefAssign:    "REF=",
	TokenLineComment:  "LINE_COMMENT",
	TokenBlockComment: "BLOCK_COMMENT",
	TokenPragma:       "PRAGMA",
	TokenIllegal:      "ILLEGAL",
}

// String returns the name of a token type for diagnostics: its spelling for a
// keyword or an operator, and an upper-case name for anything else.
func (tt TokenType) String() string {
	if s := KeywordSpelling(tt); s != "" {
		return s
	}
	if tt >= 0 && tt < tokenCount && tokenNames[tt] != "" {
		return tokenNames[tt]
	}
	return fmt.Sprintf("TokenType(%d)", int(tt))
}
