package main

import (
	"compress/gzip"
	"crypto/cipher"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
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
func LoadConfig(bkDir string, key []byte) (*Config, error) {
	fp := path.Join(bkDir, CONFIG_SUFFIX)
	in, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	var in2 io.Reader = in
	if key != nil {
		ds, err := makeDecryptionStream(key, in)
		if err != nil {
			return nil, err
		}
		in2 = ds
	}
	gz, err := gzip.NewReader(in2)
	if err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(gz)
	cfg := &Config{}
	dec.Decode(cfg)
	if cfg.Magic != CONFIG_MAGIC {
		in.Close()
		return nil, errors.New("Invalid config file: missing magic string")
	}
	if cfg.Version != VBK_VERSION {
		in.Close()
		return nil, errors.New(fmt.Sprintf("Unsupported backup version %d, expecting version %d", cfg.Version, VBK_VERSION))
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, bkDir string, key []byte) error {
	if cfg.Magic != CONFIG_MAGIC {
		return errors.New("Invalid config: missing magic string")
	}
	fp := path.Join(bkDir, CONFIG_SUFFIX)
	out, err := os.Create(fp)
	if err != nil {
		return err
	}
	var out2 io.Writer = out
	var es *cipher.StreamWriter
	if key != nil {
		es, err = makeEncryptionStream(key, out)
		if err != nil {
			out.Close()
			return err
		}
		out2 = es
	}
	gz := gzip.NewWriter(out2)
	enc := gob.NewEncoder(gz)
	err = enc.Encode(cfg)
	if err != nil {
		gz.Close()
		if es != nil {
			es.Close()
		}
		out.Close()
		return err
	}
	err = gz.Close()
	if err != nil {
		if es != nil {
			es.Close()
		}
		out.Close()
		return err
	}
	if es != nil {
		err = es.Close()
		if err != nil {
			out.Close()
			return err
		}
	} else {
		err = out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
