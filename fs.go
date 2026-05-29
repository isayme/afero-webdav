// Package webdavfs provides an afero filesystem implementation backed by a WebDAV server.
//
// It wraps a gowebdav.Client and exposes a standard afero.Fs interface.  All WebDAV protocol
// details (PROPFIND, GET, PUT, MKCOL, DELETE, MOVE) are handled by the underlying client.
// File reads use HTTP Range requests for efficient seeking; writes use io.Pipe for streaming
// uploads.
package webdavfs

import (
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

// Compile-time check: *Fs satisfies afero.Fs.
var _ afero.Fs = (*Fs)(nil)

// Fs implements afero.Fs backed by a WebDAV server.
//
// All operations are delegated to a gowebdav.Client which handles the WebDAV protocol
// (PROPFIND, GET, PUT, MKCOL, DELETE, MOVE) and authentication.
type Fs struct {
	client *gowebdav.Client
}

// New creates a new WebDAV-backed filesystem from a pre-configured gowebdav.Client.
//
// The caller is responsible for setting up authentication, timeouts, and transport
// on the client before passing it here.
func New(client *gowebdav.Client) *Fs {
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

// Stat returns file or directory metadata via WebDAV PROPFIND.
//
// The root path "/" is handled locally because many WebDAV servers return
// inconsistent results for a PROPFIND on "/".
//
// The returned FileInfo includes WebDAV-specific metadata (ETag, ContentType)
// when available from the server response.
func (fs *Fs) Stat(name string) (os.FileInfo, error) {
	name = cleanPath(name)

	if name == "" || name == "/" {
		return NewFileInfo("/", true, 0, time.Time{}), nil
	}

	info, err := fs.client.Stat(name)
	if err != nil {
		return nil, &os.PathError{
			Op:   "stat",
			Path: name,
			Err:  err,
		}
	}

	return newFileInfo(info), nil
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

// newFileInfo wraps an os.FileInfo returned by gowebdav into our FileInfo type.
//
// It extracts the WebDAV-specific ETag and Content-Type from the underlying
// gowebdav.File struct when available.
func newFileInfo(info os.FileInfo) *FileInfo {
	fi := &FileInfo{
		name:    info.Name(),
		size:    info.Size(),
		modTime: info.ModTime(),
		isDir:   info.IsDir(),
	}

	if f, ok := info.(gowebdav.File); ok {
		fi.etag = f.ETag()
		fi.cType = f.ContentType()
	}

	return fi
}

// newFileInfoFromDavFile constructs a FileInfo directly from a gowebdav.File value
// with full metadata including ETag and Content-Type.
//
// This is used when processing ReadDir results where the underlying type is known.
func newFileInfoFromDavFile(f gowebdav.File) *FileInfo {
	return &FileInfo{
		name:    f.Name(),
		size:    f.Size(),
		modTime: f.ModTime(),
		isDir:   f.IsDir(),
		etag:    f.ETag(),
		cType:   f.ContentType(),
	}
}
