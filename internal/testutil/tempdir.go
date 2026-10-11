// Package testutil reúne ajudantes usados só por testes.
package testutil

import (
	"os"
	"testing"
	"time"
)

// TempDir é um t.TempDir que tenta remover de novo: no Windows os arquivos WAL do SQLite
// às vezes demoram a ser liberados depois do Close.
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
