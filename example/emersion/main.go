package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	emersion "github.com/emersion/go-webdav"

	webdavfs "github.com/isayme/afero-webdav"
)

// emersionAdapter wraps *emersion.Client to satisfy webdavfs.Client.
type emersionAdapter struct {
	c *emersion.Client
}

func (a *emersionAdapter) ReadDir(name string) ([]os.FileInfo, error) {
	infos, err := a.c.Readdir(name, false)
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, 0, len(infos))
	for _, fi := range infos {
		if strings.TrimSuffix(fi.Path, "/") == strings.TrimSuffix(name, "/") {
			continue
		}
		result = append(result, emersionFileInfo{fi})
	}
	return result, nil
}

func (a *emersionAdapter) Stat(name string) (os.FileInfo, error) {
	fi, err := a.c.Stat(name)
	if err != nil {
		return nil, err
	}
	return emersionFileInfo{*fi}, nil
}

func (a *emersionAdapter) Remove(name string) error {
	return a.c.RemoveAll(name)
}

func (a *emersionAdapter) RemoveAll(name string) error {
	return a.c.RemoveAll(name)
}

func (a *emersionAdapter) Mkdir(name string, _ os.FileMode) error {
	return a.c.Mkdir(name)
}

func (a *emersionAdapter) MkdirAll(name string, _ os.FileMode) error {
	parts := strings.Split(strings.Trim(name, "/"), "/")
	current := "/"
	for _, p := range parts {
		if p == "" {
			continue
		}
		current = path.Join(current, p)
		if err := a.c.Mkdir(current); err != nil {
			return err
		}
	}
	return nil
}

func (a *emersionAdapter) Rename(oldpath, newpath string, overwrite bool) error {
	return a.c.MoveAll(oldpath, newpath, overwrite)
}

func (a *emersionAdapter) ReadStream(name string) (io.ReadCloser, error) {
	return a.c.Open(name)
}

func (a *emersionAdapter) ReadStreamRange(name string, offset, length int64) (io.ReadCloser, error) {
	rc, err := a.c.Open(name)
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

func (a *emersionAdapter) Write(name string, data []byte, _ os.FileMode) error {
	wc, err := a.c.Create(name)
	if err != nil {
		return err
	}
	if _, err := wc.Write(data); err != nil {
		wc.Close()
		return err
	}
	return wc.Close()
}

func (a *emersionAdapter) WriteStream(name string, stream io.Reader, _ os.FileMode) error {
	wc, err := a.c.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(wc, stream); err != nil {
		wc.Close()
		return err
	}
	return wc.Close()
}

// emersionFileInfo adapts emersion.FileInfo to os.FileInfo.
type emersionFileInfo struct {
	emersion.FileInfo
}

func (fi emersionFileInfo) Name() string {
	return path.Base(fi.FileInfo.Path)
}

func (fi emersionFileInfo) Size() int64 {
	return fi.FileInfo.Size
}

func (fi emersionFileInfo) Mode() os.FileMode {
	if fi.FileInfo.IsDir {
		return os.ModeDir | 0755
	}
	return 0644
}

func (fi emersionFileInfo) ModTime() time.Time {
	return fi.FileInfo.ModTime
}

func (fi emersionFileInfo) IsDir() bool {
	return fi.FileInfo.IsDir
}

func (fi emersionFileInfo) Sys() interface{} {
	return nil
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

	var hc emersion.HTTPClient = http.DefaultClient
	if user != "" || pass != "" {
		hc = emersion.HTTPClientWithBasicAuth(hc, user, pass)
	}

	c, err := emersion.NewClient(hc, url)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	fs := webdavfs.New(&emersionAdapter{c: c})

	if err := runDemo(fs); err != nil {
		log.Fatalf("demo failed: %v", err)
	}
}

func runDemo(fs *webdavfs.Fs) error {
	fmt.Println("--- emersion/go-webdav demo ---")

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
