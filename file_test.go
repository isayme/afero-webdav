package webdavfs

import (
	"io"
	"testing"

	"github.com/spf13/afero"
)

var (
	_ afero.File = (*File)(nil)
)

func TestFsSeek(t *testing.T) {
	fs := newTestFs(t)

	content := "Hello, WebDAV World!"
	file, err := fs.Create("/seek.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	file.Write([]byte(content))
	file.Close()

	readFile, err := fs.Open("/seek.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer readFile.Close()

	pos, err := readFile.Seek(7, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	if pos != 7 {
		t.Errorf("Seek position = %d, want 7", pos)
	}

	buf := make([]byte, 5)
	n, err := readFile.Read(buf)
	if err != nil {
		t.Fatalf("Read after seek failed: %v", err)
	}
	if string(buf[:n]) != "WebDA" {
		t.Errorf("Read after seek = %q, want %q", string(buf[:n]), "WebDA")
	}

	pos, err = readFile.Seek(-6, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek end failed: %v", err)
	}

	n, err = readFile.Read(buf)
	if err != nil {
		t.Fatalf("Read after seek end failed: %v", err)
	}
	if string(buf[:n]) != "World" {
		t.Errorf("Read after seek end = %q, want %q", string(buf[:n]), "World")
	}
}

func TestFsSeekInvalid(t *testing.T) {
	fs := newTestFs(t)

	f, _ := fs.Create("/seek_invalid.txt")
	_, err := f.Seek(-1, io.SeekStart)
	if err != ErrNotSupported {
		t.Errorf("expected ErrNotSupported on write file seek, got %v", err)
	}
	f.Close()

	rf, _ := fs.Open("/seek_invalid.txt")
	_, err = rf.Seek(-1, io.SeekStart)
	if err != ErrInvalidSeek {
		t.Errorf("expected ErrInvalidSeek on read file, got %v", err)
	}
	rf.Close()
}

func TestFsDirReaddir(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/emptydir", defaultDirMode)

	dir, err := fs.Open("/emptydir")
	if err != nil {
		t.Fatalf("Open dir failed: %v", err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir empty dir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFileReaddirnames(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/alpha", defaultDirMode)
	fs.Mkdir("/beta", defaultDirMode)

	dir, err := fs.Open("/")
	if err != nil {
		t.Fatalf("Open root failed: %v", err)
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		t.Fatalf("Readdirnames failed: %v", err)
	}

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}

	if !found["alpha"] {
		t.Error("expected alpha in names")
	}
	if !found["beta"] {
		t.Error("expected beta in names")
	}
}

func TestFileCloseMultiple(t *testing.T) {
	fs := newTestFs(t)

	f, err := fs.Create("/closetest.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("Second Close should not error, got: %v", err)
	}
}

func TestFileReaddirCountPositiveEmptyDir(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/readdir_empty", defaultDirMode)

	dir, err := fs.Open("/readdir_empty")
	if err != nil {
		t.Fatalf("Open dir failed: %v", err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(1)
	if err != io.EOF {
		t.Errorf("Readdir(1) on empty dir: expected io.EOF, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFileReaddirTruncation(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/trunc_test", defaultDirMode)
	fs.Create("/trunc_test/a.txt")
	fs.Create("/trunc_test/b.txt")

	dir, err := fs.Open("/trunc_test")
	if err != nil {
		t.Fatalf("Open dir failed: %v", err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(1)
	if err != nil {
		t.Fatalf("Readdir(1) failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestFileReaddirnamesCountPositiveEmptyDir(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/readdirnames_empty", defaultDirMode)

	dir, err := fs.Open("/readdirnames_empty")
	if err != nil {
		t.Fatalf("Open dir failed: %v", err)
	}
	defer dir.Close()

	names, err := dir.Readdirnames(1)
	if err != io.EOF {
		t.Errorf("Readdirnames(1) on empty dir: expected io.EOF, got %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}
