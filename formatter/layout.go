package formatter

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/ysmilda/stformat/internal/lexer"
)

// The token pass writes every statement on one line. When a line ends, the
// tokens it holds are laid out again here: if the line is too wide, or the
// source broke it at a place where a break is allowed, the line is broken up.
// The decision works on the tokens the printer wrote, never on the text, so a
// string literal, a comment or a pragma can not be mistaken for structure.
//
// A line is read as a sequence of tokens in which brackets form lists (their
// items separated by commas) and binary operators form chains (their operands
// separated by the operators of the lowest precedence present). A sequence that
// does not fit is laid out by the first of these that applies:
//
//  1. Break open its bracketed lists, one item per line, and keep the rest on
//     the line. Which lists: every list the source already broke, or else the
//     last list that can be broken. A list with one item is not broken open;
//     its item is laid out in place instead, so F(G(a, b)) breaks G. Chosen
//     when the sequence has no operator chain, or when the lists break open
//     and the line that opens them and the line that closes them both fit.
//  2. Break the operator chain before each operator, one operand per line,
//     indented one level deeper than the line the chain starts on.
//  3. Leave it as it is: there is nothing left to break.
//
// Every line the layout produces is laid out by the same rules, so a long
// argument of a long call is broken too.
//
// A condition of IF, ELSIF or WHILE that is joined by AND, OR or XOR is laid
// out fully instead: one operand per line, every parenthesised operand broken
// open the same way one level deeper, and THEN or DO on a line of its own.
//
// Line breaks in the source are kept where they mean something: a break right
// after an opening bracket, around a comma or before a closing bracket keeps
// that list broken open, and a break before or after an operator keeps that
// chain broken. A break anywhere else inside a statement is dropped. The output
// of a broken list or chain has breaks in exactly those places, so formatting
// it again lays it out the same way.

// maxLineLen is the widest a line may be, in columns, before it is broken.
const maxLineLen = 120

// tabWidth is how many columns a tab takes, as in the TwinCAT editor.
const tabWidth = 4

// width returns how many columns s takes on screen: a tab counts tabWidth
// columns and a character counts one, however many bytes it is encoded in.
func width(s string) int {
	return utf8.RuneCountInString(s) + (tabWidth-1)*strings.Count(s, "\t")
}

// span is a token as the printer wrote it on the current output line: where it
// sits in the output and which token of the input it is.
type span struct{ start, end, tok int }

// atom is a token of a line being laid out.
type atom struct {
	text string
	// sep is the blank the token pass put before the token. A line break
	// replaces it.
	sep string
	typ lexer.TokenType
	// nl reports whether the source has a line break before the token.
	nl bool
	// level is the precedence level of a binary operator, lowest first, and 0
	// for any other token.
	level int
}

// opLevel returns the precedence level at which a chain breaks at an operator
// of type t, or 0 when it is no binary operator. The logical operators share a
// level so that a condition breaks at each of them alike.
func opLevel(t lexer.TokenType, binary bool) int {
	switch t {
	case lexer.TokenAnd, lexer.TokenOr, lexer.TokenXor, lexer.TokenAndThen, lexer.TokenOrElse:
		return 1
	case lexer.TokenEqual, lexer.TokenNotEqual, lexer.TokenLess, lexer.TokenLessEq,
		lexer.TokenGreater, lexer.TokenGreaterEq:
		return 2
	case lexer.TokenPlus, lexer.TokenMinus:
		if binary {
			return 3
		}
	case lexer.TokenStar, lexer.TokenSlash, lexer.TokenMod:
		return 4
	case lexer.TokenPower:
		return 5
	}
	return 0
}

// node is an element of a sequence: a token, or a bracketed list.
type node struct {
	a int // index of the token; unused for a list
	l *list
}

// list is a bracketed list: the tokens of its brackets and commas, and the
// items between them.
type list struct {
	open, close int
	items       [][]node
	commas      []int
	// broken reports whether the source breaks the line at one of the list's
	// own break points.
	broken bool
	// forcedMemo caches forced: 0 not known yet, 1 no, 2 yes.
	forcedMemo int8
	// canMemo caches canBreak in the same way.
	canMemo int8
}

// splittable reports whether the list can be broken open one item per line:
// it has items and none of them is empty.
func (l *list) splittable() bool {
	if len(l.items) == 0 {
		return false
	}
	for _, it := range l.items {
		if len(it) == 0 {
			return false
		}
	}
	return true
}

// opensOwnLines reports whether the list, once chosen to break, puts its items
// on lines of their own; otherwise its items are laid out in place.
func (l *list) opensOwnLines() bool {
	return l.splittable() && (l.broken || len(l.items) > 1)
}

// parse reads the atoms of a line into a sequence. stray is the position in the
// sequence of the first closing bracket that closes nothing on this line, or
// -1. A bracket left open at the end of the line is an ordinary token.
func parse(atoms []atom) (seq []node, stray int) {
	type frame struct {
		open   int
		items  [][]node
		commas []int
		cur    []node
	}
	stray = -1
	stack := []frame{{open: -1}}
	for i, a := range atoms {
		top := &stack[len(stack)-1]
		switch a.typ {
		case lexer.TokenLParen, lexer.TokenLBracket:
			stack = append(stack, frame{open: i})
			continue
		case lexer.TokenRParen, lexer.TokenRBracket:
			if top.open >= 0 && closes(atoms[top.open].typ, a.typ) {
				l := &list{open: top.open, close: i, items: append(top.items, top.cur), commas: top.commas}
				l.broken = atoms[l.open+1].nl || a.nl
				for _, c := range l.commas {
					l.broken = l.broken || atoms[c].nl || atoms[c+1].nl
				}
				stack = stack[:len(stack)-1]
				parent := &stack[len(stack)-1]
				parent.cur = append(parent.cur, node{l: l})
				continue
			}
			if top.open < 0 && stray < 0 {
				stray = len(top.cur)
			}
		case lexer.TokenComma:
			if top.open >= 0 {
				top.items = append(top.items, top.cur)
				top.commas = append(top.commas, i)
				top.cur = nil
				continue
			}
		}
		top.cur = append(top.cur, node{a: i})
	}
	// A list the line does not close is no list here: its tokens go back into
	// the sequence around it as they came.
	for len(stack) > 1 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		parent := &stack[len(stack)-1]
		parent.cur = append(parent.cur, node{a: f.open})
		for k, it := range f.items {
			parent.cur = append(parent.cur, it...)
			parent.cur = append(parent.cur, node{a: f.commas[k]})
		}
		parent.cur = append(parent.cur, f.cur...)
	}
	return stack[0].cur, stray
}

// closes reports whether a closing bracket of type c matches an opening one of
// type o.
func closes(o, c lexer.TokenType) bool {
	return (o == lexer.TokenLParen && c == lexer.TokenRParen) ||
		(o == lexer.TokenLBracket && c == lexer.TokenRBracket)
}

// renderer writes the layout of one line.
type renderer struct {
	atoms []atom
	out   []byte
	col   int
	// fresh is set at the start of a line, where the next token drops the
	// blank before it.
	fresh bool
}

func (r *renderer) text(s string) {
	r.out = append(r.out, s...)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		r.col = width(s[i+1:])
	} else {
		r.col += width(s)
	}
}

func (r *renderer) atom(i int) {
	if !r.fresh {
		r.text(r.atoms[i].sep)
	}
	r.text(r.atoms[i].text)
	r.fresh = false
}

func (r *renderer) newline(ind int) {
	r.out = append(r.out, '\n')
	for range ind {
		r.out = append(r.out, '\t')
	}
	r.col = ind * tabWidth
	r.fresh = true
}

// flat writes nodes on the current line.
func (r *renderer) flat(nodes []node) {
	for _, n := range nodes {
		if n.l == nil {
			r.atom(n.a)
			continue
		}
		r.atom(n.l.open)
		for k, it := range n.l.items {
			r.flat(it)
			if k < len(n.l.commas) {
				r.atom(n.l.commas[k])
			}
		}
		r.atom(n.l.close)
	}
}

// advance returns the column that s, written from col, ends at, and whether
// every line of it stays within the limit. The lines after a line break inside
// s, which only a multi-line token has, start in the first column.
func advance(col int, s string) (int, bool) {
	ok := true
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			col += width(s)
			return col, ok && col <= maxLineLen
		}
		ok = ok && col+width(s[:i]) <= maxLineLen
		col, s = 0, s[i+1:]
	}
}

// measure returns the column at which nodes, written flat from col, end and
// whether they stay within the limit. fresh says whether the first token starts
// a line.
func (r *renderer) measure(nodes []node, col int, fresh bool) (int, bool, bool) {
	ok := true
	step := func(i int) {
		if !fresh {
			var fit bool
			col, fit = advance(col, r.atoms[i].sep)
			ok = ok && fit
		}
		var fit bool
		col, fit = advance(col, r.atoms[i].text)
		ok = ok && fit
		fresh = false
	}
	var walk func([]node)
	walk = func(nodes []node) {
		for _, n := range nodes {
			if n.l == nil {
				step(n.a)
				continue
			}
			step(n.l.open)
			for k, it := range n.l.items {
				walk(it)
				if k < len(n.l.commas) {
					step(n.l.commas[k])
				}
			}
			step(n.l.close)
		}
	}
	walk(nodes)
	return col, ok, fresh
}

// fits reports whether nodes fit flat on the current line with after columns
// still to follow them.
func (r *renderer) fits(nodes []node, after int) bool {
	col, ok, _ := r.measure(nodes, r.col, r.fresh)
	return ok && col+after <= maxLineLen
}

// chain returns the positions in nodes of the operators that the sequence
// breaks at, and whether the source broke the line at one of them. An operator
// at either end of the sequence joins nothing on this line and is no break.
func (r *renderer) chain(nodes []node) (ops []int, broken bool) {
	lowest := 0
	for i := 1; i < len(nodes)-1; i++ {
		if nodes[i].l == nil {
			if lv := r.atoms[nodes[i].a].level; lv > 0 && (lowest == 0 || lv < lowest) {
				lowest = lv
			}
		}
	}
	if lowest == 0 {
		return nil, false
	}
	for i := 1; i < len(nodes)-1; i++ {
		if n := nodes[i]; n.l == nil && r.atoms[n.a].level == lowest {
			ops = append(ops, i)
			broken = broken || r.atoms[n.a].nl || r.atoms[n.a+1].nl
		}
	}
	return ops, broken
}

// operands calls fn for each operand between the operators at ops.
func operands(nodes []node, ops []int, fn func(k int, operand []node) bool) bool {
	start := 0
	for k := 0; k <= len(ops); k++ {
		end := len(nodes)
		if k < len(ops) {
			end = ops[k]
		}
		if fn(k, nodes[start:end]) {
			return true
		}
		start = end + 1
	}
	return false
}

// chainBroken reports whether the source broke an operator chain of the
// sequence, at its own level or inside one of its operands, lists aside.
func (r *renderer) chainBroken(nodes []node) bool {
	ops, broken := r.chain(nodes)
	if broken {
		return true
	}
	if len(ops) == 0 {
		return false
	}
	return operands(nodes, ops, func(_ int, operand []node) bool { return r.chainBroken(operand) })
}

// forced reports whether the source broke the sequence anywhere it keeps a
// break, so that it can not be written flat.
func (r *renderer) forced(nodes []node) bool {
	if r.chainBroken(nodes) {
		return true
	}
	for _, n := range nodes {
		if n.l != nil && r.listForced(n.l) {
			return true
		}
	}
	return false
}

func (r *renderer) listForced(l *list) bool {
	if l.forcedMemo == 0 {
		l.forcedMemo = 1
		if l.broken && l.splittable() {
			l.forcedMemo = 2
		}
		for _, it := range l.items {
			if l.forcedMemo == 1 && r.forced(it) {
				l.forcedMemo = 2
			}
		}
	}
	return l.forcedMemo == 2
}

// canBreak reports whether the sequence has anything to break.
func (r *renderer) canBreak(nodes []node) bool {
	if ops, _ := r.chain(nodes); len(ops) > 0 {
		return true
	}
	return len(r.targets(nodes)) > 0
}

// targets returns the positions of the lists that rule 1 breaks: every list
// that must break, or else the last list there is something to break in.
func (r *renderer) targets(nodes []node) []int {
	var must []int
	for i, n := range nodes {
		if n.l != nil && r.listForced(n.l) {
			must = append(must, i)
		}
	}
	if len(must) > 0 {
		return must
	}
	for i := len(nodes) - 1; i >= 0; i-- {
		l := nodes[i].l
		if l == nil || !l.splittable() {
			continue
		}
		if !l.opensOwnLines() {
			if l.canMemo == 0 {
				l.canMemo = 1
				if r.canBreak(l.items[0]) {
					l.canMemo = 2
				}
			}
			if l.canMemo == 1 {
				continue
			}
		}
		return []int{i}
	}
	return nil
}

// layout writes nodes from the current position. ind is the indentation of the
// line they start on and after the width of what must follow them on their
// last line.
func (r *renderer) layout(nodes []node, ind, after int) {
	if len(nodes) == 0 {
		return
	}
	if !r.forced(nodes) && r.fits(nodes, after) {
		r.flat(nodes)
		return
	}
	ops, _ := r.chain(nodes)
	if t := r.targets(nodes); len(t) > 0 && !r.chainBroken(nodes) &&
		(len(ops) == 0 || r.listsFit(nodes, t, ind, after)) {
		r.breakLists(nodes, t, ind, after)
		return
	}
	if len(ops) > 0 {
		r.breakChain(nodes, ops, ind, after)
		return
	}
	r.flat(nodes)
}

// listsFit reports whether breaking the lists at targets leaves the lines
// around them within the limit: the line that opens the first list, the lines
// between them and the line that closes the last one.
func (r *renderer) listsFit(nodes []node, targets []int, ind, after int) bool {
	col, fresh := r.col, r.fresh
	t := 0
	for i, n := range nodes {
		var ok bool
		if t < len(targets) && targets[t] == i {
			t++
			if _, ok, _ = r.measure([]node{{a: n.l.open}}, col, fresh); !ok {
				return false
			}
			if !n.l.opensOwnLines() {
				// The item would be laid out in place and break inside the
				// brackets, which reads worse than breaking the chain around
				// them.
				return false
			}
			col, _, fresh = r.measure([]node{{a: n.l.close}}, ind*tabWidth, true)
			continue
		}
		if col, ok, fresh = r.measure(nodes[i:i+1], col, fresh); !ok {
			return false
		}
	}
	return col+after <= maxLineLen
}

// breakLists writes nodes with the lists at targets broken open (rule 1).
func (r *renderer) breakLists(nodes []node, targets []int, ind, after int) {
	t := 0
	for i, n := range nodes {
		if t >= len(targets) || targets[t] != i {
			r.flat(nodes[i : i+1])
			continue
		}
		t++
		l := n.l
		r.atom(l.open)
		if !l.opensOwnLines() {
			// The items stay where they are and are laid out in place, with
			// what follows the list on this line counted after the last one.
			rest := after
			if t == len(targets) {
				end, _, _ := r.measure(append([]node{{a: l.close}}, nodes[i+1:]...), 0, false)
				rest += end
			} else {
				rest += width(r.atoms[l.close].text)
			}
			for k, it := range l.items {
				a := 1
				if k == len(l.items)-1 {
					a = rest
				}
				r.layout(it, ind, a)
				if k < len(l.commas) {
					r.atom(l.commas[k])
				}
			}
			r.atom(l.close)
			continue
		}
		for k, it := range l.items {
			r.newline(ind + 1)
			a := 0
			if k < len(l.commas) {
				a = 1
			}
			r.layout(it, ind+1, a)
			if k < len(l.commas) {
				r.atom(l.commas[k])
			}
		}
		r.newline(ind)
		r.atom(l.close)
	}
}

// breakChain writes nodes with a line break before each operator at ops
// (rule 2).
func (r *renderer) breakChain(nodes []node, ops []int, ind, after int) {
	operands(nodes, ops, func(k int, operand []node) bool {
		if k > 0 {
			r.newline(ind + 1)
			r.atom(nodes[ops[k-1]].a)
		}
		a := 0
		if k == len(ops) {
			a = after
		}
		r.layout(operand, ind+1, a)
		return false
	})
}

// condition lays out the line of an IF, ELSIF or WHILE whose condition is
// joined by logical operators or is one parenthesised group, and reports
// whether it did.
func (r *renderer) condition(code []node, ind int) bool {
	if len(code) < 3 || code[0].l != nil || code[len(code)-1].l != nil {
		return false
	}
	first, last := r.atoms[code[0].a].typ, r.atoms[code[len(code)-1].a].typ
	switch {
	case (first == lexer.TokenIf || first == lexer.TokenElsif) && last == lexer.TokenThen:
	case first == lexer.TokenWhile && last == lexer.TokenDo:
	default:
		return false
	}
	cond := code[1 : len(code)-1]
	if r.logicalOps(cond) == nil && r.parenOperand(cond) == nil {
		return false
	}
	r.atom(code[0].a)
	r.expand(cond, ind, 1, true)
	r.newline(ind)
	r.atom(code[len(code)-1].a)
	return true
}

// logicalOps returns the positions of the logical operators a condition is
// joined by, or nil when its lowest operators are of another kind.
func (r *renderer) logicalOps(cond []node) []int {
	if ops, _ := r.chain(cond); len(ops) > 0 && r.atoms[cond[ops[0]].a].level == 1 {
		return ops
	}
	return nil
}

// parenOperand returns the list when nodes are exactly one parenthesised
// expression, and nil otherwise.
func (r *renderer) parenOperand(nodes []node) *list {
	if len(nodes) != 1 || nodes[0].l == nil {
		return nil
	}
	l := nodes[0].l
	if r.atoms[l.open].typ != lexer.TokenLParen || len(l.items) != 1 || len(l.items[0]) == 0 {
		return nil
	}
	return l
}

// expand writes the operands of a condition one per line at depth levels below
// ind. The first one stays on the current line when inlineFirst is set.
func (r *renderer) expand(cond []node, ind, depth int, inlineFirst bool) {
	ops := r.logicalOps(cond)
	operands(cond, ops, func(k int, operand []node) bool {
		if k > 0 || !inlineFirst {
			r.newline(ind + depth)
		}
		if k > 0 {
			r.atom(cond[ops[k-1]].a)
		}
		if l := r.parenOperand(operand); l != nil {
			r.atom(l.open)
			r.expand(l.items[0], ind, depth+1, false)
			r.newline(ind + depth)
			r.atom(l.close)
			return false
		}
		r.layout(operand, ind+depth, 0)
		return false
	})
}

// continued lays out a line that ends a list an earlier line opened: its items
// one per line one level deeper and the closing bracket back at ind. stray is
// the position of that bracket. It reports whether it did.
func (r *renderer) continued(code []node, stray, ind int, fits bool) bool {
	var items [][]node
	var commas []int
	start := 0
	broken := r.atoms[code[stray].a].nl
	for i, n := range code[:stray] {
		if n.l == nil && r.atoms[n.a].typ == lexer.TokenComma {
			items = append(items, code[start:i])
			commas = append(commas, n.a)
			broken = broken || r.atoms[n.a].nl || r.atoms[n.a+1].nl
			start = i + 1
		}
	}
	items = append(items, code[start:stray])
	for _, it := range items {
		if len(it) == 0 {
			return false
		}
	}
	if !broken && (fits || len(items) < 2) {
		return false
	}
	r.out = r.out[:0]
	r.newline(ind + 1)
	r.out = r.out[1:] // the line break before the line is already written
	for k, it := range items {
		if k > 0 {
			r.newline(ind + 1)
		}
		a := 0
		if k < len(commas) {
			a = 1
		}
		r.layout(it, ind+1, a)
		if k < len(commas) {
			r.atom(commas[k])
		}
	}
	r.newline(ind)
	r.atom(code[stray].a)
	r.layout(code[stray+1:], ind, 0)
	return true
}

// endLine lays out the line the printer has just finished, if it has to be,
// and forgets its tokens.
func (f *printer) endLine() {
	if !f.frozen && len(f.spans) > 0 {
		// The blank written after a binary operator belongs to the operand
		// that follows. When none follows on this line (a dangling a *), it
		// would trail the line, and only a line that is laid out again would
		// lose it, so formatting twice would differ.
		last := f.spans[len(f.spans)-1].end
		if len(f.buf) > last && len(bytes.TrimRight(f.buf[last:], " \t")) == 0 {
			f.buf = f.buf[:last]
			f.lastCh = f.buf[last-1]
			if last > 1 {
				f.prevCh = f.buf[last-2]
			}
		}
		f.layoutLine()
	}
	f.spans = f.spans[:0]
	f.frozen = false
}

// newline ends the current line and starts the next one.
func (f *printer) newline() {
	f.endLine()
	f.writeCh('\n')
	f.lineBegin = len(f.buf)
}

// layoutLine breaks up the current line when it is too wide or the source
// broke it where a break is kept, and rewrites it in the output.
func (f *printer) layoutLine() {
	spans := f.spans
	// Brackets left open carry over to the lines after, so that a line which
	// closes one knows it continues a list.
	opens, closes := 0, 0
	for _, s := range spans {
		switch f.tokens[s.tok].Type {
		case lexer.TokenLParen, lexer.TokenLBracket:
			opens++
		case lexer.TokenRParen, lexer.TokenRBracket:
			if opens > 0 {
				opens--
			} else {
				closes++
			}
		}
	}
	carried := f.carry
	f.carry = max(0, f.carry-closes) + opens

	first := spans[0].start
	for _, c := range f.buf[f.lineBegin:first] {
		if c != '\t' {
			return
		}
	}
	ind := first - f.lineBegin
	// A comment ends the line it is on; it counts for nothing towards the
	// width and stays at the end of the last line.
	n := len(spans)
	if lexer.IsComment(f.tokens[spans[n-1].tok].Type) {
		n--
	}
	if n == 0 {
		return
	}
	code := f.buf[first:spans[n-1].end]
	fits := ind*tabWidth+utf8.RuneCount(code)+(tabWidth-1)*bytes.Count(code, []byte{'\t'}) <= maxLineLen
	if bytes.IndexByte(code, '\n') >= 0 {
		_, fits = advance(ind*tabWidth, string(code))
	}
	if fits {
		// Only a break next to a bracket, a comma or an operator can be kept,
		// so a line without one is done.
		broken := false
		for i, s := range spans[1:n] {
			if f.breakBefore(s.tok) && (keepsBreak(f.tokens[s.tok].Type) || keepsBreak(f.tokens[spans[i].tok].Type)) {
				broken = true
				break
			}
		}
		if !broken {
			return
		}
	}

	// One copy of the line; the atoms slice their text out of it.
	line := string(f.buf[f.lineBegin:])
	at := func(off int) int { return off - f.lineBegin }
	atoms := make([]atom, n)
	prevEnd := first
	for i, s := range spans[:n] {
		t := f.tokens[s.tok]
		if lexer.IsComment(t.Type) {
			return
		}
		sep := line[at(prevEnd):at(s.start)]
		if strings.Trim(sep, " ") != "" {
			return
		}
		atoms[i] = atom{
			text:  line[at(s.start):at(s.end)],
			sep:   sep,
			typ:   t.Type,
			nl:    i > 0 && f.breakBefore(s.tok),
			level: opLevel(t.Type, f.afterOperandAt(s.tok)),
		}
		prevEnd = s.end
	}
	seq, stray := parse(atoms)
	r := &renderer{atoms: atoms, col: ind * tabWidth, fresh: true}
	r.out = append(r.out, f.buf[f.lineBegin:first]...)
	switch {
	case carried > 0 && stray >= 0 && r.continued(seq, stray, ind, fits):
	case !fits || r.forced(seq):
		if !r.condition(seq, ind) {
			r.layout(seq, ind, 0)
		}
	default:
		return
	}
	if n < len(spans) {
		r.out = append(r.out, f.buf[spans[n-1].end:]...)
	}
	if bytes.Equal(r.out, f.buf[f.lineBegin:]) {
		return
	}
	f.buf = append(f.buf[:f.lineBegin], r.out...)
	if len(f.buf) > 1 {
		f.prevCh = f.buf[len(f.buf)-2]
	}
}

// keepsBreak reports whether a line break next to a token of type t may be one
// that the layout keeps.
func keepsBreak(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenLParen, lexer.TokenLBracket, lexer.TokenRParen, lexer.TokenRBracket, lexer.TokenComma:
		return true
	}
	return opLevel(t, true) > 0
}

// breakBefore reports whether the source has a line break right before token i.
func (f *printer) breakBefore(i int) bool {
	if i == 0 {
		return false
	}
	prev, tok := f.tokens[i-1], f.tokens[i]
	return strings.IndexByte(f.source[prev.Offset+len(prev.Literal):tok.Offset], '\n') >= 0
}
