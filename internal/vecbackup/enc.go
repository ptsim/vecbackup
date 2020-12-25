package vecbackup

import (
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/pbkdf2"
	"io"
)

type EncKey [32]byte

func (key EncKey) String() string {
	return fmt.Sprintf("Key-%x", []byte(key[:]))
}

func encryptBytes(key *EncKey, text []byte) ([]byte, error) {
	// from golang secretbox example
	// You must use a different nonce for each message you encrypt with the
	// same key. Since the nonce here is 192 bits long, a random value
	// provides a sufficiently small probability of repeats.
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// This encrypts the text and appends the result to the nonce.
	encrypted := secretbox.Seal(nonce[:], text, &nonce, (*[32]byte)((key)))
	return encrypted, nil
}

func decryptBytes(key *EncKey, encrypted []byte) ([]byte, error) {
	var nonce [24]byte
	copy(nonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open(nil, encrypted[24:], &nonce, (*[32]byte)(key))
	if !ok {
		return nil, errors.New("secretbox.Open failed")
	}
	return decrypted, nil
}

func genKey(pw []byte, rounds int) ([]byte, *EncKey, *EncKey, []byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, nil, nil, err
	}
	key := pbkdf2.Key(pw, salt, rounds, 32, sha1.New)
	var masterKey EncKey
	copy(masterKey[:], key)
	var storageKey EncKey
	if _, err := io.ReadFull(rand.Reader, storageKey[:]); err != nil {
		return nil, nil, nil, nil, err
	}
	fpSecret := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, fpSecret); err != nil {
		return nil, nil, nil, nil, err
	}
	return salt, &masterKey, &storageKey, fpSecret, nil
}

func getMasterKey(pw, salt []byte, rounds int) *EncKey {
	key := pbkdf2.Key(pw, salt, rounds, 32, sha1.New)
	var masterKey EncKey
	copy(masterKey[:], key)
	return &masterKey
}
