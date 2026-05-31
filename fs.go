// Package webdavfs provides an afero filesystem implementation backed by a WebDAV server.
//
// It uses a Client interface and exposes a standard afero.Fs interface.  All WebDAV protocol
// details (PROPFIND, GET, PUT, MKCOL, DELETE, MOVE) are handled by the underlying client.
// File reads use HTTP Range requests for efficient seeking; writes use io.Pipe for streaming
// uploads.
package webdavfs

import (
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// Client is the interface that wraps the WebDAV client methods used by Fs.
//
// Any WebDAV client implementation that satisfies this interface can be used
// as the backend, making the package independent of a specific client library.
// The gowebdav.Client from github.com/studio-b12/gowebdav satisfies this
// interface by default.
type Client interface {
	ReadDir(path string) ([]os.FileInfo, error)
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	RemoveAll(path string) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string, overwrite bool) error
	ReadStream(path string) (io.ReadCloser, error)
	ReadStreamRange(path string, offset, length int64) (io.ReadCloser, error)
	Write(path string, data []byte, mode os.FileMode) error
	WriteStream(path string, stream io.Reader, mode os.FileMode) error
}

// Default permission bits for files and directories created through this
// filesystem.  WebDAV does not expose Unix permission metadata, so these
// values are used for all entries.
const (
	defaultFileMode os.FileMode = 0644 // owner rw, group r, other r
	defaultDirMode  os.FileMode = 0755 // owner rwx, group rx, other rx
)

// Compile-time check: *Fs satisfies afero.Fs.
var _ afero.Fs = (*Fs)(nil)

// Fs implements afero.Fs backed by a WebDAV server.
//
// All operations are delegated to a Client which handles the WebDAV protocol
// (PROPFIND, GET, PUT, MKCOL, DELETE, MOVE) and authentication.
type Fs struct {
	client Client
}

// New creates a new WebDAV-backed filesystem from a pre-configured Client.
//
// The caller is responsible for setting up authentication, timeouts, and transport
// on the client before passing it here.
func New(client Client) *Fs {
	return &Fs{
		client: client,
	}
}

func (fs *Fs) Name() string {
	return "WebdavFs"
}

// Create creates a new file and returns a writable handle.
//
// An empty object is written first to ensure the file exists on the server,
// then the file is re-opened for streaming write.  This two-step approach
// mirrors afero-s3 (PutObject then OpenFile) and works around the lack of
// an atomic "create-or-truncate" in the WebDAV PUT semantics.
func (fs *Fs) Create(name string) (afero.File, error) {
	name = cleanPath(name)

	if err := fs.client.Write(name, []byte{}, defaultFileMode); err != nil {
		return nil, err
	}

	return fs.OpenFile(name, os.O_WRONLY, defaultFileMode)
}

func (fs *Fs) Mkdir(name string, perm os.FileMode) error {
	name = cleanPath(name)
	return fs.client.Mkdir(name, perm)
}

func (fs *Fs) MkdirAll(name string, perm os.FileMode) error {
	name = cleanPath(name)
	return fs.client.MkdirAll(name, perm)
}

func (fs *Fs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file for reading or writing.
//
// Supported flags: O_RDONLY, O_WRONLY, O_CREATE (implied by O_WRONLY).
//
// O_RDWR, O_APPEND, and O_EXCL are not supported because WebDAV operates over
// HTTP which does not provide the necessary atomic read-write or append guarantees.
//
// For reads: the file is stat'd first to cache its size (needed for Range-based
// seeking).  The actual HTTP GET request is deferred to the first Read() call.
//
// For writes: a streaming upload via io.Pipe is started immediately.  The caller
// must Close() the file to finalise the upload.
func (fs *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name = cleanPath(name)

	if flag&os.O_RDWR != 0 {
		return nil, ErrNotSupported
	}

	if flag&os.O_APPEND != 0 {
		return nil, ErrNotSupported
	}

	if flag&os.O_EXCL != 0 {
		return nil, ErrNotSupported
	}

	file := &File{
		fs:   fs,
		name: name,
	}

	if flag&os.O_WRONLY != 0 {
		if err := file.openWriteStream(); err != nil {
			return nil, err
		}
		return file, nil
	}

	info, err := fs.Stat(name)
	if err != nil {
		return nil, err
	}

	file.fileInfo = info

	if info.IsDir() {
		return file, nil
	}

	return file, nil
}

// Remove deletes a single file or empty directory via WebDAV DELETE.
func (fs *Fs) Remove(name string) error {
	name = cleanPath(name)
	return fs.client.Remove(name)
}

// RemoveAll recursively deletes a path and all its children.
//
// gowebdav.Client.RemoveAll handles the recursive listing and deletion.
func (fs *Fs) RemoveAll(name string) error {
	name = cleanPath(name)
	return fs.client.RemoveAll(name)
}

// Rename moves a file or directory via WebDAV MOVE.
//
// Identity rename (oldname == newname) is a no-op.
// Overwrite is disabled (gowebdav Rename with overwrite=false).
func (fs *Fs) Rename(oldname, newname string) error {
	oldname = cleanPath(oldname)
	newname = cleanPath(newname)

	if oldname == newname {
		return nil
	}

	return fs.client.Rename(oldname, newname, false)
}

// fileInfo wraps an os.FileInfo to override Mode() with fixed permission bits,
// since WebDAV does not expose Unix permission metadata.
type fileInfo struct {
	os.FileInfo
}

func (fi fileInfo) Mode() os.FileMode {
	if fi.IsDir() {
		return os.ModeDir | defaultDirMode
	}
	return defaultFileMode
}

// wrapFileInfo wraps an os.FileInfo into fileInfo, overriding Mode() with
// fixed permission bits. Already-wrapped values and nil are returned as-is.
func wrapFileInfo(fi os.FileInfo) os.FileInfo {
	if fi == nil {
		return nil
	}
	if _, ok := fi.(fileInfo); ok {
		return fi
	}
	return fileInfo{FileInfo: fi}
}

// rootStat implements os.FileInfo for the filesystem root "/".
//
// Many WebDAV servers return inconsistent results for a PROPFIND on "/",
// so the root path is handled locally instead of making a server round-trip.
type rootStat struct{}

func (rootStat) Name() string       { return "/" }
func (rootStat) Size() int64        { return 0 }
func (rootStat) Mode() os.FileMode  { return os.ModeDir | defaultDirMode }
func (rootStat) ModTime() time.Time { return time.Time{} }
func (rootStat) IsDir() bool        { return true }
func (rootStat) Sys() any           { return nil }

// Stat returns file or directory metadata via WebDAV PROPFIND.
//
// The root path "/" is handled locally because many WebDAV servers return
// inconsistent results for a PROPFIND on "/".
func (fs *Fs) Stat(name string) (os.FileInfo, error) {
	name = cleanPath(name)

	if name == "" || name == "/" {
		return rootStat{}, nil
	}

	info, err := fs.client.Stat(name)
	if err != nil {
		return nil, &os.PathError{
			Op:   "stat",
			Path: name,
			Err:  err,
		}
	}

	return wrapFileInfo(info), nil
}

// Chmod is not supported because standard WebDAV does not expose Unix permission bits.
func (fs *Fs) Chmod(_ string, _ os.FileMode) error {
	return ErrNotSupported
}

// Chtimes is not supported because WebDAV does not provide a standard
// method for setting modification times independently of upload.
func (fs *Fs) Chtimes(_ string, _, _ time.Time) error {
	return ErrNotSupported
}

// Chown is not supported because WebDAV does not expose ownership metadata.
func (fs *Fs) Chown(_ string, _, _ int) error {
	return ErrNotSupported
}

// cleanPath normalizes a path for use with the WebDAV server.
//
// It converts backslashes to forward slashes, collapses redundant separators
// and parent-directory references via path.Clean, and maps empty/root paths
// consistently to "/".
func cleanPath(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean(name)
	if name == "." || name == "/" {
		return "/"
	}
	return name
}
