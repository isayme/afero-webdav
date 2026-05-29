package webdavfs

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

var (
	_ afero.Fs   = (*Fs)(nil)
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{".", "/"},
		{"/foo", "/foo"},
		{"foo", "foo"},
		{"/foo/bar", "/foo/bar"},
		{"/foo/../bar", "/bar"},
		{"/foo/./bar", "/foo/bar"},
		{"/foo//bar", "/foo/bar"},
		{"\\foo", "/foo"},
		{"\\foo\\bar", "/foo/bar"},
		{"/foo/bar/", "/foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanPath(tt.input)
			if result != tt.expected {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFsName(t *testing.T) {
	fs := New(gowebdav.NewClient("http://localhost:0", "", ""))
	if fs.Name() != "WebdavFs" {
		t.Errorf("Name() = %q, want %q", fs.Name(), "WebdavFs")
	}
}

func TestFsStatRoot(t *testing.T) {
	fs := newTestFs(t)

	info, err := fs.Stat("/")
	if err != nil {
		t.Fatalf("Stat root failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("root should be a directory")
	}
}

func TestFsStatNotExist(t *testing.T) {
	fs := newTestFs(t)

	_, err := fs.Stat("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestFsUnsupportedOps(t *testing.T) {
	fs := New(gowebdav.NewClient("http://localhost:0", "", ""))

	if err := fs.Chmod("/test", defaultFileMode); err != ErrNotSupported {
		t.Errorf("Chmod should return ErrNotSupported, got %v", err)
	}
	if err := fs.Chtimes("/test", time.Now(), time.Now()); err != ErrNotSupported {
		t.Errorf("Chtimes should return ErrNotSupported, got %v", err)
	}
	if err := fs.Chown("/test", 0, 0); err != ErrNotSupported {
		t.Errorf("Chown should return ErrNotSupported, got %v", err)
	}
}

func TestOpenFileFlags(t *testing.T) {
	fs := New(gowebdav.NewClient("http://localhost:0", "", ""))

	_, err := fs.OpenFile("/test", os.O_RDWR, defaultFileMode)
	if err != ErrNotSupported {
		t.Errorf("O_RDWR should return ErrNotSupported, got %v", err)
	}
	_, err = fs.OpenFile("/test", os.O_APPEND, defaultFileMode)
	if err != ErrNotSupported {
		t.Errorf("O_APPEND should return ErrNotSupported, got %v", err)
	}
}

func TestFsCreateAndRead(t *testing.T) {
	fs := newTestFs(t)

	content := "Hello, WebDAV!"

	file, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	readFile, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer readFile.Close()

	data, err := io.ReadAll(readFile)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(data) != content {
		t.Errorf("Read content = %q, want %q", string(data), content)
	}
}

func TestFsMkdirAndStat(t *testing.T) {
	fs := newTestFs(t)

	if err := fs.Mkdir("/mydir", defaultDirMode); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	info, err := fs.Stat("/mydir")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
	if info.Name() != "mydir" {
		t.Errorf("Name() = %q, want %q", info.Name(), "mydir")
	}
}

func TestFsRemove(t *testing.T) {
	fs := newTestFs(t)

	file, err := fs.Create("/todelete.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	file.Close()

	if err := fs.Remove("/todelete.txt"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = fs.Stat("/todelete.txt")
	if err == nil {
		t.Error("expected error after removal")
	}
}

func TestFsRename(t *testing.T) {
	fs := newTestFs(t)

	file, err := fs.Create("/old.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	file.Write([]byte("hello"))
	file.Close()

	if err := fs.Rename("/old.txt", "/new.txt"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	_, err = fs.Stat("/new.txt")
	if err != nil {
		t.Errorf("Stat new file failed: %v", err)
	}
	_, err = fs.Stat("/old.txt")
	if err == nil {
		t.Error("old file should not exist after rename")
	}
}

func TestFsCreateMkdirAll(t *testing.T) {
	fs := newTestFs(t)

	if err := fs.MkdirAll("/a/b/c", defaultDirMode); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	info, err := fs.Stat("/a/b/c")
	if err != nil {
		t.Fatalf("Stat after MkdirAll failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestFsRemoveAll(t *testing.T) {
	fs := newTestFs(t)

	fs.MkdirAll("/a/b", defaultDirMode)
	file, _ := fs.Create("/a/b/file.txt")
	file.Close()

	if err := fs.RemoveAll("/a"); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	_, err := fs.Stat("/a")
	if err == nil {
		t.Error("expected error after RemoveAll")
	}
}

func TestFsReaddir(t *testing.T) {
	fs := newTestFs(t)

	fs.Mkdir("/dir1", defaultDirMode)
	fs.Mkdir("/dir2", defaultDirMode)

	file, _ := fs.Create("/file1.txt")
	file.Close()

	dir, err := fs.Open("/")
	if err != nil {
		t.Fatalf("Open root failed: %v", err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir failed: %v", err)
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	if !names["dir1"] {
		t.Error("expected dir1 in listing")
	}
	if !names["dir2"] {
		t.Error("expected dir2 in listing")
	}
	if !names["file1.txt"] {
		t.Error("expected file1.txt in listing")
	}
}

func TestFsNonExistentDir(t *testing.T) {
	fs := newTestFs(t)

	dir, err := fs.Open("/nonexistent_dir")
	if err == nil {
		dir.Close()
		t.Error("expected error opening nonexistent directory")
	}
}
