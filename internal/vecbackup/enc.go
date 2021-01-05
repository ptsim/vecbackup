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

func encryptBytes(key *EncKey, text []byte, out []byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}
	total := len(nonce) + len(text) + secretbox.Overhead
	if cap(out) < len(nonce)+len(text) {
		out = make([]byte, len(nonce), total)
	} else {
		out = out[:len(nonce)]
	}
	copy(out, nonce[:])
	encrypted := secretbox.Seal(out, text, &nonce, (*[32]byte)((key)))
	return encrypted, nil
}

func decryptBytes(key *EncKey, encrypted []byte, out []byte) ([]byte, error) {
	var nonce [24]byte
	copy(nonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open(out[:0], encrypted[24:], &nonce, (*[32]byte)(key))
	if !ok {
		return nil, errors.New("Unable to decrypt")
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
