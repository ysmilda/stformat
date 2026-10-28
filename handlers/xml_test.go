package handlers

import (
	"bytes"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// TestXMLFiles formats every testdata/*.in.{tcpou,tcgvl,tcdut,tcio} file and
// compares the result against its .golden. counterpart.
//
// runGolden also checks idempotency, and that the ST inside the CDATA sections
// kept every token, so this is the whole check for a TwinCAT fixture. The plain
// ST half is TestSTFiles.
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

// extractSTBlocks returns the trimmed ST source inside every <Declaration>
// and <ST> CDATA block, mirroring the XMLHandler's selection logic. Each entry
// carries the byte range of the CDATA section it came from, so that a caller can
// put a modified block back where it found it.
func extractSTBlocks(src []byte) []string {
	out := make([]string, 0, 8)
	for _, b := range stBlockRanges(src) {
		out = append(out, strings.TrimSpace(string(src[b.from:b.to])))
	}
	return out
}

// stBlock is one ST-carrying CDATA section and where it sits in the file.
type stBlock struct {
	from, to int // the body of the CDATA section, without the delimiters
}

// stBlockRanges returns the CDATA sections of a TwinCAT XML file that hold ST,
// in the order they appear.
func stBlockRanges(src []byte) []stBlock {
	out := make([]stBlock, 0, 8)
	dec := xml.NewDecoder(bytes.NewReader(src))
	var stack []string
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
					out = append(out, stBlock{from: int(tokStart) + len("<![CDATA["), to: int(tokEnd) - len("]]>")})
				}
			}
		}
		tokStart = tokEnd
	}
	return out
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

// TestXMLLineEnding checks that formatting an XML file keeps the line ending
// it uses, including the line breaks inserted around a CDATA section. The XML
// outside the sections is never touched.
func TestXMLLineEnding(t *testing.T) {
	t.Parallel()
	lf := "<?xml version=\"1.0\"?>\n<TcPlcObject>\n  <POU Name=\"P\" Id=\"{1}\">\n" +
		"    <Declaration><![CDATA[TYPE t : STRUCT a : INT; END_STRUCT END_TYPE]]></Declaration>\n" +
		"  </POU>\n</TcPlcObject>\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	for _, in := range []string{lf, crlf} {
		out, err := (&XMLHandler{}).Format([]byte(in))
		if err != nil {
			t.Fatalf("Format: %v", err)
		}
		got := string(out)
		crlfIn := strings.Contains(in, "\r\n")
		if crlfIn != strings.Contains(got, "\r\n") {
			t.Errorf("line ending changed for CRLF=%v input: %q", crlfIn, got)
		}
		// No mixing: an LF file must gain no carriage return at all.
		if !crlfIn && strings.Contains(got, "\r") {
			t.Errorf("unexpected carriage returns: %q", got)
		}
		// And a CRLF file must not keep a bare LF.
		if crlfIn && strings.Count(got, "\n") != strings.Count(got, "\r\n") {
			t.Errorf("mixed line endings: %q", got)
		}
		if again, err := (&XMLHandler{}).Format(out); err != nil || string(again) != got {
			t.Errorf("not idempotent: %q", string(again))
		}
	}
}

// TestLooksLikeST checks that prose in a CDATA section is not mistaken for ST.
// It used to be, because the guard accepted any text with two or more words,
// and keywords in the prose were then uppercased.
func TestLooksLikeST(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"not-CDATA":                             false,
		"some prose, not code":                  false,
		"Returns: the number of items written.": false,
		"":                                      false,
		"x := 1;":                               true,
		"FUNCTION f : BOOL":                     true,
		"TYPE S : STRUCT a : INT; END_STRUCT":   true,
		"(key := 'aaa', description := 'slot')": true,
		"arr : ARRAY[0..3] OF INT;":             true,
		"s : STRING(80) := 'hi';":               true,
	}
	for in, want := range cases {
		if got := looksLikeST(in); got != want {
			t.Errorf("looksLikeST(%q) = %v, want %v", in, got, want)
		}
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
		if _, ok := h.(*XMLHandler); !ok {
			t.Errorf("ForExtension(%q): expected xml handler, got %T", ext, h)
		}
	}
	for _, ext := range []string{".st", ".iecst", ".tcst"} {
		h, err := r.ForExtension(ext)
		if err != nil {
			t.Errorf("ForExtension(%q): %v", ext, err)
			continue
		}
		if _, ok := h.(*STHandler); !ok {
			t.Errorf("ForExtension(%q): expected st handler, got %T", ext, h)
		}
	}
}

// TestRegistryIgnoresOtherXML guards against formatting arbitrary XML: a
// directory walk must not pick up .xml or .tcproj files, whose <ST> or
// <Declaration> elements need not hold Structured Text.
func TestRegistryIgnoresOtherXML(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	for _, ext := range []string{".xml", ".tcproj", ".plcproj"} {
		if _, err := r.ForExtension(ext); err == nil {
			t.Errorf("ForExtension(%q) found a handler, want ErrUnsupportedFileType", ext)
		}
	}
}
