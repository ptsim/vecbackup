package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"io"
	"io/ioutil"
	"testing"
)

func TestEncryption(t *testing.T) {
	key := make([]byte, 100)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1<<20)
	_, err = io.ReadFull(rand.Reader, data)
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	plaintext := data[:0]
	t.Log("Testing plaintext len", len(plaintext))
	testEncrypt(t, key, plaintext)
	testEncrypt2(t, key, plaintext)
	testEncrypt3(t, key, plaintext)
	testEncrypt4(t, key, plaintext)
	for i := uint(0); i < 20; i++ {
		plaintext = data[:(1 << i)]
		t.Log("Testing plaintext len", len(plaintext))
		testEncrypt(t, key, plaintext)
		testEncrypt2(t, key, plaintext)
		testEncrypt3(t, key, plaintext)
		testEncrypt4(t, key, plaintext)
	}
}

func testEncrypt(t *testing.T, key, plaintext []byte) {
	k := sha512.Sum512_256(key)
	ciphertext, err := encryptBytes(k[:], plaintext)
	if err != nil {
		t.Fatal(err)
	}

	result, err := decryptBytes(k[:], ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
}

func testEncrypt2(t *testing.T, key []byte, plaintext []byte) {
	k := sha512.Sum512_256(key)
	var out bytes.Buffer
	w, err := makeEncryptionStream(k[:], &out)
	if err != nil {
		t.Fatal(err)
	}
	n, err := w.Write(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(plaintext) {
		t.Fatal("Didn't write enough", n, len(plaintext))
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := out.Bytes()

	result, err := decryptBytes(k[:], ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
}

func testEncrypt3(t *testing.T, key []byte, plaintext []byte) {
	k := sha512.Sum512_256(key)
	var out bytes.Buffer
	w, err := makeEncryptionStream(k[:], &out)
	if err != nil {
		t.Fatal(err)
	}
	n, err := w.Write(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(plaintext) {
		t.Fatal("Didn't write enough", n, len(plaintext))
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := out.Bytes()

	r, err := makeDecryptionStream(k[:], bytes.NewReader(ciphertext))
	result, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
}

func testEncrypt4(t *testing.T, key []byte, plaintext []byte) {
	k := sha512.Sum512_256(key)
	ciphertext, err := encryptBytes(k[:], plaintext)
	if err != nil {
		t.Fatal(err)
	}

	r, err := makeDecryptionStream(k[:], bytes.NewReader(ciphertext))
	result, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("result does not match plaintext\n")
	}
}
