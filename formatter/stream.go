package formatter

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// Copy formats src into dst and reports how many bytes were written.
//
// It honours the formatting directives, so a caller can pipe a file or a stream
// through the formatter without caring about its content. A file-level
// "stformat:ignore" in the leading comment block is streamed straight through:
// the input is never parsed, so an ignored stream may be of any size. Any other
// input is read in full and formatted, because wrapping a line at 120
// characters needs the whole line and the block structure around it.
func Copy(dst io.Writer, src io.Reader) (int64, error) {
	br := bufio.NewReader(src)
	head, ignored := readLeadingComments(br)
	rest := io.MultiReader(bytes.NewReader(head), br)
	if ignored {
		return io.Copy(dst, rest)
	}
	source, err := io.ReadAll(rest)
	if err != nil {
		return 0, err
	}
	return io.Copy(dst, strings.NewReader(Format(string(source))))
}

// readLeadingComments consumes the comment block at the top of br. It returns
// those bytes together with whether they carried a file-level
// "stformat:ignore". The consumed bytes are handed back so that the caller can
// treat the input as one uninterrupted stream.
func readLeadingComments(br *bufio.Reader) (head []byte, ignored bool) {
	var buf bytes.Buffer
	inBlockComment := false
scan:
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			// End of input, or a read error that surfaces when the caller
			// copies the rest of the stream.
			break scan
		}
		buf.WriteString(line)

		t := strings.TrimSpace(line)
		switch {
		case inBlockComment:
			inBlockComment = !strings.HasSuffix(t, "*)")
		case t == "":
			// Blank lines may sit between the directive and the code below.
		case strings.HasPrefix(t, "//"):
			ignored = ignored || lineDirective(line) == directiveIgnore
		case strings.HasPrefix(t, "(*"):
			inBlockComment = !strings.HasSuffix(t, "*)")
			ignored = ignored || lineDirective(line) == directiveIgnore
		default:
			break scan
		}
	}
	return buf.Bytes(), ignored
}
