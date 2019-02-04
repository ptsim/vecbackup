package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha1"
	"encoding/gob"
	"errors"
	"fmt"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"io/ioutil"
	"os"
	"path"
)

const PBKDF_ROUNDS = 100000

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

type EncConfig struct {
	Magic          string
	EncryptionType int
	Salt           []byte
	EncryptionKey  []byte
}

const (
	ENC_CONFIG_MAGIC = "PTVBKCFG"
	ENC_CONFIG       = "vecbackup-enc-config"
	NO_ENCRYPTION    = iota
	SYMMETRIC_ENCRYPTION
)

func CheckEncConfig(ec *EncConfig) error {
	if ec.Magic != ENC_CONFIG_MAGIC {
		return errors.New("Invalid enc config: missing magic string")
	}
	if ec.EncryptionType != NO_ENCRYPTION && ec.EncryptionType != SYMMETRIC_ENCRYPTION {
		return errors.New(fmt.Sprintf("Unsupposed encryption method: %d", ec.EncryptionType))
	}
	if ec.EncryptionType == SYMMETRIC_ENCRYPTION {
		if len(ec.Salt) == 0 {
			return errors.New("Invalid enc config: Missing salt")
		}
		if len(ec.EncryptionKey) == 0 {
			return errors.New("Invalid enc config: Missing encrypted key")
		}
	}
	return nil
}

func ReadEncConfig(fp string) (*EncConfig, error) {
	in, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	dec := gob.NewDecoder(in)
	ec := &EncConfig{}
	dec.Decode(ec)
	err = CheckEncConfig(ec)
	if err != nil {
		return nil, err
	}
	return ec, nil
}

func WriteEncConfig(bkDir string, ec *EncConfig) error {
	err := CheckEncConfig(ec)
	if err != nil {
		return err
	}
	fp := path.Join(bkDir, ENC_CONFIG)
	_, err = os.Stat(fp)
	if !os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("Enc config file already exists: %s", fp))
	}
	out, err := os.Create(fp)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(out)
	err = enc.Encode(ec)
	if err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func WriteNewEncConfig(pwFile, bkDir string) error {
	if pwFile == "" {
		return WriteEncConfig(bkDir, &EncConfig{Magic: ENC_CONFIG_MAGIC, EncryptionType: NO_ENCRYPTION})
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read password file: %s", pwFile))
	}
	ec := EncConfig{Magic: ENC_CONFIG_MAGIC, EncryptionType: SYMMETRIC_ENCRYPTION, Salt: make([]byte, 32)}
	if _, err := io.ReadFull(rand.Reader, ec.Salt); err != nil {
		return err
	}
	key2 := pbkdf2.Key(pw, ec.Salt, PBKDF_ROUNDS, 32, sha1.New)
	var key3 [32]byte
	copy(key3[:], key2)
	var rawKey [32]byte
	if _, err := io.ReadFull(rand.Reader, rawKey[:]); err != nil {
		return err
	}
	ec.EncryptionKey, err = encryptBytes(&key3, rawKey[:])
	if err != nil {
		return err
	}
	return WriteEncConfig(bkDir, &ec)
}

func GetKey(pwFile, bkDir string) (*[32]byte, error) {
	ec, err := ReadEncConfig(path.Join(bkDir, ENC_CONFIG))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot read enc config file: %s", err))
	}
	if ec.EncryptionType == NO_ENCRYPTION {
		if pwFile != "" {
			return nil, errors.New("Backup is not encrypted")
		}
		return nil, nil
	}
	if pwFile == "" {
		return nil, errors.New("Backup is encrypted")
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot read pw file: %s", err))
	}
	key2 := pbkdf2.Key(pw, ec.Salt, PBKDF_ROUNDS, 32, sha1.New)
	var key3 [32]byte
	copy(key3[:], key2)
	key, err := decryptBytes(&key3, ec.EncryptionKey)
	if err != nil {
		return nil, errors.New("Wrong password")
	}
	if len(key) != 32 {
		return nil, errors.New("Encrypted key is not 32 bytes long")
	}
	var key4 [32]byte
	copy(key4[:], key)
	return &key4, nil
}
