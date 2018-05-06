package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/gob"
	"errors"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"io/ioutil"
	"os"
	"path"
)

func makeEncryptionStream(key []byte, out io.Writer) (*cipher.StreamWriter, error) {
	if len(key) != 32 {
		return nil, errors.New("Incorrect key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	var iv [aes.BlockSize]byte
	if _, err := io.ReadFull(rand.Reader, iv[:]); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv[:])
	n, err := out.Write(iv[:])
	if err != nil {
		return nil, err
	}
	if n != len(iv) {
		return nil, io.ErrShortWrite
	}
	return &cipher.StreamWriter{S: cfb, W: out}, nil
}

func makeDecryptionStream(key []byte, in io.Reader) (*cipher.StreamReader, error) {
	if len(key) != 32 {
		return nil, errors.New("Incorrect key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	var iv [aes.BlockSize]byte
	if _, err := io.ReadFull(in, iv[:]); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBDecrypter(block, iv[:])
	return &cipher.StreamReader{S: cfb, R: in}, nil
}

func encryptBytes(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := []byte(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func decryptBytes(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	return text, nil
}

type EncConfig struct {
	Magic          string
	EncryptionType int
	Salt           []byte
	Check          []byte
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
	key2 := pbkdf2.Key(pw, ec.Salt, 10000, 32, sha1.New)
	var check [32]byte
	if _, err := io.ReadFull(rand.Reader, check[:16]); err != nil {
		return err
	}
	copy(check[16:32], check[:16])
	ec.Check, err = encryptBytes(key2, check[:])
	if err != nil {
		return err
	}
	var rawKey [32]byte
	if _, err := io.ReadFull(rand.Reader, rawKey[:]); err != nil {
		return err
	}
	ec.EncryptionKey, err = encryptBytes(key2, rawKey[:])
	if err != nil {
		return err
	}
	return WriteEncConfig(bkDir, &ec)
}

func GetKey(pwFile, bkDir string) ([]byte, error) {
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
	key2 := pbkdf2.Key(pw, ec.Salt, 10000, 32, sha1.New)
	check, err := decryptBytes(key2, ec.Check)
	if len(check) != 32 || bytes.Compare(check[:16], check[16:32]) != 0 {
		return nil, errors.New("Wrong password")
	}
	key, err := decryptBytes(key2, ec.EncryptionKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot decrypt enc key: %s", err))
	}
	return append([]byte(nil), key[:]...), nil
}
