package webdavfs

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/webdav"

	"github.com/studio-b12/gowebdav"
)

// setupTestServer starts an embedded WebDAV server backed by a temp directory
// and returns a configured Fs pointing at it along with the server root URL.
func setupTestServer(t *testing.T) (rootURL string) {
	t.Helper()

	tmpDir := t.TempDir()

	h := &webdav.Handler{
		FileSystem: webdav.Dir(tmpDir),
		LockSystem: webdav.NewMemLS(),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	server := &http.Server{Handler: h}

	go func() { server.Serve(listener) }()

	t.Cleanup(func() {
		server.Close()
		listener.Close()
	})

	addr := listener.Addr().String()
	rootURL = fmt.Sprintf("http://%s/", addr)
	return rootURL
}

// newTestFs creates an Fs backed by a test WebDAV server and waits for it
// to become ready.  Tests that cannot reach the server are silently skipped.
func newTestFs(t *testing.T) *Fs {
	t.Helper()

	rootURL := setupTestServer(t)

	if err := waitForServer(rootURL); err != nil {
		t.Skipf("test server not ready: %v", err)
	}

	client := gowebdav.NewClient(rootURL, "", "")
	client.SetTimeout(5 * time.Second)

	return New(client)
}

// waitForServer polls the given URL until the server responds or a deadline
// is reached.
func waitForServer(rootURL string) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(rootURL)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("server did not start within deadline")
}
