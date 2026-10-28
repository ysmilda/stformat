package handlers

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ysmilda/stformat/formatter"
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
// golden file instead.
//
// It also holds the output to the two properties that do not depend on the
// golden bytes: a second pass changes nothing, and formatting dropped no token.
// The golden says what the output must look like; these say it is allowed to
// look that way. Both used to be separate tests looping over the same files,
// which formatted every fixture twice over for no extra coverage.
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

	if *update {
		if err := os.WriteFile(goldenPath(input), got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath(input))
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

	// Nothing lost: for an XML file that is the ST inside the CDATA sections,
	// for a plain ST file the whole text.
	if _, isXML := h.(*XMLHandler); !isXML {
		assertTokensPreserved(t, string(in), string(got))
		return
	}
	before, after := extractSTBlocks(in), extractSTBlocks(got)
	if len(before) != len(after) {
		t.Fatalf("ST block count changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		assertTokensPreserved(t, before[i], after[i])
	}
}

// assertTokensPreserved verifies with formatter.Verify that formatting only
// changed whitespace, keyword case and comment spacing.
func assertTokensPreserved(t *testing.T, input, output string) {
	t.Helper()
	if msg := tokensPreserved(input, output); msg != "" {
		t.Error(msg)
	}
}

// tokensPreserved is assertTokensPreserved as a function that describes the
// first violation instead of failing on a test, so that the property sweep can
// shrink an input down to what makes it fail.
func tokensPreserved(input, output string) string {
	if err := formatter.Verify(input, output); err != nil {
		return err.Error() + "\n" + diffStrings(input, output)
	}
	return ""
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
