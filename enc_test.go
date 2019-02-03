package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"io"
	"testing"
)

func TestEncryption(t *testing.T) {
	key := make([]byte, 100)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		t.Fatal(err)
	}
	const LOG_MAX_SIZE = 25
	const MAX_SIZE = 1 << LOG_MAX_SIZE
	data := make([]byte, MAX_SIZE)
	t.Log("Testing zero buffer len", len(data))
	testEncrypt(t, key, data)
	testEncGzip(t, key, data)
	// first half random, second half zeros
	_, err = io.ReadFull(rand.Reader, data[:MAX_SIZE/2])
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	plaintext := data[:0]
	t.Log("Testing plaintext len", len(plaintext))
	testEncrypt(t, key, plaintext)
	testEncGzip(t, key, plaintext)
	testGzip(t, plaintext)
	for i := uint(0); i <= LOG_MAX_SIZE; i++ {
		start := MAX_SIZE/2 - (1 << (i - 1))
		plaintext = data[start : start+(1<<i)]
		t.Log("Testing plaintext len", len(plaintext))
		testEncrypt(t, key, plaintext)
		testEncGzip(t, key, plaintext)
		testGzip(t, plaintext)
	}
}

func testEncrypt(t *testing.T, key, plaintext []byte) {
	k := sha512.Sum512_256(key)
	ciphertext, err := encryptBytes(&k, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	result, err := decryptBytes(&k, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
	t.Logf("testEncryptSB succeeded with plaintext len: %d, cipherlen: %d, overhead: %d", len(plaintext), len(ciphertext), len(ciphertext)-len(plaintext))
}

func testEncGzip(t *testing.T, key, plaintext []byte) {
	k := sha512.Sum512_256(key)
	ciphertext, err := encGzipBytes(&k, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	result, err := decGunzipBytes(&k, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
	t.Logf("testEncGzipSB succeeded with plaintext len: %d, cipherlen: %d, overhead: %d", len(plaintext), len(ciphertext), len(ciphertext)-len(plaintext))
}

func testGzip(t *testing.T, plaintext []byte) {
	gzipText := gzipBytes(plaintext)
	result, err := gunzipBytes(gzipText)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
	t.Logf("testGzipSB succeeded with plaintext len: %d, gziplen: %d, overhead: %d", len(plaintext), len(gzipText), len(gzipText)-len(plaintext))
}
