package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLegacyPlainKeyIsReused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pachingo.key")
	key := bytes.Repeat([]byte{9}, aesKeyLen)
	if err := os.WriteFile(path, key, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadOrCreateAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("legacy key was not reused")
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && bytes.Equal(data, key) {
		t.Fatal("plaintext key still on disk")
	}
	again, err := loadOrCreateAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, key) {
		t.Fatal("reloaded key differs")
	}
}
