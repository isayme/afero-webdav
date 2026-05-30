package webdavfs

import (
	"io"
	"os"

	"github.com/spf13/afero"
)

// Compile-time check: *File satisfies afero.File.
var _ afero.File = (*File)(nil)

// File implements afero.File backed by WebDAV.
//
// A File is in one of three states: read-only, write-only, or a directory handle.
// The state is determined by how the file was opened (see Fs.OpenFile).
//
// Read state uses an HTTP GET stream with Range support for seeking.
// Write state uses an io.Pipe goroutine that uploads via PUT on Close.
// Directory state delegates listing to the WebDAV PROPFIND endpoint.
type File struct {
	// fs is the parent filesystem that created this file.
	fs   *Fs
	name string

	// reader is the HTTP response body from a GET request, lazily opened on first Read.
	reader io.ReadCloser
	// readOff tracks the virtual read position for seek operations.
	readOff int64

	// writer is the write side of an io.Pipe, feeding data to the streaming PUT goroutine.
	writer io.WriteCloser
	// writeResult receives the error (if any) from the PUT goroutine after Close.
	writeResult chan error

	// fileInfo caches the stat result so that Seek(SeekEnd) can compute the file size
	// without an additional PROPFIND round-trip.
	fileInfo os.FileInfo
	// closed prevents double-close and operations on a closed handle.
	closed bool
}

func NewFile(fs *Fs, name string) *File {
	return &File{
		fs:   fs,
		name: name,
	}
}

func (f *File) Name() string {
	return f.name
}

// Readdir reads the directory contents via WebDAV PROPFIND with depth 1.
//
// If count <= 0, all entries are returned.  If count > 0, at most count entries
// are returned (partial read), matching the afero.File interface contract.
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	infos, err := f.fs.client.ReadDir(f.name)
	if err != nil {
		return nil, err
	}

	if count > 0 {
		if len(infos) > count {
			infos = infos[:count]
		}
		if len(infos) == 0 {
			err = io.EOF
		}
	}

	return infos, err
}

func (f *File) Readdirnames(n int) ([]string, error) {
	infos, err := f.Readdir(n)
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name()
	}
	return names, err
}

// Stat returns file metadata.  The result is cached in fileInfo for Seek(SeekEnd).
func (f *File) Stat() (os.FileInfo, error) {
	if f.closed {
		return nil, afero.ErrFileClosed
	}

	info, err := f.fs.Stat(f.name)
	if err == nil {
		f.fileInfo = info
	}
	return info, err
}

// Sync is a no-op because each Write call is already streaming to the server.
func (f *File) Sync() error {
	return nil
}

// Truncate is not supported because WebDAV does not provide a truncation operation.
func (f *File) Truncate(_ int64) error {
	return ErrNotSupported
}

// Read reads up to len(p) bytes from the file.
//
// The underlying HTTP GET stream is lazily opened on the first Read call.
// Subsequent reads continue streaming from where the previous read left off.
// When the cached file size is known, ReadStreamRange is used so that only
// the needed bytes are transferred.
func (f *File) Read(p []byte) (int, error) {
	if f.closed {
		return 0, afero.ErrFileClosed
	}

	if f.reader == nil {
		if err := f.openReadStream(); err != nil {
			return 0, err
		}
	}

	n, err := f.reader.Read(p)
	f.readOff += int64(n)
	return n, err
}

// ReadAt reads len(p) bytes starting at byte offset off.
//
// Implemented as Seek + Read.  This is not zero-copy but keeps the implementation
// simple and correct.
func (f *File) ReadAt(p []byte, off int64) (int, error) {
	if f.closed {
		return 0, afero.ErrFileClosed
	}

	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}

	return f.Read(p)
}

// Seek sets the read offset for the next Read or ReadAt call.
//
// Write seeking is not supported because WebDAV PUT does not support partial
// updates (it always replaces the entire resource).  Only read seeking is
// implemented, using HTTP Range requests.
//
// When the current reader is at a different position, it is closed and a new
// stream will be opened lazily on the next Read.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, afero.ErrFileClosed
	}

	if f.writer != nil {
		return 0, ErrNotSupported
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.readOff + offset
	case io.SeekEnd:
		if f.fileInfo == nil {
			if _, err := f.Stat(); err != nil {
				return 0, err
			}
		}
		newOffset = f.fileInfo.Size() + offset
	default:
		return 0, ErrInvalidSeek
	}

	if newOffset < 0 {
		return 0, ErrInvalidSeek
	}

	if newOffset == f.readOff {
		return newOffset, nil
	}

	if f.reader != nil {
		f.reader.Close()
		f.reader = nil
	}

	f.readOff = newOffset
	return newOffset, nil
}

// Write writes data to the file via the streaming PUT pipe.
//
// Data is buffered in the io.Pipe and sent to the server in a background goroutine.
// The upload is finalised when Close() is called.
func (f *File) Write(p []byte) (int, error) {
	if f.closed {
		return 0, afero.ErrFileClosed
	}

	if f.writer == nil {
		return 0, ErrReadOnly
	}

	return f.writer.Write(p)
}

// WriteAt is not supported because WebDAV PUT always replaces the entire resource.
func (f *File) WriteAt(p []byte, _ int64) (int, error) {
	return 0, ErrNotSupported
}

// WriteString is a convenience wrapper around Write.
func (f *File) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Close finalises the file handle.
//
// For a read file: closes the underlying HTTP response body.
// For a write file: closes the PipeWriter, then waits for the background PUT
// goroutine to finish and reports any upload error.
// A closed directory handle is a no-op.
func (f *File) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	var err error

	if f.reader != nil {
		err = f.reader.Close()
		f.reader = nil
	}

	if f.writer != nil {
		if closeErr := f.writer.Close(); closeErr != nil {
			err = closeErr
		}
		if f.writeResult != nil {
			resultErr := <-f.writeResult
			if resultErr != nil {
				err = resultErr
			}
			close(f.writeResult)
			f.writeResult = nil
		}
		f.writer = nil
	}

	return err
}

// openReadStream lazily opens an HTTP GET stream at the current readOff.
//
// Three code paths:
//   1. Known file size (fileInfo.Size() > 0): use ReadStreamRange with the exact
//      remaining bytes.  This is the common case after a successful Stat.
//   2. Unknown size, offset == 0: use plain ReadStream (GET without Range header).
//   3. Unknown size, offset > 0: use ReadStreamRange with length=0, which sets
//      the Range header to "bytes=N-" (everything from N onward).  The server may
//      return 206 Partial Content or discard N bytes and return the rest.
func (f *File) openReadStream() error {
	var fileSize int64

	if f.fileInfo != nil {
		fileSize = f.fileInfo.Size()
	}

	if f.readOff >= fileSize {
		return io.EOF
	}

	var reader io.ReadCloser
	var err error

	if fileSize > 0 {
		reader, err = f.fs.client.ReadStreamRange(f.name, f.readOff, fileSize-f.readOff)
	} else {
		if f.readOff == 0 {
			reader, err = f.fs.client.ReadStream(f.name)
		} else {
			reader, err = f.fs.client.ReadStreamRange(f.name, f.readOff, 0)
		}
	}

	if err != nil {
		return err
	}

	f.reader = reader
	return nil
}

// openWriteStream sets up a streaming PUT upload via io.Pipe.
//
// A goroutine reads from the pipe and uploads to the WebDAV server using
// gowebdav.Client.WriteStream.  The error from the upload is communicated
// back through the writeResult channel and surfaced to the caller on Close().
//
// This is the same pattern used by afero-s3 for streaming uploads.
func (f *File) openWriteStream() error {
	pr, pw := io.Pipe()
	f.writer = pw
	f.writeResult = make(chan error, 1)

	go func() {
		err := f.fs.client.WriteStream(f.name, pr, defaultFileMode)
		f.writeResult <- err
	}()

	return nil
}
