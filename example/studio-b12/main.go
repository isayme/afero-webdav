package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/studio-b12/gowebdav"

	webdavfs "github.com/isayme/afero-webdav"
)

func main() {
	url := os.Getenv("WEBDAV_URL")
	if url == "" {
		log.Fatal("WEBDAV_URL is required")
	}
	user := os.Getenv("WEBDAV_USER")
	pass := os.Getenv("WEBDAV_PASS")

	c := gowebdav.NewClient(url, user, pass)
	c.SetTimeout(10 * 1_000_000_000)

	fs := webdavfs.New(c)

	if err := runDemo(fs); err != nil {
		log.Fatalf("demo failed: %v", err)
	}
}

func runDemo(fs *webdavfs.Fs) error {
	fmt.Println("--- studio-b12/gowebdav demo ---")

	// stat root
	info, err := fs.Stat("/")
	if err != nil {
		return fmt.Errorf("stat root: %w", err)
	}
	fmt.Printf("root: IsDir=%v\n", info.IsDir())

	// create directory
	if err := fs.MkdirAll("/afero-demo", 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	fmt.Println("created /afero-demo")

	// write a file
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

	// read it back
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

	// list directory
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

	// cleanup
	if err := fs.RemoveAll("/afero-demo"); err != nil {
		return fmt.Errorf("removeall: %w", err)
	}
	fmt.Println("removed /afero-demo")

	return nil
}
