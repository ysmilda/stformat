package formatter

import "strings"

// PostProcess applies line-level formatting rules after the main
// token-driven pass:
//  1. Colon alignment in VAR/STRUCT declaration blocks
//  2. Assignment (:=) alignment in consecutive assignment sequences
//  3. Long-line wrapping for function/FB calls with multiple arguments
//  4. Long-line wrapping for IF/ELSIF conditions at AND/OR/XOR operators,
//     recursively breaking open parenthesised operands
func PostProcess(output string) string {
	lines := strings.Split(output, "\n")
	lines = alignVarColons(lines)
	lines = alignAssignments(lines)
	lines = wrapLongLines(lines)
	return strings.Join(lines, "\n")
}

// --------------- colon alignment ---------------

// alignVarColons aligns the colon position across declarations
// inside VAR/STRUCT blocks.
func alignVarColons(lines []string) []string {
	inBlock := false
	blockStart := 0

	for i := range lines {
		t := strings.TrimSpace(lines[i])
		kw := blockKeyword(t)
		switch kw {
		case "var", "struct":
			if !inBlock {
				inBlock = true
				blockStart = i + 1
			}
		case "end":
			if inBlock {
				applyColonAlign(lines, blockStart, i)
				inBlock = false
			}
		}
	}
	return lines
}

// blockKeyword classifies a line as starting/ending a var/struct block.
func blockKeyword(t string) string {
	if t == "" {
		return ""
	}
	// VAR / VAR_INPUT / VAR_OUTPUT / ... / VAR_EXTERNAL
	if strings.HasPrefix(t, "VAR") {
		fields := strings.Fields(t)
		if len(fields) == 1 {
			return "var"
		}
		// VAR RETAIN, VAR CONSTANT etc. — still a var block
		return "var"
	}
	if strings.HasPrefix(t, "END_VAR") || strings.HasPrefix(t, "END_STRUCT") {
		return "end"
	}
	if t == "STRUCT" || t == "UNION" {
		return "struct"
	}
	return ""
}

// applyColonAlign aligns colons across declaration lines in [start, end).
func applyColonAlign(lines []string, start, end int) {
	// First pass: find the maximum colon column.
	maxColon := 0
	for i := start; i < end; i++ {
		idx := declColonIndex(lines[i])
		if idx > maxColon {
			maxColon = idx
		}
	}
	if maxColon == 0 {
		return
	}
	// Second pass: pad each line.
	for i := start; i < end; i++ {
		idx := declColonIndex(lines[i])
		if idx > 0 && idx < maxColon {
			pad := strings.Repeat(" ", maxColon-idx)
			lines[i] = lines[i][:idx] + pad + lines[i][idx:]
		}
	}
}

// declColonIndex returns the byte index of the first " : " in a
// declaration line, or 0 if not a declaration.
func declColonIndex(line string) int {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "(*") {
		return 0
	}
	// Must have an identifier at the start (the variable name).
	if len(t) > 0 && !isIdentStart(t[0]) {
		return 0
	}
	idx := strings.Index(line, " : ")
	if idx < 0 {
		idx = strings.Index(line, "  : ")
	}
	return idx
}

// --------------- assignment alignment ---------------

// alignAssignments aligns the := operator across consecutive
// assignment statements at the same indent level.
func alignAssignments(lines []string) []string {
	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if !isAssignmentLine(t) {
			i++
			continue
		}
		// Found start of a potential assignment group.
		indent := lineIndent(lines[i])
		groupStart := i
		for i < len(lines) {
			t = strings.TrimSpace(lines[i])
			curIndent := lineIndent(lines[i])
			if curIndent != indent || !isAssignmentLine(t) {
				break
			}
			i++
		}
		groupEnd := i
		if groupEnd-groupStart >= 2 {
			applyAssignmentAlign(lines, groupStart, groupEnd)
		}
	}
	return lines
}

// isAssignmentLine reports whether t is a simple assignment:
// `name[.field] ... := expr;`
func isAssignmentLine(t string) bool {
	if t == "" {
		return false
	}
	// Must start with an identifier.
	if !isIdentStart(t[0]) {
		return false
	}
	// Must contain := and end with ;
	before, _, ok := strings.Cut(t, " := ")
	if !ok {
		return false
	}
	// The LHS must be an identifier chain (name, name.field, name^, etc.).
	lhs := before
	for _, c := range lhs {
		if !isIdentChar(byte(c)) && c != '.' && c != '^' {
			return false
		}
	}
	return strings.HasSuffix(t, ";")
}

// applyAssignmentAlign pads the LHS of each line so := lines up.
func applyAssignmentAlign(lines []string, start, end int) {
	maxLHS := 0
	for i := start; i < end; i++ {
		t := strings.TrimSpace(lines[i])
		if idx := strings.Index(t, " := "); idx > maxLHS {
			maxLHS = idx
		}
	}
	for i := start; i < end; i++ {
		indent := lineIndent(lines[i])
		t := strings.TrimSpace(lines[i])
		if idx := strings.Index(t, " := "); idx > 0 && idx < maxLHS {
			pad := strings.Repeat(" ", maxLHS-idx)
			t = t[:idx] + pad + t[idx:]
			lines[i] = indent + t
		}
	}
}

// --------------- long-line wrapping ---------------

const maxLineLen = 120

// wrapLongLines splits lines that exceed maxLineLen and contain a
// multi-argument function/FB call. Comment lines are never wrapped.
func wrapLongLines(lines []string) []string {
	var result []string
	inBlockComment := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "(*") {
			inBlockComment = !strings.HasSuffix(t, "*)")
		} else if inBlockComment {
			if strings.HasSuffix(t, "*)") {
				inBlockComment = false
			}
		}
		if len(line) > maxLineLen && !isCommentLine(t, inBlockComment) {
			if wrapped := wrapIf(line); wrapped != nil {
				result = append(result, wrapped...)
				continue
			}
			if wrapped := wrapStatement(line); wrapped != nil {
				result = append(result, wrapped...)
				continue
			}
		}
		result = append(result, line)
	}
	return result
}

// isCommentLine reports whether a trimmed line is (part of) a comment.
func isCommentLine(t string, inBlockComment bool) bool {
	return inBlockComment ||
		strings.HasPrefix(t, "//") ||
		strings.HasPrefix(t, "(*") ||
		strings.Contains(t, "//")
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
	semi := strings.LastIndex(trimmed, ";")
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

// findLastCallParens finds the matching ( and ) of the last call
// in stmt. Returns (open, close) indices or (-1, -1).
func findLastCallParens(stmt string) (int, int) {
	close := -1
	for i := len(stmt) - 1; i >= 0; i-- {
		if stmt[i] == ')' {
			close = i
			break
		}
	}
	if close < 0 {
		return -1, -1
	}
	depth := 1
	for i := close - 1; i >= 0; i-- {
		switch stmt[i] {
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

// splitTopLevelArgs splits a string by commas at depth 0.
func splitTopLevelArgs(s string) []string {
	var args []string
	depth := 0
	start := 0
	for i := range len(s) {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, s[start:i])
				start = i + 1
			}
		}
	}
	if start <= len(s) {
		args = append(args, s[start:])
	}
	return args
}

// --------------- long IF/ELSIF wrapping ---------------

// wrapIf wraps a long single-line IF/ELSIF header by placing each condition
// operand on its own indented line, splitting at top-level AND/OR/XOR
// operators. Every parenthesised operand is broken open: its operands sit one
// indent level deeper and the closing paren aligns with the operand level.
// Nested groups recurse. Returns nil if the line is not an IF/ELSIF header
// or if the condition cannot be split.
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
	if len(lines) > 0 {
		lines[len(lines)-1] += thenSuffix
	}
	return lines
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
// treating doubled quote characters as an escaped quote.
func skipQuoted(s string, i int) int {
	quote := s[i]
	i++
	for i < len(s) {
		if s[i] == quote {
			if i+1 < len(s) && s[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
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
