package handlers

import (
	"path/filepath"
	"testing"
)

// TestSTFiles formats every testdata/*.in.{st,iecst,tcst} file and compares
// the result against its .golden. counterpart.
//
// runGolden also checks idempotency and that no token was dropped, so this is
// the whole check for a plain ST fixture. The TwinCAT half is TestXMLFiles,
// which differs only in which extensions it globs and in how the ST is reached.
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
