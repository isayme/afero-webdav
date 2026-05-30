package webdavfs

import (
	"os"
	"testing"
	"time"
)

// testFileInfo implements os.FileInfo for testing purposes.
type testFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (fi testFileInfo) Name() string       { return fi.name }
func (fi testFileInfo) Size() int64        { return fi.size }
func (fi testFileInfo) Mode() os.FileMode  { return 0 }
func (fi testFileInfo) ModTime() time.Time { return fi.modTime }
func (fi testFileInfo) IsDir() bool        { return fi.isDir }
func (fi testFileInfo) Sys() interface{}   { return nil }

func TestFileInfoFileMode(t *testing.T) {
	now := time.Now()
	inner := testFileInfo{name: "test.txt", size: 100, modTime: now, isDir: false}
	wrapped := wrapFileInfo(inner)

	if wrapped.Name() != "test.txt" {
		t.Errorf("Name() = %q, want %q", wrapped.Name(), "test.txt")
	}
	if wrapped.Size() != 100 {
		t.Errorf("Size() = %d, want %d", wrapped.Size(), 100)
	}
	if wrapped.Mode() != defaultFileMode {
		t.Errorf("Mode() = %v, want %v", wrapped.Mode(), defaultFileMode)
	}
	if !wrapped.ModTime().Equal(now) {
		t.Errorf("ModTime() mismatch")
	}
	if wrapped.IsDir() {
		t.Error("IsDir() should be false")
	}
}

func TestFileInfoDirMode(t *testing.T) {
	now := time.Now()
	inner := testFileInfo{name: "mydir", size: 0, modTime: now, isDir: true}
	wrapped := wrapFileInfo(inner)

	if wrapped.Name() != "mydir" {
		t.Errorf("Name() = %q, want %q", wrapped.Name(), "mydir")
	}
	if !wrapped.IsDir() {
		t.Error("IsDir() should be true")
	}
	if wrapped.Mode() != os.ModeDir|defaultDirMode {
		t.Errorf("Mode() = %v, want %v", wrapped.Mode(), os.ModeDir|defaultDirMode)
	}
}

func TestFileInfoDoubleWrap(t *testing.T) {
	inner := testFileInfo{name: "x", isDir: false}
	once := wrapFileInfo(inner)
	twice := wrapFileInfo(once)

	if twice.Mode() != defaultFileMode {
		t.Errorf("Mode() after double wrap = %v, want %v", twice.Mode(), defaultFileMode)
	}
	if twice.Name() != "x" {
		t.Errorf("Name() = %q, want %q", twice.Name(), "x")
	}
}

func TestFileInfoSys(t *testing.T) {
	inner := testFileInfo{name: "test", isDir: false}
	wrapped := wrapFileInfo(inner)
	if wrapped.Sys() != nil {
		t.Error("Sys() should return nil")
	}
}
