package webdavfs

import (
	"testing"
)

func TestErrors(t *testing.T) {
	if ErrNotSupported.Error() != "operation not supported" {
		t.Errorf("unexpected error message: %s", ErrNotSupported)
	}
	if ErrInvalidSeek.Error() != "invalid seek offset" {
		t.Errorf("unexpected error message: %s", ErrInvalidSeek)
	}
	if ErrReadOnly.Error() != "file opened read-only" {
		t.Errorf("unexpected error message: %s", ErrReadOnly)
	}
	if ErrWriteOnly.Error() != "file opened write-only" {
		t.Errorf("unexpected error message: %s", ErrWriteOnly)
	}
}
