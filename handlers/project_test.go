package handlers

// This file contains an opt-in integration test (TestProjectFormatLossless)
// that runs against a real ST/TwinCAT project tree.
//
// It is enabled by setting the STFORMAT_PROJECT_DIR environment variable to
// the directory containing the project, e.g. (PowerShell):
//
//	$env:STFORMAT_PROJECT_DIR = 'C:\path\to\project'
//	go test ./handlers -run TestProjectFormatLossless
//
// Every supported file in that tree is formatted in memory (files on disk
// are never modified) and the result is compared against the original. The
// check is deliberately independent of the internal lexer/parser: both the
// original and the formatted output are normalised by lowercasing every
// character and stripping all whitespace, and the two normalised strings
// must be equal. This verifies that formatting only rewrites whitespace and
// case; any removed, added or reordered content would survive normalisation
// and fail the test.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// projectDirEnv is the environment variable that points at a directory
// containing an ST/TwinCAT project.
const projectDirEnv = "STFORMAT_PROJECT_DIR"

// TestProjectFormatLossless formats every supported file under the directory
// named by projectDirEnv and verifies, without the internal lexer, that the
// only differences are whitespace and character case.
func TestProjectFormatLossless(t *testing.T) {
	t.Parallel()
	root := os.Getenv(projectDirEnv)
	if root == "" {
		t.Skipf("%s not set; skipping real-project format test", projectDirEnv)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("project dir %q: %v", root, err)
	}
	if !info.IsDir() {
		t.Fatalf("project dir %q is not a directory", root)
	}

	registry := NewRegistry()
	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, err := registry.ForExtension(strings.ToLower(filepath.Ext(path))); err == nil {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no supported source files found under %q", root)
	}
	sort.Strings(files)

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			ext := strings.ToLower(filepath.Ext(file))
			handler, err := registry.ForExtension(ext)
			if err != nil {
				t.Fatalf("no handler for %s: %v", file, err)
			}

			original, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			// Format in memory only; never write back to the project.
			formatted, err := handler.Format(original)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}

			if got, want := normalize(formatted), normalize(original); got != want {
				t.Errorf("formatting changed more than whitespace and case:\n%s", diffNormalized(want, got))
			}
		})
	}
}

// normalize returns src with every Unicode character lowercased and all
// whitespace removed.
func normalize(src []byte) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, string(src))
}

// diffNormalized describes where two normalized strings diverge. Normalization
// collapses whitespace, so the strings have no line breaks and a per-line diff
// would be useless. Instead it reports the byte offset of the first difference
// and the surrounding text of both sides, truncating long runs so failure
// output stays readable.
func diffNormalized(want, got string) string {
	n := min(len(want), len(got))
	i := 0
	for i < n && want[i] == got[i] {
		i++
	}
	j, k := len(want), len(got)
	for j > i && k > i && want[j-1] == got[k-1] {
		j--
		k--
	}
	return fmt.Sprintf(
		"first divergence at byte %d (want %d, got %d)\n  want: %s\n  got:  %s",
		i, len(want), len(got),
		clipDiff(want[i:j]),
		clipDiff(got[i:k]),
	)
}

// clipDiff quotes s, truncating runs longer than max chars with a byte-count
// marker so only the divergent window is shown.
func clipDiff(s string) string {
	const max = 100
	if len(s) <= max {
		return strconv.Quote(s)
	}
	return strconv.Quote(s[:max]) + fmt.Sprintf(" ... (+%d bytes)", len(s)-max)
}

// TestDiffNormalized checks that diffNormalized pinpoints the first divergence
// (trimming the common prefix and suffix) and clips oversized windows so
// failure output stays readable.
func TestDiffNormalized(t *testing.T) {
	t.Parallel()
	got := diffNormalized("abcdef", "abcXef")
	want := "first divergence at byte 3 (want 6, got 6)\n" +
		"  want: \"d\"\n  got:  \"X\""
	if got != want {
		t.Errorf("diffNormalized:\n got: %q\nwant: %q", got, want)
	}

	got = diffNormalized("a", "abc")
	want2 := "first divergence at byte 1 (want 1, got 3)\n" +
		"  want: \"\"\n  got:  \"bc\""
	if got != want2 {
		t.Errorf("diffNormalized prefix:\n got: %q\nwant: %q", got, want2)
	}

	got = diffNormalized(strings.Repeat("x", 250), "y")
	want3 := "first divergence at byte 0 (want 250, got 1)\n" +
		"  want: \"" + strings.Repeat("x", 100) + "\" ... (+150 bytes)\n" +
		"  got:  \"y\""
	if got != want3 {
		t.Errorf("diffNormalized clipped:\n got: %q\nwant: %q", got, want3)
	}
}
