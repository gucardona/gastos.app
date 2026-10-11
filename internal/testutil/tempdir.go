// Package testutil holds helpers used only by tests.
package testutil

import (
	"os"
	"testing"
	"time"
)

// TempDir is a t.TempDir that retries the removal: on Windows the SQLite WAL files
// sometimes take a moment to be released after Close.
func TempDir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gastos-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 20; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	})
	return dir
}
