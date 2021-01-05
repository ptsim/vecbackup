package vecbackup

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
	t.Log("Testing nil buffer")
	testEncrypt(t, key, nil)
	// first half random, second half zeros
	const LOG_MAX_SIZE = 25
	const MAX_SIZE = 1 << LOG_MAX_SIZE
	data := make([]byte, MAX_SIZE)
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
	for i := uint(0); i <= LOG_MAX_SIZE; i++ {
		start := MAX_SIZE/2 - (1 << (i - 1))
		plaintext = data[start : start+(1<<i)]
		t.Log("Testing plaintext len", len(plaintext))
		testEncrypt(t, key, plaintext)
	}
}

func testEncrypt(t *testing.T, key, plaintext []byte) {
	var k EncKey = sha512.Sum512_256(key)
	ciphertext, err := encryptBytes(&k, plaintext, nil)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext2, err := encryptBytes(&k, plaintext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(ciphertext, ciphertext2) == 0 {
		t.Fatal("Ciphertext should not be the same")
	}
	result, err := decryptBytes(&k, ciphertext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result, plaintext) != 0 {
		t.Fatal("Decrypted results does not match plaintext\n")
	}
	result2, err := decryptBytes(&k, ciphertext2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(result2, plaintext) != 0 {
		t.Fatal("Decrypted results does not match plaintext\n")
	}
	t.Logf("testEncryptSB succeeded with plaintext len: %d, ciphertext len: %d, overhead: %d", len(plaintext), len(ciphertext), len(ciphertext)-len(plaintext))
}
