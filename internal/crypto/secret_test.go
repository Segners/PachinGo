package crypto_test

import (
	"testing"

	"github.com/Segners/PachinGo/internal/crypto"
)

func TestEncryptDecrypt(t *testing.T) {
	enc, err := crypto.Encrypt("s3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "" || enc == "s3cret!" {
		t.Fatal("expected ciphertext")
	}
	plain, err := crypto.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "s3cret!" {
		t.Fatalf("got %q", plain)
	}
}
