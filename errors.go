package webdavfs

import (
	"errors"
	"fmt"
)

// Sentinel errors for well-known failure modes of the WebDAV filesystem.
var (
	// ErrNotSupported is returned when an operation cannot be performed over
	// the WebDAV protocol (e.g. Chmod, random-access writes).
	ErrNotSupported = errors.New("operation not supported")
	// ErrInvalidSeek is returned when a seek position would go negative.
	ErrInvalidSeek = errors.New("invalid seek offset")
	// ErrReadOnly is returned when Write is called on a file opened read-only.
	ErrReadOnly = errors.New("file opened read-only")
	// ErrWriteOnly is returned when Read is called on a file opened write-only.
	ErrWriteOnly = errors.New("file opened write-only")
	// ErrFileClosed is returned when an operation is attempted on a closed file.
	ErrFileClosed = errors.New("file is closed")
	// ErrIsDirectory is returned when a file operation is attempted on a directory.
	ErrIsDirectory = errors.New("is a directory")
	// ErrNotImplemented is a placeholder for operations that may be added later.
	ErrNotImplemented = errors.New("not implemented")
)

// PathError records an error and the operation and path that caused it,
// analogous to os.PathError.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *PathError) Unwrap() error {
	return e.Err
}
