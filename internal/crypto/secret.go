package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	keyFileName = "pachingo.key"
	aesKeyLen   = 32
)

// KeyPath retourne le chemin du fichier de clé local.
func KeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pachingo", keyFileName), nil
}

// LoadOrCreateKey charge la clé AES-256, ou en crée une.
// Elle est protégée par DPAPI (Windows) ou le trousseau (Linux).
// Une clé historique en clair est migrée au premier accès.
func LoadOrCreateKey() ([]byte, error) {
	path, err := KeyPath()
	if err != nil {
		return nil, err
	}
	return loadOrCreateAt(path)
}

func loadOrCreateAt(path string) ([]byte, error) {
	key, err := readStoredKey(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, aesKeyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := writeStoredKey(path, key); err != nil {
		return nil, err
	}
	return key, nil
}

func readPlainKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != aesKeyLen {
		return nil, fmt.Errorf("invalid key length in %s", path)
	}
	return data, nil
}

func writePlainKey(path string, key []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, key, 0o600)
}

func shredFile(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	_ = os.WriteFile(path, make([]byte, info.Size()), 0o600)
	_ = os.Remove(path)
}

// Encrypt encodes plaintext as base64(nonce|ciphertext) using AES-GCM.
func Encrypt(plaintext string) (string, error) {
	key, err := LoadOrCreateKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func Decrypt(encoded string) (string, error) {
	key, err := LoadOrCreateKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Fingerprint returns a short hex fingerprint of the local key (for diagnostics).
func Fingerprint() (string, error) {
	key, err := LoadOrCreateKey()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(key)
	return fmt.Sprintf("%x", sum[:8]), nil
}
