package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
)

const (
	CONFIG_MAGIC  = "PTVBKCFG"
	VBK_VERSION   = 1
	CONFIG_SUFFIX = "vecbackup-config"
)

type Config struct {
	Magic     string
	Version   int
	ChunkSize int
}

//---------------------------------------------------------------------------
func MakeConfig(chunk_size int) *Config {
	return &Config{Magic: CONFIG_MAGIC, Version: VBK_VERSION, ChunkSize: chunk_size}
}

//---------------------------------------------------------------------------
func LoadConfig(bkDir string, key *[32]byte) (*Config, error) {
	fp := path.Join(bkDir, CONFIG_SUFFIX)
	ciphertext, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	var text []byte
	if key == nil {
		text, err = gunzipBytes(ciphertext)
	} else {
		text, err = decGunzipBytes(key, ciphertext)
	}
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(text)
	dec := gob.NewDecoder(buf)
	cfg := &Config{}
	dec.Decode(cfg)
	if cfg.Magic != CONFIG_MAGIC {
		return nil, errors.New("Invalid config file: missing magic string")
	}
	if cfg.Version != VBK_VERSION {
		return nil, errors.New(fmt.Sprintf("Unsupported backup version %d, expecting version %d", cfg.Version, VBK_VERSION))
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, bkDir string, key *[32]byte) error {
	if cfg.Magic != CONFIG_MAGIC {
		return errors.New("Invalid config: missing magic string")
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(cfg)
	if err != nil {
		return err
	}
	var ciphertext []byte
	if key == nil {
		ciphertext = gzipBytes(buf.Bytes())
		err = nil
	} else {
		ciphertext, err = encGzipBytes(key, buf.Bytes())
	}
	if err != nil {
		return err
	}
	fp := path.Join(bkDir, CONFIG_SUFFIX)
	return ioutil.WriteFile(fp, ciphertext, 0666)
}
