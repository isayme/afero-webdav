package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rickb777/gowebdav"
	"github.com/rickb777/gowebdav/auth"

	webdavfs "github.com/isayme/afero-webdav"
)

// rickb777Adapter wraps rickb777/gowebdav.Client to satisfy webdavfs.Client.
type rickb777Adapter struct {
	c gowebdav.Client
}

func (a *rickb777Adapter) ReadDir(path string) ([]os.FileInfo, error) {
	return a.c.ReadDir(path)
}

func (a *rickb777Adapter) Stat(path string) (os.FileInfo, error) {
	return a.c.Stat(path)
}

func (a *rickb777Adapter) Remove(path string) error {
	return a.c.Remove(path)
}

func (a *rickb777Adapter) RemoveAll(path string) error {
	return a.c.RemoveAll(path)
}

func (a *rickb777Adapter) Mkdir(path string, perm os.FileMode) error {
	return a.c.Mkdir(path, perm)
}

func (a *rickb777Adapter) MkdirAll(path string, perm os.FileMode) error {
	return a.c.MkdirAll(path, perm)
}

func (a *rickb777Adapter) Rename(oldpath, newpath string, overwrite bool) error {
	if overwrite {
		return a.c.Rename(oldpath, newpath)
	}
	return a.c.RenameWithoutOverwriting(oldpath, newpath)
}

func (a *rickb777Adapter) ReadStream(path string) (io.ReadCloser, error) {
	return a.c.ReadStream(path)
}

func (a *rickb777Adapter) ReadStreamRange(path string, offset, length int64) (io.ReadCloser, error) {
	rc, err := a.c.ReadStream(path)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := io.CopyN(io.Discard, rc, offset); err != nil {
			rc.Close()
			return nil, err
		}
	}
	if length > 0 {
		return readCloser{io.LimitReader(rc, length), rc}, nil
	}
	return rc, nil
}

func (a *rickb777Adapter) Write(path string, data []byte, _ os.FileMode) error {
	return a.c.WriteFile(path, data, "application/octet-stream")
}

func (a *rickb777Adapter) WriteStream(path string, stream io.Reader, _ os.FileMode) error {
	return a.c.WriteStream(path, stream, "application/octet-stream")
}

// readCloser bundles an io.Reader and io.Closer into one.
type readCloser struct {
	io.Reader
	io.Closer
}

func main() {
	url := os.Getenv("WEBDAV_URL")
	if url == "" {
		log.Fatal("WEBDAV_URL is required")
	}
	user := os.Getenv("WEBDAV_USER")
	pass := os.Getenv("WEBDAV_PASS")

	var opts []gowebdav.ClientOpt
	if user != "" || pass != "" {
		opts = append(opts, gowebdav.SetAuthentication(auth.Basic(user, pass)))
	}
	opts = append(opts, gowebdav.SetHttpClient(&http.Client{Timeout: 10 * time.Second}))

	c := gowebdav.NewClient(url, opts...)

	fs := webdavfs.New(&rickb777Adapter{c: c})

	if err := runDemo(fs); err != nil {
		log.Fatalf("demo failed: %v", err)
	}
}

func runDemo(fs *webdavfs.Fs) error {
	fmt.Println("--- rickb777/gowebdav demo ---")

	info, err := fs.Stat("/")
	if err != nil {
		return fmt.Errorf("stat root: %w", err)
	}
	fmt.Printf("root: IsDir=%v\n", info.IsDir())

	if err := fs.MkdirAll("/afero-demo", 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	fmt.Println("created /afero-demo")

	f, err := fs.Create("/afero-demo/hello.txt")
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if _, err := f.Write([]byte("Hello, WebDAV!")); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close write: %w", err)
	}
	fmt.Println("wrote /afero-demo/hello.txt")

	rf, err := fs.Open("/afero-demo/hello.txt")
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer rf.Close()

	data, err := io.ReadAll(rf)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	fmt.Printf("read: %q\n", string(data))

	dir, err := fs.Open("/afero-demo")
	if err != nil {
		return fmt.Errorf("open dir: %w", err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		return fmt.Errorf("readdir: %w", err)
	}
	for _, e := range entries {
		fmt.Printf("  %s (size=%d, dir=%v)\n", e.Name(), e.Size(), e.IsDir())
	}

	if err := fs.RemoveAll("/afero-demo"); err != nil {
		return fmt.Errorf("removeall: %w", err)
	}
	fmt.Println("removed /afero-demo")

	return nil
}
