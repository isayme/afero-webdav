package webdavfs

import (
	"os"
	"time"
)

// Default permission bits for files and directories created through this
// filesystem.  WebDAV does not expose Unix permission metadata, so these
// values are used for all entries.
const (
	defaultFileMode os.FileMode = 0644 // owner rw, group r, other r
	defaultDirMode  os.FileMode = 0755 // owner rwx, group rx, other rx
)

// Compile-time check: *FileInfo satisfies os.FileInfo.
var _ os.FileInfo = (*FileInfo)(nil)

// FileInfo implements os.FileInfo with WebDAV-specific extensions.
//
// In addition to the standard os.FileInfo methods, FileInfo exposes
// ETag() and ContentType() which are extracted from the WebDAV PROPFIND response.
// Mode() returns fixed values because WebDAV does not expose Unix permission bits.
type FileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
	etag    string
	cType   string
}

func NewFileInfo(name string, isDir bool, size int64, modTime time.Time) *FileInfo {
	return &FileInfo{
		name:    name,
		isDir:   isDir,
		size:    size,
		modTime: modTime,
	}
}

func (fi *FileInfo) Name() string {
	return fi.name
}

func (fi *FileInfo) Size() int64 {
	return fi.size
}

// Mode returns fixed permission bits because WebDAV does not expose Unix permissions.
func (fi *FileInfo) Mode() os.FileMode {
	if fi.isDir {
		return os.ModeDir | defaultDirMode
	}
	return defaultFileMode
}

func (fi *FileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *FileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *FileInfo) Sys() interface{} {
	return nil
}

func (fi *FileInfo) ETag() string {
	return fi.etag
}

func (fi *FileInfo) ContentType() string {
	return fi.cType
}
