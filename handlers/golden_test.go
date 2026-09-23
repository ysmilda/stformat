package handlers

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ysmilda/stformat/internal/lexer"
)

var update = flag.Bool("update", false, "update golden files")

// globInputs returns all testdata input files matching one of the given
// extensions, sorted by path for stable test order.
func globInputs(t *testing.T, exts ...string) []string {
	t.Helper()
	var files []string
	for _, ext := range exts {
		matches, err := filepath.Glob(filepath.Join("testdata", "*in."+ext))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatalf("no testdata inputs for %v", exts)
	}
	sort.Strings(files)
	return files
}

// goldenPath returns the golden counterpart of an input testdata file.
func goldenPath(input string) string {
	return strings.Replace(input, ".in.", ".golden.", 1)
}

// runGolden formats the testdata input with the handler and compares the
// result byte-for-byte against the golden file. With -update it writes the
// golden file instead. It also checks that formatting is idempotent.
func runGolden(t *testing.T, h Handler, input string) {
	t.Helper()
	in, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.Format(in)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	golden := goldenPath(input)
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("mismatch for %s\n%s", input, diffStrings(string(want), string(got)))
	}

	// Idempotency: reformatting the output must not change it.
	again, err := h.Format(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(got) {
		t.Errorf("not idempotent for %s", input)
	}
}

// assertTokensPreserved verifies that formatting only rearranges whitespace,
// keyword case and keyword aliases: every token must keep its type, and all
// non-keyword literals (identifiers, numbers, strings, pragmas) must be
// byte-identical. Comment tokens are exempt because the formatter normalises
// comment content (space and capital letter after the opening).
func assertTokensPreserved(t *testing.T, input, output string) {
	t.Helper()
	ti := tokensOf(input)
	to := tokensOf(output)
	if len(ti) != len(to) {
		t.Errorf("token count changed: %d -> %d", len(ti), len(to))
	}
	n := min(len(to), len(ti))
	for i := range n {
		a, b := ti[i], to[i]
		if a.Type != b.Type {
			t.Errorf("token %d type changed: %s -> %s", i, lexer.TokenName(a.Type), lexer.TokenName(b.Type))
			continue
		}
		// Keywords may be normalised in spelling and case (if -> IF, tod ->
		// TOD); the token type already carries their meaning.
		// Comments are normalised (space + capital letter) so their literal
		// may change.
		if lexer.KeywordSpelling(a.Type) == "" &&
			a.Type != lexer.TokenLineComment && a.Type != lexer.TokenBlockComment &&
			a.Literal != b.Literal {
			t.Errorf("token %d literal changed: %q -> %q", i, a.Literal, b.Literal)
		}
	}
}

// tokensOf lexes src and returns all tokens except the trailing EOF token.
func tokensOf(src string) []lexer.Token {
	toks := lexer.Lex(src)
	for len(toks) > 0 && toks[len(toks)-1].Type == lexer.TokenEOF {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// diffStrings renders a per-line side-by-side diff of two strings.
func diffStrings(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w == g {
			continue
		}
		fmt.Fprintf(&b, "  want: %q\n  got:  %q\n", w, g)
	}
	return b.String()
}
