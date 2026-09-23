package handlers

import "github.com/ysmilda/stformat/formatter"

// STHandler formats plain Structured Text files.
type STHandler struct{}

func (h *STHandler) Name() string { return "st" }

func (h *STHandler) Extensions() []string { return []string{".st", ".iecst", ".tcst"} }

func (h *STHandler) Format(contents []byte) ([]byte, error) {
	formatted := formatter.Format(string(contents))
	return []byte(formatted), nil
}

func (h *STHandler) IsST() bool { return true }
