package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	CONFIG_MAGIC     = "PTVBKCFG"
	ENC_CONFIG_MAGIC = "PTVBKCFGENC"
	VBK_VERSION      = 1
	CONFIG_FILE      = "vecbackup-config"
)

type Config struct {
	EncryptionKey *[32]byte
	Magic         string
	Version       int
	ChunkSize     int
}

type EncConfig struct {
	Magic            string
	Version          int
	EncryptionType   int
	PBKDF2Iterations int
	Salt             []byte
	Config           []byte
}

const (
	NO_ENCRYPTION        = 0
	SYMMETRIC_ENCRYPTION = 1
)

//---------------------------------------------------------------------------
func MakeConfig(chunk_size int) *Config {
	return &Config{EncryptionKey: nil, Magic: CONFIG_MAGIC, Version: VBK_VERSION, ChunkSize: chunk_size}
}

func checkConfig(cfg *Config) error {
	if cfg.Magic != CONFIG_MAGIC {
		return errors.New("Invalid config file: missing magic string")
	}
	if cfg.Version != VBK_VERSION {
		return errors.New(fmt.Sprintf("Unsupported backup version %d, expecting version %d", cfg.Version, VBK_VERSION))
	}
	return nil
}

func configToBytes(cfg *Config) ([]byte, error) {
	err := checkConfig(cfg)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(cfg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func configFromBytes(b []byte) (*Config, error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	cfg := &Config{}
	dec.Decode(cfg)
	err := checkConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func EqualConfig(cfg1, cfg2 *Config) bool {
	return cfg1.Magic == cfg2.Magic && cfg1.Version == cfg2.Version && cfg1.ChunkSize == cfg2.ChunkSize && EqualKey(cfg1.EncryptionKey, cfg2.EncryptionKey)
}

func EqualKey(k1, k2 *[32]byte) bool {
	if k1 == nil || k2 == nil {
		return k1 == k2
	}
	return bytes.Compare(k1[:], k2[:]) == 0
}

func checkEncConfig(ec *EncConfig) error {
	if ec.Magic != ENC_CONFIG_MAGIC {
		return errors.New("Invalid enc config: missing magic string")
	}
	if ec.Version != VBK_VERSION {
		return errors.New(fmt.Sprintf("Unsupported backup version %d, expecting version %d", ec.Version, VBK_VERSION))
	}
	if ec.EncryptionType != NO_ENCRYPTION && ec.EncryptionType != SYMMETRIC_ENCRYPTION {
		return errors.New(fmt.Sprintf("Unsupposed encryption method: %d", ec.EncryptionType))
	}
	if ec.EncryptionType == SYMMETRIC_ENCRYPTION {
		if len(ec.Salt) == 0 {
			return errors.New("Invalid enc config: Missing salt")
		}
	}
	return nil
}

func readEncConfig(fp string) (*EncConfig, error) {
	in, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	dec := gob.NewDecoder(in)
	ec := &EncConfig{}
	dec.Decode(ec)
	err = checkEncConfig(ec)
	if err != nil {
		return nil, err
	}
	return ec, nil
}

func writeEncConfig(fp string, ec *EncConfig) error {
	err := checkEncConfig(ec)
	if err != nil {
		return err
	}
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

func makeEncConfig(pwFile string, rounds int, cfg *Config) (*EncConfig, error) {
	if cfg.EncryptionKey != nil {
		return nil, errors.New("Encryption key is not empty in Config")
	}
	if pwFile == "" {
		configBytes, err := configToBytes(cfg)
		if err != nil {
			return nil, err
		}
		return &EncConfig{Magic: ENC_CONFIG_MAGIC, Version: VBK_VERSION, EncryptionType: NO_ENCRYPTION, Config: configBytes}, nil
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot read password file: %s", pwFile))
	}
	salt, masterKey, storageKey, err := GenKey(pw, rounds)
	if err != nil {
		return nil, err
	}
	cfg.EncryptionKey = storageKey
	configBytes, err := configToBytes(cfg)
	if err != nil {
		return nil, err
	}
	encCfg, err := encryptBytes(masterKey, configBytes)
	if err != nil {
		return nil, err
	}
	return &EncConfig{Magic: ENC_CONFIG_MAGIC, Version: VBK_VERSION, EncryptionType: SYMMETRIC_ENCRYPTION, PBKDF2Iterations: rounds, Salt: salt, Config: encCfg}, nil
}

func WriteNewConfig(pwFile, bkDir string, rounds int, cfg *Config) error {
	encCfg, err := makeEncConfig(pwFile, rounds, cfg)
	if err != nil {
		return err
	}
	return writeEncConfig(path.Join(bkDir, CONFIG_FILE), encCfg)
}

func GetConfig(pwFile, bkDir string) (*Config, error) {
	ec, err := readEncConfig(path.Join(bkDir, CONFIG_FILE))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot read enc config file: %s", err))
	}
	if ec.EncryptionType == NO_ENCRYPTION {
		if pwFile != "" {
			return nil, errors.New("Backup is not encrypted")
		}
		cfg, err := configFromBytes(ec.Config)
		if err != nil {
			return nil, err
		}
		if cfg.EncryptionKey != nil {
			return nil, errors.New("Encryption key is not empty")
		}
		return cfg, nil
	}
	if pwFile == "" {
		return nil, errors.New("Backup is encrypted")
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot read pw file: %s", err))
	}
	masterKey := GetMasterKey(pw, ec.Salt, ec.PBKDF2Iterations)
	configBytes, err := decryptBytes(masterKey, ec.Config)
	cfg, err := configFromBytes(configBytes)
	if err != nil {
		return nil, errors.New("Wrong password")
	}
	return cfg, nil
}
