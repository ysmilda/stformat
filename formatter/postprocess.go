package formatter

import "strings"

// PostProcess applies line-level formatting rules after the main
// token-driven pass:
//  1. Long-line wrapping for function/FB calls with multiple arguments
//  2. Long-line wrapping for IF/ELSIF conditions at AND/OR/XOR operators,
//     recursively breaking open parenthesised operands
//
// Nothing is aligned: declarations and assignments keep the single space that
// the token pass put around ':' and ':='. A comment at the end of a line never
// counts towards the line length, and lines inside a "stformat:off" region are
// left exactly as the token pass emitted them.
func PostProcess(output string) string {
	lines := strings.Split(output, "\n")
	return strings.Join(wrapLongLines(lines, verbatimLines(lines)), "\n")
}

// --------------- ignored regions ---------------

// verbatimLines marks the lines that belong to a formatting directive:
// everything from a "stformat:off" line up to (but excluding) its
// "stformat:on" line, and everything from a file-level "stformat:ignore" to
// the end of the file. Those lines are never reflowed. Like the token pass, a
// "stformat:ignore" only counts in the comment block at the top.
func verbatimLines(lines []string) []bool {
	marked := make([]bool, len(lines))
	ignoring, leading := false, true
	for i, line := range lines {
		switch lineDirective(line) {
		case directiveIgnore:
			ignoring = leading
		case directiveOff:
			ignoring = true
		case directiveOn:
			ignoring = false
		}
		marked[i] = ignoring
		if leading {
			if t := strings.TrimSpace(line); t != "" && !isCommentLine(t, false) {
				leading = false
			}
		}
	}
	return marked
}

// isVerbatim reports whether line i was copied from the source unchanged.
func isVerbatim(verbatim []bool, i int) bool {
	return i < len(verbatim) && verbatim[i]
}

// --------------- long-line wrapping ---------------

const maxLineLen = 120

// wrapLongLines splits lines whose code exceeds maxLineLen and that contain a
// multi-argument function/FB call or a splittable IF/ELSIF condition.
// Comment-only lines, lines with an unterminated string literal and lines
// inside an ignored region are never wrapped. A comment at the end of a line
// does not count towards the length and stays at the end of the last line
// that the wrap produces.
func wrapLongLines(lines []string, verbatim []bool) []string {
	var result []string
	inBlockComment := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "(*") {
			inBlockComment = !strings.HasSuffix(t, "*)")
		} else if inBlockComment && strings.HasSuffix(t, "*)") {
			inBlockComment = false
		}
		code, comment := splitTrailingComment(line)
		if isVerbatim(verbatim, i) || isCommentLine(t, inBlockComment) ||
			hasOpenQuote(code) || len(code) <= maxLineLen {
			result = append(result, line)
			continue
		}
		if wrapped := wrapIf(code); wrapped != nil {
			result = append(result, withTrailingComment(wrapped, comment)...)
			continue
		}
		if wrapped := wrapStatement(code); wrapped != nil {
			result = append(result, withTrailingComment(wrapped, comment)...)
			continue
		}
		result = append(result, line)
	}
	return result
}

// isCommentLine reports whether a trimmed line is (part of) a comment.
func isCommentLine(t string, inBlockComment bool) bool {
	return inBlockComment ||
		strings.HasPrefix(t, "//") ||
		strings.HasPrefix(t, "(*")
}

// splitTrailingComment splits a line into its code and the comment that trails
// it. Either part can be empty. A "//" or "(*" inside a string literal does not
// start a comment.
func splitTrailingComment(line string) (code, comment string) {
	code, comment = line, ""
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'', '"':
			i = skipQuoted(line, i) - 1
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				code, comment = line[:i], line[i:]
				return trimBothEnds(code, comment)
			}
		case '(':
			if i+1 < len(line) && line[i+1] == '*' &&
				strings.HasSuffix(strings.TrimSpace(line), "*)") {
				code, comment = line[:i], line[i:]
				return trimBothEnds(code, comment)
			}
		}
	}
	return code, comment
}

// trimBothEnds removes trailing blanks from code and the surrounding blanks
// from comment.
func trimBothEnds(code, comment string) (string, string) {
	return strings.TrimRight(code, " \t"), strings.TrimSpace(comment)
}

// withTrailingComment appends comment to the last line of wrapped, separated by
// two spaces. An empty comment leaves wrapped untouched.
func withTrailingComment(wrapped []string, comment string) []string {
	if comment == "" || len(wrapped) == 0 {
		return wrapped
	}
	last := len(wrapped) - 1
	wrapped[last] = strings.TrimRight(wrapped[last], " \t") + "  " + comment
	return wrapped
}

// hasOpenQuote reports whether s ends inside an unterminated string literal,
// i.e. the literal continues on the next line and must not be reflowed.
func hasOpenQuote(s string) bool {
	for i := 0; i < len(s); {
		if s[i] != '\'' && s[i] != '"' {
			i++
			continue
		}
		end := skipQuoted(s, i)
		if end > len(s) {
			return true
		}
		i = end
	}
	return false
}

// wrapStatement attempts to wrap a single long statement line by
// placing each argument of the outermost call on its own line.
// Returns nil if wrapping is not applicable.
func wrapStatement(line string) []string {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil
	}

	// Find the semicolon that ends the statement.
	semi := lastSemicolon(trimmed)
	if semi < 0 {
		return nil
	}
	stmt := trimmed[:semi]
	suffix := trimmed[semi:] // ";" or ";..."

	// Find the last (...) in the statement — the outermost call.
	open, close := findLastCallParens(stmt)
	if open < 0 || close <= open {
		return nil
	}

	// Only wrap genuine call argument lists: the '(' must directly follow
	// an identifier, ')' or ']' (e.g. `foo(`, `obj.foo(`). Parenthesized
	// sub-expressions and array-literal elements such as `[... (a := 1)]`
	// must not be treated as calls.
	if open == 0 {
		return nil
	}
	if c := stmt[open-1]; !isIdentChar(c) && c != ')' && c != ']' {
		return nil
	}

	// Extract arguments and split by top-level commas.
	argsStr := stmt[open+1 : close]
	args := splitTopLevelArgs(argsStr)
	if len(args) <= 1 {
		return nil
	}

	indent := lineIndent(line)
	callPrefix := strings.TrimSpace(stmt[:open])
	tail := stmt[close+1:] // content after the call's ')' up to the ';'

	var result []string
	result = append(result, indent+callPrefix+"(")
	for i, a := range args {
		a = strings.TrimSpace(a)
		if i < len(args)-1 {
			result = append(result, indent+"\t"+a+",")
		} else {
			result = append(result, indent+"\t"+a)
		}
	}
	result = append(result, indent+")"+tail+suffix)
	return result
}

// lastSemicolon returns the index of the last ';' outside a string literal, or
// -1 when there is none. Scanning backwards means a literal is walked once per
// ';' or ')' inside it, which is irrelevant at maxLineLen but would matter if
// that limit ever became configurable.
func lastSemicolon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '\'', '"':
			i = skipQuotedStart(s, i) - 1
		case ';':
			return i
		}
	}
	return -1
}

// skipQuotedStart returns the index just before the opening quote of the
// literal that ends at index i.
func skipQuotedStart(s string, i int) int {
	quote := s[i]
	for j := i - 1; j >= 0; j-- {
		if s[j] != quote {
			continue
		}
		// A doubled quote is an escaped quote, not the opening one.
		if j > 0 && s[j-1] == quote {
			j--
			continue
		}
		return j
	}
	return 0
}

// findLastCallParens finds the matching ( and ) of the last call
// in stmt. Returns (open, close) indices or (-1, -1).
func findLastCallParens(stmt string) (int, int) {
	for i := len(stmt) - 1; i >= 0; i-- {
		switch stmt[i] {
		case '\'', '"':
			i = skipQuotedStart(stmt, i) - 1
		case ')':
			return matchParens(stmt, i)
		}
	}
	return -1, -1
}

// matchParens walks back from the ')' at close to its opening '('.
func matchParens(stmt string, close int) (int, int) {
	depth := 1
	for i := close - 1; i >= 0; i-- {
		switch stmt[i] {
		case '\'', '"':
			i = skipQuotedStart(stmt, i) - 1
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i, close
			}
		}
	}
	return -1, -1
}

// splitTopLevelArgs splits a string by commas at depth 0, ignoring commas and
// brackets inside string literals. A leading or trailing comma yields no empty
// argument.
func splitTopLevelArgs(s string) []string {
	var args []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'', '"':
			// Jump past the literal; it may contain commas and brackets.
			i = skipQuoted(s, i) - 1
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = appendArg(args, s[start:i])
				start = i + 1
			}
		}
	}
	return appendArg(args, s[start:])
}

// appendArg adds one argument, skipping an empty one so that a leading,
// trailing or doubled comma does not produce a blank line.
func appendArg(args []string, arg string) []string {
	if arg == "" {
		return args
	}
	return append(args, arg)
}

// --------------- long IF/ELSIF wrapping ---------------

// wrapIf wraps a long single-line IF/ELSIF header by placing each condition
// operand on its own indented line, splitting at top-level AND/OR/XOR
// operators. Every parenthesised operand is broken open: its operands sit one
// indent level deeper and the closing paren aligns with the operand level.
// Nested groups recurse. The THEN keyword moves to a line of its own at the
// indentation of the IF/ELSIF. Returns nil if the line is not an IF/ELSIF
// header or if the condition cannot be split.
func wrapIf(line string) []string {
	trimmed := strings.TrimSpace(line)

	var kw, rest string
	switch {
	case strings.HasPrefix(trimmed, "IF "):
		kw = "IF "
		rest = trimmed[3:]
	case strings.HasPrefix(trimmed, "ELSIF "):
		kw = "ELSIF "
		rest = trimmed[6:]
	default:
		return nil
	}

	cond, thenSuffix := findThenBoundary(rest)
	if thenSuffix == "" {
		return nil
	}
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return nil
	}

	// The condition is splittable if it has depth-0 operators or is itself a
	// single parenthesised group whose contents can be laid out.
	if len(findTopLevelOps(cond)) == 0 {
		if _, ok := parenGroup(cond); !ok {
			return nil
		}
	}

	indent := lineIndent(line)
	lines := emitOperands(cond, indent+kw, indent+"\t", true)
	if len(lines) == 0 {
		return nil
	}
	lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], " \t")
	return append(lines, indent+strings.TrimSpace(thenSuffix))
}

// emitOperands lays out the operands of cond. inlineFirst moves the first
// operand onto the firstLinePrefix line (the keyword line at the top level);
// otherwise every operand starts on a fresh line at opLevel (paren content).
// opLevel is the indentation of operand lines and closing parens; the operands
// of a parenthesised group sit one level deeper.
func emitOperands(cond, firstLinePrefix, opLevel string, inlineFirst bool) []string {
	ops := findTopLevelOps(cond)
	operands := splitTopLevelOperands(cond, ops)
	var lines []string
	for i, seg := range operands {
		open := opLevel
		if i == 0 && inlineFirst {
			open = firstLinePrefix
		}
		if inner, lead, ok := parenOperand(seg); ok {
			lines = append(lines, open+lead+"(")
			lines = append(lines, emitOperands(inner, "", opLevel+"\t", false)...)
			lines = append(lines, opLevel+")")
			continue
		}
		lines = append(lines, open+seg)
	}
	return lines
}

// splitTopLevelOperands splits cond at its top-level operators. With no split
// points the whole condition is returned as a single operand.
func splitTopLevelOperands(cond string, ops []int) []string {
	if len(ops) == 0 {
		return []string{strings.TrimSpace(cond)}
	}
	operands := make([]string, 0, len(ops)+1)
	operands = append(operands, strings.TrimSpace(cond[:ops[0]]))
	for i := 1; i < len(ops); i++ {
		operands = append(operands, strings.TrimSpace(cond[ops[i-1]:ops[i]]))
	}
	operands = append(operands, strings.TrimSpace(cond[ops[len(ops)-1]:]))
	return operands
}

// parenOperand reports whether seg is a single parenthesised operand,
// optionally preceded by an AND/OR/XOR operator. It returns the content
// between the outer parens and the leading operator.
func parenOperand(seg string) (inner, lead string, ok bool) {
	s := strings.TrimSpace(seg)
	for _, op := range []string{"AND", "OR", "XOR"} {
		if s == op || strings.HasPrefix(s, op+" ") {
			lead = op + " "
			s = strings.TrimSpace(s[len(op):])
			break
		}
	}
	content, ok := parenGroup(s)
	return content, lead, ok
}

// parenGroup verifies that s is exactly one balanced pair of parentheses
// (skipping string literals) and returns the text between them.
func parenGroup(s string) (inner string, ok bool) {
	if len(s) < 2 || s[0] != '(' {
		return "", false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' || s[i] == '"' {
			i = skipQuoted(s, i) - 1
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return "", false
			}
			if depth == 0 {
				if i != len(s)-1 {
					return "", false
				}
				return strings.TrimSpace(s[1:i]), true
			}
		}
	}
	return "", false
}

// findThenBoundary locates the first " THEN" token at depth 0 and splits the
// string around it. The suffix includes the leading space and everything from
// " THEN" to the end. Returns ("", "") when THEN is not found.
func findThenBoundary(s string) (cond, suffix string) {
	depth := 0
	for i := 0; i+5 <= len(s); i++ {
		if s[i] == '\'' || s[i] == '"' {
			i = skipQuoted(s, i) - 1
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && s[i:i+5] == " THEN" {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// findTopLevelOps returns the byte offsets of top-level AND, OR, and XOR
// keywords in s. Top-level means outside any parenthesised expressions or
// string literals.
func findTopLevelOps(s string) []int {
	depth := 0
	var positions []int
	i := 0
	for i < len(s) {
		if s[i] == '\'' || s[i] == '"' {
			i = skipQuoted(s, i)
			continue
		}
		switch s[i] {
		case '(', '[':
			depth++
			i++
			continue
		case ')', ']':
			if depth > 0 {
				depth--
			}
			i++
			continue
		}
		if depth == 0 && isIdentStart(s[i]) {
			j := i
			for j < len(s) && isIdentChar(s[j]) {
				j++
			}
			w := s[i:j]
			if (w == "AND" || w == "OR" || w == "XOR") &&
				(i == 0 || s[i-1] == ' ') &&
				(j >= len(s) || s[j] == ' ') {
				positions = append(positions, i)
			}
			i = j
			continue
		}
		i++
	}
	return positions
}

// skipQuoted returns the index just past the quoted string starting at i,
// treating doubled quote characters and a dollar escape as part of the
// literal. It returns a value greater than len(s) when the literal is
// unterminated.
func skipQuoted(s string, i int) int {
	quote := s[i]
	i++
	for i < len(s) {
		switch {
		case s[i] == '$' && i+1 < len(s):
			i += 2
		case s[i] == quote:
			if i+1 < len(s) && s[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		default:
			i++
		}
	}
	return len(s) + 1
}

// --------------- helpers ---------------

func lineIndent(line string) string {
	for i := range len(line) {
		if line[i] != ' ' && line[i] != '\t' {
			return line[:i]
		}
	}
	return line
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentChar(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}
