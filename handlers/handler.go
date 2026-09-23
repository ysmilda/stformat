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
	// Name returns the handler's name.
	Name() string
	// Extensions returns the file extensions this handler supports.
	Extensions() []string
	// Format formats the given file contents and returns the formatted output.
	Format(contents []byte) ([]byte, error)
	// IsST returns whether the handler processes plain ST (not wrapped in XML etc.).
	IsST() bool
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

// Register adds a custom handler.
func (r *Registry) Register(h Handler) {
	r.handlers = append(r.handlers, h)
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
