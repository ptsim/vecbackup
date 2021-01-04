package vecbackup

import (
	"bytes"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
)

type Config struct {
	ChunkSize     int32
	Compress      CompressionMode
	EncryptionKey *EncKey
	FPSecret      []byte
}

//---------------------------------------------------------------------------
func checkConfig(cfg *Config, encrypted bool) {
	if encrypted {
		if cfg.EncryptionKey == nil {
			panic("Internal error, invalid encryption key.")
		}
		if cfg.FPSecret == nil || len(cfg.FPSecret) < 64 {
			panic("Internal error, invalid secret.")
		}
	} else {
		if cfg.EncryptionKey != nil || cfg.FPSecret != nil {
			panic("Internal error, extra key")
		}
	}
}

func configToBytes(cfg *Config, encrypted bool) ([]byte, error) {
	checkConfig(cfg, encrypted)
	cp := ConfigProto{ChunkSize: cfg.ChunkSize, Compress: cfg.Compress}
	if encrypted {
		cp.FPSecret = cfg.FPSecret
		cp.EncryptionKey = cfg.EncryptionKey[:]
	}
	return proto.Marshal(&cp)
}

func configFromBytes(b []byte, encrypted bool) (*Config, error) {
	cp := ConfigProto{}
	if err := proto.Unmarshal(b, &cp); err != nil {
		return nil, err
	}
	cfg := &Config{ChunkSize: cp.ChunkSize, Compress: cp.Compress}
	if encrypted {
		cfg.FPSecret = cp.FPSecret
		if len(cp.EncryptionKey) != 32 {
			return nil, errors.New("Invalid encryption key in config file.")
		}
		var mykey EncKey
		copy(mykey[:], cp.EncryptionKey)
		cfg.EncryptionKey = &mykey
	}
	checkConfig(cfg, encrypted)
	return cfg, nil
}

func writeEncConfig(sm StorageMgr, d, p string, t EncType, iterations int64, salt, config []byte) error {
	ec := EncConfigProto{Version: VC_VERSION, Type: t, Iterations: iterations, Salt: salt, Config: config}
	pb, err := proto.Marshal(&ec)
	if err != nil {
		return err
	}
	if exists, err := sm.ExistInDir(d, p); exists {
		return fmt.Errorf("Config file already exists in repo: %s", d)
	} else if err != nil {
		return err
	}
	fp := sm.JoinPath(d, p)
	var buf bytes.Buffer
	buf.Write([]byte(VC_MAGIC))
	buf.Write(pb)
	return sm.WriteFile(fp, buf.Bytes())
}

func WriteNewConfig(pwFile string, sm StorageMgr, repo string, rounds int, cfg *Config) error {
	if pwFile == "" {
		configBytes, err := configToBytes(cfg, false)
		if err != nil {
			return err
		}
		return writeEncConfig(sm, repo, CONFIG_FILE, EncType_NO_ENCRYPTION, 0, nil, configBytes)
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return fmt.Errorf("Cannot read password file: %s", pwFile)
	}
	salt, masterKey, storageKey, fpSecret, err := genKey(pw, rounds)
	if err != nil {
		return err
	}
	cfg.EncryptionKey = storageKey
	cfg.FPSecret = fpSecret
	configBytes, err := configToBytes(cfg, true)
	if err != nil {
		return err
	}
	enc, err := encryptBytes(masterKey, configBytes)
	if err != nil {
		return err
	}
	return writeEncConfig(sm, repo, CONFIG_FILE, EncType_SYMMETRIC, int64(rounds), salt, enc)
}

func decodeConfig(data []byte) (*EncConfigProto, error) {
	var in = bytes.NewBuffer(data)
	ec := EncConfigProto{}
	var h [len(VC_MAGIC)]byte
	if _, err := io.ReadFull(in, h[:]); err != nil || bytes.Compare(h[:], []byte(VC_MAGIC)) != 0 {
		return nil, errors.New("Invalid config file")
	}
	if b, err := ioutil.ReadAll(in); err != nil {
		return nil, err
	} else {
		if err := proto.Unmarshal(b, &ec); err != nil {
			return nil, err
		}
	}
	if ec.Version != VC_VERSION {
		return nil, errors.New("Incompatible config file.")
	}
	return &ec, nil
}

func GetConfig(pwFile string, sm StorageMgr, repo string) (*Config, error) {
	b, err := sm.ReadFile(sm.JoinPath(repo, CONFIG_FILE), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		return nil, err
	}
	ec, err := decodeConfig(b)
	if err != nil {
		return nil, fmt.Errorf("Invalid repository: %s", err)
	}
	if ec.Type == EncType_NO_ENCRYPTION {
		if pwFile != "" {
			return nil, errors.New("Backup is not encrypted")
		}
		return configFromBytes(ec.Config, false)
	} else if ec.Type != EncType_SYMMETRIC {
		return nil, errors.New("Unknown encryption type.")
	}
	if pwFile == "" {
		return nil, errors.New("Backup is encrypted")
	}
	pw, err := ioutil.ReadFile(pwFile)
	if err != nil {
		return nil, fmt.Errorf("Cannot read pw file: %s", err)
	}
	masterKey := getMasterKey(pw, ec.Salt, int(ec.Iterations))
	configBytes, err := decryptBytes(masterKey, ec.Config)
	if err != nil {
		return nil, errors.New("Wrong password")
	}
	return configFromBytes(configBytes, true)
}
