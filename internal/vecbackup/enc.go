package vecbackup

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"io/ioutil"
)

func encryptBytes(key *[32]byte, text []byte) ([]byte, error) {
	// from golang secretbox example
	// You must use a different nonce for each message you encrypt with the
	// same key. Since the nonce here is 192 bits long, a random value
	// provides a sufficiently small probability of repeats.
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// This encrypts the text and appends the result to the nonce.
	encrypted := secretbox.Seal(nonce[:], text, &nonce, key)
	return encrypted, nil
}

func decryptBytes(key *[32]byte, encrypted []byte) ([]byte, error) {
	var nonce [24]byte
	copy(nonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open(nil, encrypted[24:], &nonce, key)
	if !ok {
		return nil, errors.New("secretbox.Open failed")
	}
	return decrypted, nil
}

func encGzipBytes(key *[32]byte, text []byte) ([]byte, error) {
	var gzipBuf bytes.Buffer
	gzw := gzip.NewWriter(&gzipBuf)
	gzw.Write(text)
	gzw.Close()
	return encryptBytes(key, gzipBuf.Bytes())
}

func decGunzipBytes(key *[32]byte, encrypted []byte) ([]byte, error) {
	gzipText, err := decryptBytes(key, encrypted)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewBuffer(gzipText))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return ioutil.ReadAll(gz)
}

func gzipBytes(text []byte) []byte {
	var gzipBuf bytes.Buffer
	gzw := gzip.NewWriter(&gzipBuf)
	gzw.Write(text)
	gzw.Close()
	return gzipBuf.Bytes()
}

func gunzipBytes(gzipText []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(gzipText))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return ioutil.ReadAll(gz)
}

func GenKey(pw []byte, rounds int) ([]byte, *[32]byte, *[32]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, nil, err
	}
	key := pbkdf2.Key(pw, salt, rounds, 32, sha1.New)
	var masterKey [32]byte
	copy(masterKey[:], key)
	var storageKey [32]byte
	if _, err := io.ReadFull(rand.Reader, storageKey[:]); err != nil {
		return nil, nil, nil, err
	}
	return salt, &masterKey, &storageKey, nil
}

func GetMasterKey(pw, salt []byte, rounds int) *[32]byte {
	key := pbkdf2.Key(pw, salt, rounds, 32, sha1.New)
	var masterKey [32]byte
	copy(masterKey[:], key)
	return &masterKey
}
