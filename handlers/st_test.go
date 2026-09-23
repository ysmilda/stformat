package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSTFiles formats every testdata/*.in.{st,iecst,tcst} file and compares
// the result against its .golden. counterpart.
func TestSTFiles(t *testing.T) {
	t.Parallel()
	h := &STHandler{}
	for _, f := range globInputs(t, "st", "iecst", "tcst") {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			runGolden(t, h, f)
		})
	}
}

// TestSTOnlyChangesWhitespace verifies that formatting .st files never drops
// or rewrites any token: every change must be whitespace or keyword case.
func TestSTOnlyChangesWhitespace(t *testing.T) {
	t.Parallel()
	h := &STHandler{}
	for _, f := range globInputs(t, "st", "iecst", "tcst") {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			got, err := h.Format(data)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			assertTokensPreserved(t, string(data), string(got))
		})
	}
}
