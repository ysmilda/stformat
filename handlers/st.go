package handlers

import "github.com/ysmilda/stformat/formatter"

// STHandler formats plain Structured Text files.
type STHandler struct{}

// Extensions returns the plain ST file extensions.
func (h *STHandler) Extensions() []string { return []string{".st", ".iecst", ".tcst"} }

// Format formats a plain ST file. It fails, as formatter.Format does, on a file
// that is not well formed and rather than return output that changes the code.
func (h *STHandler) Format(contents []byte) ([]byte, error) {
	formatted, err := formatter.Format(string(contents))
	if err != nil {
		return nil, err
	}
	return []byte(formatted), nil
}
