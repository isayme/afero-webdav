package webdavfs

import (
	"os"
	"testing"
	"time"
)

var (
	_ os.FileInfo = (*FileInfo)(nil)
)

func TestFileInfo(t *testing.T) {
	now := time.Now()

	t.Run("file", func(t *testing.T) {
		fi := NewFileInfo("test.txt", false, 1024, now)

		if fi.Name() != "test.txt" {
			t.Errorf("Name() = %q, want %q", fi.Name(), "test.txt")
		}
		if fi.Size() != 1024 {
			t.Errorf("Size() = %d, want %d", fi.Size(), 1024)
		}
		if fi.IsDir() {
			t.Error("IsDir() should be false")
		}
		if fi.Mode() != defaultFileMode {
			t.Errorf("Mode() = %v, want %v", fi.Mode(), defaultFileMode)
		}
		if !fi.ModTime().Equal(now) {
			t.Errorf("ModTime() = %v, want %v", fi.ModTime(), now)
		}
		if fi.Sys() != nil {
			t.Errorf("Sys() should be nil")
		}
	})

	t.Run("directory", func(t *testing.T) {
		fi := NewFileInfo("mydir", true, 0, now)

		if fi.Name() != "mydir" {
			t.Errorf("Name() = %q, want %q", fi.Name(), "mydir")
		}
		if !fi.IsDir() {
			t.Error("IsDir() should be true")
		}
		if fi.Mode() != os.ModeDir|defaultDirMode {
			t.Errorf("Mode() = %v, want %v", fi.Mode(), os.ModeDir|defaultDirMode)
		}
	})
}

func TestFileInfoSys(t *testing.T) {
	fi := NewFileInfo("test", false, 100, time.Now())
	if fi.Sys() != nil {
		t.Error("Sys() should return nil")
	}
}

func TestFileInfoContentType(t *testing.T) {
	now := time.Now()
	fi := NewFileInfo("test.html", false, 100, now)
	fi.cType = "text/html"
	fi.etag = `"abc123"`

	if fi.ContentType() != "text/html" {
		t.Errorf("ContentType() = %q, want %q", fi.ContentType(), "text/html")
	}
	if fi.ETag() != `"abc123"` {
		t.Errorf("ETag() = %q, want %q", fi.ETag(), `"abc123"`)
	}
}
