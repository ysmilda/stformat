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
// columns needs the whole line and the block structure around it; it fails as
// Format does, before anything is written to dst.
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
	formatted, err := Format(string(source))
	if err != nil {
		return 0, err
	}
	return io.Copy(dst, strings.NewReader(formatted))
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
		// A last line without a line break comes back together with the
		// error, so it is kept before the error is looked at: dropping it
		// would lose the whole input when it is a single unterminated line.
		line, err := br.ReadString('\n')
		buf.WriteString(line)
		if err != nil {
			// End of input, or a read error that surfaces when the caller
			// copies the rest of the stream. A directive on the partial line
			// needs no look here: the whole input is now in head, and Format
			// honours it.
			break scan
		}

		t := strings.TrimSpace(strings.TrimPrefix(line, utf8BOM))
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
