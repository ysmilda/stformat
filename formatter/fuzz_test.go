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
		out := Format(src)
		if err := Verify(src, out); err != nil {
			t.Fatalf("%v\ninput:  %q\noutput: %q", err, src, out)
		}
		if again := Format(out); again != out {
			t.Fatalf("not idempotent\ninput: %q\nonce:  %q\ntwice: %q", src, out, again)
		}
	})
}
