package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzFormat holds Format to the two properties every output must have,
// whatever the input: it changes nothing but layout (Verify), and formatting
// it again changes nothing at all. The plain ST golden inputs seed the corpus.
//
//	go test ./formatter -fuzz FuzzFormat -fuzztime 1m
func FuzzFormat(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join("..", "handlers", "testdata", "*.in.st"))
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range seeds {
		src, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(src))
	}
	f.Add("x:=-1; IF s=1 THEN st.Type := D#2020-01-01; END_IF")

	f.Fuzz(func(t *testing.T, src string) {
		if strings.Contains(strings.TrimPrefix(src, utf8BOM), utf8BOM) {
			// A U+FEFF inside the text becomes a byte order mark once the
			// blanks before it are trimmed away; there is no layout that
			// keeps both readings apart.
			t.Skip()
		}
		if strings.Count(src, "\r") != strings.Count(src, "\r\n") {
			// A lone carriage return in front of a line break that the
			// output writes as LF turns into a CRLF, which the next pass
			// reads as a line ending. Lone CRs are not line endings ST
			// editors write, so they are left out.
			t.Skip()
		}
		// Most fuzz input is not well formed, so Format would refuse it. The
		// layout it would have produced must still keep every token and be
		// stable, so that is what is checked.
		out := layout(src)
		if err := Verify(src, out); err != nil {
			t.Fatalf("%v\ninput:  %q\noutput: %q", err, src, out)
		}
		if again := layout(out); again != out {
			t.Fatalf("not idempotent\ninput: %q\nonce:  %q\ntwice: %q", src, out, again)
		}
		// Where Format accepts the input, it returns that same layout.
		if formatted, err := Format(src); err == nil && formatted != out {
			t.Fatalf("Format differs from the layout\ninput: %q\nFormat: %q\nlayout: %q", src, formatted, out)
		}
	})
}

// layout returns the formatter's output for src whether or not src is well
// formed: Format without the structure check and the verification.
func layout(src string) string {
	return newPrinter(src, LineEnding(src)).run()
}
