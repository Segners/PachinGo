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

const keyFileName = "pachingo.key"

// KeyPath returns the path used to store the local encryption key.
func KeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pachingo", keyFileName), nil
}

// LoadOrCreateKey loads the AES-256 key from disk or creates a new one.
func LoadOrCreateKey() ([]byte, error) {
	path, err := KeyPath()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("invalid key length in %s", path)
		}
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
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
