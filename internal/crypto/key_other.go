//go:build !windows && !linux

package crypto

func readStoredKey(path string) ([]byte, error) {
	return readPlainKey(path)
}

func writeStoredKey(path string, key []byte) error {
	return writePlainKey(path, key)
}
