//go:build windows

package crypto

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// readStoredKey lit la clé. Un fichier de 32 octets est l'ancien format en clair :
// il est réécrit protégé par DPAPI.
func readStoredKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == aesKeyLen {
		if err := writeStoredKey(path, data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return unprotectKey(data)
}

func writeStoredKey(path string, key []byte) error {
	blob, err := protectKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return err
	}
	backup := path + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Rename(backup, path)
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func protectKey(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty key")
	}
	in := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var out windows.DataBlob
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data)))) }()
	buf := make([]byte, out.Size)
	copy(buf, unsafe.Slice(out.Data, out.Size))
	return buf, nil
}

func unprotectKey(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty protected key")
	}
	in := windows.DataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	var out windows.DataBlob
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data)))) }()
	buf := make([]byte, out.Size)
	copy(buf, unsafe.Slice(out.Data, out.Size))
	if len(buf) != aesKeyLen {
		return nil, fmt.Errorf("invalid unlocked key length")
	}
	return buf, nil
}
