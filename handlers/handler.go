// Package handlers provides pluggable file type handlers that format
// different file types containing Structured Text code.
package handlers

import (
	"errors"
	"slices"
)

// ErrUnsupportedFileType is returned when no handler supports the given file.
var ErrUnsupportedFileType = errors.New("unsupported file type")

// Handler formats source code of a specific type.
type Handler interface {
	// Extensions returns the lowercase file extensions this handler supports.
	Extensions() []string
	// Format formats the given file contents and returns the formatted output.
	Format(contents []byte) ([]byte, error)
	// Blocks returns the Structured Text in the given file contents, in file
	// order.
	Blocks(contents []byte) ([]Block, error)
}

// Block is a stretch of Structured Text inside a file: the whole of a plain ST
// file, or one CDATA section of a TwinCAT XML file.
type Block struct {
	// Text is the Structured Text, exactly as it appears in the file.
	Text string
	// Offset is the byte offset of Text in the file.
	Offset int

	// start and end delimit the bytes Format replaces to rewrite the block:
	// for XML the whole <![CDATA[...]]> section.
	start, end int
}

// Registry holds all registered handlers.
type Registry struct {
	handlers []Handler
}

// NewRegistry creates a registry with default handlers.
func NewRegistry() *Registry {
	return &Registry{
		handlers: []Handler{
			&STHandler{},
			&XMLHandler{},
		},
	}
}

// ForExtension returns the handler for a file extension, or ErrUnsupportedFileType.
func (r *Registry) ForExtension(ext string) (Handler, error) {
	for _, h := range r.handlers {
		if slices.Contains(h.Extensions(), ext) {
			return h, nil
		}
	}
	return nil, ErrUnsupportedFileType
}

// Extensions returns all file extensions supported by registered handlers.
func (r *Registry) Extensions() []string {
	var exts []string
	for _, h := range r.handlers {
		exts = append(exts, h.Extensions()...)
	}
	return exts
}
