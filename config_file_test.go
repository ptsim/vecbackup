package main

import (
	"os"
	"path"
	"testing"
)

func ConfigTestHelper(t *testing.T, key []byte, chunk_size int) {
	cfg := MakeConfig(chunk_size)
	if cfg.Magic != CONFIG_MAGIC {
		t.Fatal("cfg.Magic is incorrect:", cfg)
	}
	const BKDIR = "."
	err := SaveConfig(cfg, BKDIR, key)
	if err != nil {
		t.Fatal("Cannot save config", err)
	}
	if key != nil {
		_, err = LoadConfig(BKDIR, nil)
		if err == nil {
			t.Fatal("Should not be able to load encrypted config without key")
		}
	}
	bad_key := []byte("d9ksiufhj932098432jflskjflskjflj")
	_, err = LoadConfig(BKDIR, bad_key)
	if err == nil {
		t.Fatal("Should not be able to load config with bad key")
	}
	cfg2, err := LoadConfig(BKDIR, key)
	if err != nil {
		t.Fatal("Cannot load config", err)
	}
	if *cfg != *cfg2 {
		t.Fatal("Configs do not match", cfg, cfg2)
	}
	_ = os.Remove(path.Join(BKDIR, CONFIG_SUFFIX))
}

func TestConfig(t *testing.T) {
	ConfigTestHelper(t, nil, 38542)
	ConfigTestHelper(t, nil, 1)
	key := []byte("hliehf3209hflkhflaskdhflaksdjhf0")
	ConfigTestHelper(t, key, 9229283)
	ConfigTestHelper(t, key, 238493)
}
