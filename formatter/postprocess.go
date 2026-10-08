package formatter

import (
	"bytes"
	"strings"
)

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
	var (
		b        strings.Builder
		r        reflower
		wrapped  bool // set once a line has been reflowed
		bc       blockComments
		ignoring bool
		leading  = true // still inside the leading comment block
	)

	// The lines are walked in place rather than split into a []string. Every
	// line that is not reflowed is copied straight into the builder, and the
	// untouched text before the first reflow is copied in one go. A file whose
	// lines all fit therefore costs no allocation at all: the builder is never
	// created and the input is returned as it came in.
	pos := 0
	for {
		nl := strings.IndexByte(output[pos:], '\n')
		lineEnd, hasNL := len(output), nl >= 0
		if hasNL {
			lineEnd = pos + nl
		}
		line := output[pos:lineEnd]

		// Directive and block-comment state only move on a line that can carry a
		// comment delimiter. A single vectorised byte search decides whether
		// the two of them are worth looking at; running the searches per
		// delimiter instead costs more than the scans it saves, because the
		// call overhead dominates on lines of a few dozen bytes.
		if mayCarryComment(line) {
			switch lineDirective(line) {
			case directiveIgnore:
				ignoring = leading
			case directiveOff:
				ignoring = true
			case directiveOn:
				ignoring = false
			}
			bc.scan(line)
		}
		verbatim := ignoring
		if leading {
			if t := strings.TrimSpace(line); t != "" && !isCommentLine(t, false) {
				leading = false
			}
		}

		if r.reflow(line, verbatim, bc.open) {
			if !wrapped {
				b.Grow(len(output) + len(output)/4 + 64)
				b.WriteString(output[:pos])
				wrapped = true
			}
			b.Write(r.lines)
		} else if wrapped {
			b.WriteString(line)
		}

		if !hasNL {
			break
		}
		if wrapped {
			b.WriteByte('\n')
		}
		pos = lineEnd + 1
	}

	if !wrapped {
		return output
	}
	return b.String()
}

// --------------- ignored regions ---------------

// mayCarryComment reports whether line holds any of the bytes that a line
// comment, a block comment or a directive can be built from. A line with none
// of them cannot open or close a comment and cannot carry a directive.
func mayCarryComment(line string) bool {
	return strings.IndexByte(line, '/') >= 0 ||
		strings.IndexByte(line, '(') >= 0 ||
		strings.IndexByte(line, '*') >= 0
}

// blockComments tracks whether a line-by-line scan has entered a (* ... *)
// comment. A line can open or close one whatever its length, so every line has
// to be offered; only the lines that mention a delimiter are worth inspecting.
type blockComments struct{ open bool }

func (b *blockComments) scan(line string) {
	t := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(t, "(*"):
		b.open = !strings.HasSuffix(t, "*)")
	case b.open && strings.HasSuffix(t, "*)"):
		b.open = false
	}
}

// --------------- long-line wrapping ---------------

const maxLineLen = 120

// reflow lays out one over-long line into the scratch buffer and reports
// whether it did.
//
// A line is only a candidate when its code exceeds maxLineLen and carries a
// multi-argument function/FB call or a splittable IF/ELSIF condition.
// Comment-only lines, lines with an unterminated string literal and lines
// inside an ignored region are never reflowed. A comment at the end of a line
// does not count towards the length and stays at the end of the last line that
// the reflow produces.
func (r *reflower) reflow(line string, verbatim, inBlockComment bool) bool {
	// A line within the limit can never be reflowed, and every remaining check
	// walks it, so this shortcut carries most lines of a typical file.
	if len(line) <= maxLineLen {
		return false
	}
	t := strings.TrimSpace(line)
	if verbatim || isCommentLine(t, inBlockComment) {
		return false
	}
	code, comment := splitTrailingComment(line)
	if len(code) <= maxLineLen || hasOpenQuote(code) {
		return false
	}
	r.lines = r.lines[:0]
	if !r.wrapIf(code) && !r.wrapStatement(code) {
		return false
	}
	if comment != "" {
		// The comment counts for nothing towards the length and rides along at
		// the end of the last line the reflow produced.
		r.trimLastLine()
		r.lines = append(r.lines, "  "...)
		r.lines = append(r.lines, comment...)
	}
	return true
}

// trimLastLine drops the trailing blanks of the line that r.last points at.
func (r *reflower) trimLastLine() {
	r.lines = r.lines[:r.last+len(bytes.TrimRight(r.lines[r.last:], " \t"))]
}

// reflower builds the replacement text for one over-long line.
//
// The text goes into a scratch buffer that is reused for every line of the
// file, and the operators and arguments are found into reusable slices, so
// reflowing a file allocates nothing per line no matter how many lines it has
// to break.
type reflower struct {
	// lines holds the replacement lines joined by '\n', without a trailing
	// newline; last is the offset within it where the final line begins.
	lines []byte
	last  int
	// ops holds one buffer of top-level AND/OR/XOR offsets per nesting level,
	// and args the arguments of the call wrapStatement is breaking up.
	ops  [][]int
	args []string
	// indent is the line's own indentation extended with a tab per level, and
	// base is where the line's own indentation ends.
	indent []byte
	base   int
}

func (r *reflower) write(s string) {
	r.lines = append(r.lines, s...)
}

// newLine starts a new output line, recording where it begins so that a
// trailing comment can be attached to it. The leading '\n' is skipped for the
// first line of a reflow.
func (r *reflower) newLine() {
	if len(r.lines) > 0 {
		r.lines = append(r.lines, '\n')
	}
	r.last = len(r.lines)
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

// wrapStatement lays out a single long statement line by placing each argument
// of the outermost call on its own line. It reports whether it wrote a
// replacement.
func (r *reflower) wrapStatement(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}

	// Find the semicolon that ends the statement.
	semi := lastSemicolon(trimmed)
	if semi < 0 {
		return false
	}
	stmt := trimmed[:semi]
	suffix := trimmed[semi:] // ";" or ";..."

	// Find the last (...) in the statement — the outermost call.
	open, close := findLastCallParens(stmt)
	if open < 0 || close <= open {
		return false
	}

	// Only wrap genuine call argument lists: the '(' must directly follow
	// an identifier, ')' or ']' (e.g. `foo(`, `obj.foo(`). Parenthesized
	// sub-expressions and array-literal elements such as `[... (a := 1)]`
	// must not be treated as calls.
	if open == 0 {
		return false
	}
	if c := stmt[open-1]; !isIdentChar(c) && c != ')' && c != ']' {
		return false
	}

	// Extract arguments and split by top-level commas.
	args := r.splitArgs(stmt[open+1 : close])
	if len(args) <= 1 {
		return false
	}

	indent := lineIndent(line)
	callPrefix := strings.TrimSpace(stmt[:open])
	tail := stmt[close+1:] // content after the call's ')' up to the ';'

	r.write(indent)
	r.write(callPrefix)
	r.write("(")
	for i, a := range args {
		r.newLine()
		r.write(indent)
		r.write("\t")
		r.write(strings.TrimSpace(a))
		if i < len(args)-1 {
			r.write(",")
		}
	}
	r.newLine()
	r.write(indent)
	r.write(")")
	r.write(tail)
	r.write(suffix)
	return true
}

// splitArgs splits a call's argument list by the commas at depth 0, ignoring
// commas and brackets inside string literals, and returns the arguments in the
// scratch slice. A leading, trailing or doubled comma yields no empty
// argument.
func (r *reflower) splitArgs(s string) []string {
	args := r.args[:0]
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
				if arg := s[start:i]; arg != "" {
					args = append(args, arg)
				}
				start = i + 1
			}
		}
	}
	if arg := s[start:]; arg != "" {
		args = append(args, arg)
	}
	r.args = args
	return args
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

// --------------- long IF/ELSIF wrapping ---------------

// wrapIf lays out a long single-line IF/ELSIF header by placing each condition
// operand on its own indented line, splitting at top-level AND/OR/XOR
// operators. Every parenthesised operand is broken open: its operands sit one
// indent level deeper and the closing paren aligns with the operand level.
// Nested groups recurse. The THEN keyword moves to a line of its own at the
// indentation of the IF/ELSIF. It reports whether it wrote a replacement.
func (r *reflower) wrapIf(line string) bool {
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
		return false
	}

	cond, thenSuffix := findThenBoundary(rest)
	if thenSuffix == "" {
		return false
	}
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false
	}

	// The condition is splittable if it has depth-0 operators or is itself a
	// single parenthesised group whose contents can be laid out.
	if len(r.topLevelOps(cond, 0)) == 0 {
		if _, ok := parenGroup(cond); !ok {
			return false
		}
	}

	// The keyword shares its line with the first operand, so the operand lines
	// start one tab in and the first one is prefixed with the keyword.
	r.indent = append(r.indent[:0], lineIndent(line)...)
	r.base = len(r.indent)
	r.emitOperands(cond, kw, 1, true)
	if len(r.lines) == 0 {
		return false
	}
	// Trim the trailing blank of the operand that ends the condition, then put
	// THEN on a line of its own.
	r.trimLastLine()
	r.newLine()
	r.lines = append(r.lines, r.indentAt(0)...)
	r.write(strings.TrimSpace(thenSuffix))
	return true
}

// emitOperands lays out the operands of cond at the given nesting depth.
// inlineFirst moves the first operand onto the keyword line (the top level
// only); otherwise every operand starts on its own line at that depth, as do
// the closing parens of the operands that are parenthesised groups. The
// operands of such a group sit one level deeper.
func (r *reflower) emitOperands(cond, kw string, depth int, inlineFirst bool) {
	ops := r.topLevelOps(cond, depth)
	// The operands are the spans between the operators, walked in place.
	start := 0
	for i := range len(ops) + 1 {
		end := len(cond)
		if i < len(ops) {
			end = ops[i]
		}
		seg := strings.TrimSpace(cond[start:end])
		start = end

		if inner, lead, ok := parenOperand(seg); ok {
			if i == 0 && inlineFirst {
				r.lines = append(r.lines, r.indentAt(0)...)
				r.write(kw)
			} else {
				r.newLine()
				r.lines = append(r.lines, r.indentAt(depth)...)
			}
			r.write(lead)
			r.write("(")
			r.emitOperands(inner, "", depth+1, false)
			r.newLine()
			r.lines = append(r.lines, r.indentAt(depth)...)
			r.write(")")
			continue
		}
		if i == 0 && inlineFirst {
			r.lines = append(r.lines, r.indentAt(0)...)
			r.write(kw)
			r.write(seg)
			continue
		}
		r.newLine()
		r.lines = append(r.lines, r.indentAt(depth)...)
		r.write(seg)
	}
}

// indentAt returns the indentation for a nesting depth: the line's own
// indentation followed by one tab per level. It is a slice of a shared buffer,
// so it stays valid until the buffer has to grow; callers use it immediately
// and never hold it across a deeper level.
func (r *reflower) indentAt(depth int) []byte {
	for len(r.indent) < r.base+depth {
		r.indent = append(r.indent, '\t')
	}
	return r.indent[:r.base+depth]
}

// topLevelOps returns the byte offsets of the top-level AND, OR, and XOR
// keywords in s. Top-level means outside any parenthesised expressions or
// string literals.
//
// Each nesting level gets its own buffer, because the caller keeps walking the
// operands of one level while a deeper level is being laid out.
func (r *reflower) topLevelOps(s string, level int) []int {
	for len(r.ops) <= level {
		r.ops = append(r.ops, nil)
	}
	depth := 0
	ops := r.ops[level][:0]
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
				ops = append(ops, i)
			}
			i = j
			continue
		}
		i++
	}
	r.ops[level] = ops
	return ops
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
