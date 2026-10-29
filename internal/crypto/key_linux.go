//go:build linux

package crypto

import (
	"encoding/base64"
	"errors"
	"os"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "PachinGo"
	keyringUser    = "aes-key"
)

var errKeyringDown = errors.New("keyring unavailable")

// readStoredKey lit la clé dans le trousseau Secret Service.
// Sans trousseau, le fichier local en 0600 sert de repli.
func readStoredKey(path string) ([]byte, error) {
	switch key, err := readKeyring(); {
	case err == nil:
		shredFile(path)
		return key, nil
	case errors.Is(err, errKeyringDown):
		return readPlainKey(path)
	case errors.Is(err, os.ErrNotExist):
		plain, ferr := readPlainKey(path)
		if ferr != nil {
			return nil, ferr
		}
		if werr := writeKeyring(plain); werr != nil {
			return plain, nil
		}
		shredFile(path)
		return plain, nil
	default:
		return nil, err
	}
}

func writeStoredKey(path string, key []byte) error {
	if err := writeKeyring(key); err == nil {
		shredFile(path)
		return nil
	}
	return writePlainKey(path, key)
}

func readKeyring() ([]byte, error) {
	secret, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, errKeyringDown
	}
	raw, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(raw) != aesKeyLen {
		return nil, errors.New("clé du trousseau invalide")
	}
	return raw, nil
}

func writeKeyring(key []byte) error {
	return keyring.Set(keyringService, keyringUser, base64.StdEncoding.EncodeToString(key))
}
