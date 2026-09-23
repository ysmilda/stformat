package handlers

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestXMLFiles formats every testdata/*.in.{tcpou,tcgvl,tcdut,tcio} file and
// compares the result against its .golden. counterpart.
func TestXMLFiles(t *testing.T) {
	t.Parallel()
	h := &XMLHandler{}
	for _, f := range globInputs(t, "tcpou", "tcgvl", "tcdut", "tcio") {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			runGolden(t, h, f)
		})
	}
}

// TestXMLOnlyChangesWhitespace verifies that the ST inside TwinCAT XML files
// is never rewritten semantically: each Declaration/ST CDATA block must keep
// the identical token stream (only whitespace and keyword case may change).
func TestXMLOnlyChangesWhitespace(t *testing.T) {
	t.Parallel()
	h := &XMLHandler{}
	for _, f := range globInputs(t, "tcpou", "tcgvl", "tcdut", "tcio") {
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
			in := extractSTBlocks(data)
			out := extractSTBlocks(got)
			if len(in) != len(out) {
				t.Fatalf("ST block count changed: %d -> %d", len(in), len(out))
			}
			for i := range in {
				assertTokensPreserved(t, in[i], out[i])
			}
		})
	}
}

// extractSTBlocks returns the trimmed ST source inside every <Declaration>
// and <ST> CDATA block, mirroring the XMLHandler's selection logic.
func extractSTBlocks(src []byte) []string {
	dec := xml.NewDecoder(bytes.NewReader(src))
	var stack []string
	var blocks []string
	tokStart := int64(0)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		tokEnd := dec.InputOffset()
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if isSTBlock(stack) && isCDATA(src, tokStart, tokEnd) {
				if inner := strings.TrimSpace(string(t)); inner != "" {
					blocks = append(blocks, inner)
				}
			}
		}
		tokStart = tokEnd
	}
	return blocks
}

// TestXMLMalformed checks that malformed XML is reported as an error rather
// than being silently rewritten.
func TestXMLMalformed(t *testing.T) {
	t.Parallel()
	in := []byte("<Declaration></Declaration></unbalanced>")
	if _, err := (&XMLHandler{}).Format(in); err == nil {
		t.Error("expected error for malformed XML, got nil")
	}
}

// TestRegistryExtensionMapping verifies that each supported extension is
// routed to the correct handler.
func TestRegistryExtensionMapping(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	for _, ext := range []string{".tcpou", ".tcgvl", ".tcdut", ".tcio"} {
		h, err := r.ForExtension(ext)
		if err != nil {
			t.Errorf("ForExtension(%q): %v", ext, err)
			continue
		}
		if h.Name() != "xml" {
			t.Errorf("ForExtension(%q): expected xml handler, got %q", ext, h.Name())
		}
	}
	for _, ext := range []string{".st", ".iecst", ".tcst"} {
		h, err := r.ForExtension(ext)
		if err != nil {
			t.Errorf("ForExtension(%q): %v", ext, err)
			continue
		}
		if h.Name() != "st" {
			t.Errorf("ForExtension(%q): expected st handler, got %q", ext, h.Name())
		}
	}
}
