package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ysmilda/stformat/formatter"
)

// XMLHandler formats TwinCAT XML files that embed Structured Text
// inside <![CDATA[...]]> sections of <Declaration> and <ST> elements.
// Only the CDATA content is touched; all other XML bytes round-trip
// verbatim.
type XMLHandler struct{}

func (h *XMLHandler) Name() string { return "xml" }

func (h *XMLHandler) Extensions() []string {
	return []string{".tcpou", ".tcgvl", ".tcdut", ".tcio", ".tcproj", ".xml"}
}

func (h *XMLHandler) IsST() bool { return false }

type xmlEdit struct {
	start, end int
	repl       []byte
}

// Format parses the XML, formats ST inside <Declaration> and <ST> CDATA
// sections, and returns the result with all untouched bytes unchanged.
func (h *XMLHandler) Format(contents []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(contents))

	var stack []string
	var edits []xmlEdit

	tokStart := int64(0)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml parse error: %w", err)
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
			if isSTBlock(stack) && isCDATA(contents, tokStart, tokEnd) {
				raw := string(t)
				inner := strings.TrimSpace(raw)
				if len(inner) > 0 && looksLikeST(inner) {
					formatted := formatter.Format(inner)
					// Declaration/implementation content starts and ends
					// with a line break so git diffs are per-line readable.
					wrapped := "\n" + strings.TrimRight(formatted, "\n") + "\n"
					if wrapped != raw {
						edits = append(edits, xmlEdit{
							start: int(tokStart),
							end:   int(tokEnd),
							repl:  []byte("<![CDATA[" + wrapped + "]]>"),
						})
					}
				}
			}
		}
		tokStart = tokEnd
	}

	return applyEdits(contents, edits), nil
}

// isSTBlock reports whether the top of the element stack is a standard
// TwinCAT ST container.
func isSTBlock(stack []string) bool {
	if len(stack) == 0 {
		return false
	}
	top := stack[len(stack)-1]
	return top == "Declaration" || top == "ST"
}

// isCDATA reports whether the token byte range in src starts a CDATA section.
func isCDATA(src []byte, from, to int64) bool {
	if from < 0 || from >= to || int(to) > len(src) {
		return false
	}
	return bytes.HasPrefix(src[from:to], []byte("<![CDATA["))
}

// looksLikeST is a light guard to avoid reformatting text nodes that are
// not Structured Text.
func looksLikeST(s string) bool {
	return strings.ContainsAny(s, ":=;()[]") || len(strings.Fields(s)) > 1
}

// applyEdits applies edits in ascending offset order, copying untouched
// spans between each replacement.
func applyEdits(src []byte, edits []xmlEdit) []byte {
	if len(edits) == 0 {
		return src
	}
	var out bytes.Buffer
	cursor := 0
	for _, e := range edits {
		// Edits are naturally in ascending order (token scan); apply in
		// place by copying untouched spans between them.
		if e.start < cursor {
			continue
		}
		out.Write(src[cursor:e.start])
		out.Write(e.repl)
		cursor = e.end
	}
	out.Write(src[cursor:])
	return out.Bytes()
}
