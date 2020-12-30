package vecbackup

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func equalConfig(cfg1, cfg2 *Config) bool {
	return cfg1.ChunkSize == cfg2.ChunkSize && equalKey(cfg1.EncryptionKey, cfg2.EncryptionKey) && bytes.Compare(cfg1.FPSecret, cfg2.FPSecret) == 0 && cfg1.Compress == cfg2.Compress
}

func equalKey(k1, k2 *EncKey) bool {
	if k1 == nil || k2 == nil {
		return k1 == k2
	}
	return bytes.Compare(k1[:], k2[:]) == 0
}

func EncConfigTestHelper(t *testing.T, tmpDir, pwFile, badPwFile string, chunk_size int32, compress CompressionMode) {
	t.Logf("Testing encconfig pwfile <%s> badpwfile <%s> chunksize %d", pwFile, badPwFile, chunk_size)
	cfg := &Config{ChunkSize: chunk_size, Compress: compress}
	_ = os.Remove(path.Join(tmpDir, CONFIG_FILE))
	defer os.Remove(path.Join(tmpDir, CONFIG_FILE))
	sm, repo2 := GetStorageMgr(tmpDir)
	err := WriteNewConfig(pwFile, sm, repo2, 200000, cfg)
	if err != nil {
		t.Fatal("Cannot save config:", err)
	}
	if pwFile != "" {
		_, err = GetConfig("", sm, repo2)
		if err == nil {
			t.Fatal("Should not be able to load encrypted config without pw file")
		}
	}
	_, err = GetConfig(badPwFile, sm, repo2)
	if err == nil {
		t.Fatal("Should not be able to load config with bad pw file")
	}
	cfg2, err := GetConfig(pwFile, sm, repo2)
	if err != nil {
		t.Fatal("Cannot load enc config", err)
	}
	if !equalConfig(cfg, cfg2) {
		t.Fatal("Configs in enc config do not match", cfg, cfg2)
	}
}

func TestEncConfig(t *testing.T) {
	const TMPDIR = "./test_enc_dir"
	pwFile := path.Join(TMPDIR, "goodpw")
	badPwFile := path.Join(TMPDIR, "badpw")
	os.Mkdir(TMPDIR, 0755)
	defer os.RemoveAll(TMPDIR)
	err := ioutil.WriteFile(pwFile, []byte("oicewoe90390j0w9jf0wejf0weh"), 0444)
	if err != nil {
		t.Fatalf("Failed to create passwd file: %s", err)
	}
	err = ioutil.WriteFile(badPwFile, []byte("f00fjsoidfjsodjhfosjd"), 0444)
	if err != nil {
		t.Fatalf("Failed to create bad passwd file: %s", err)
	}
	EncConfigTestHelper(t, TMPDIR, "", badPwFile, 38542, CompressionMode_AUTO)
	EncConfigTestHelper(t, TMPDIR, "", badPwFile, 1, CompressionMode_SLOW)
	EncConfigTestHelper(t, TMPDIR, pwFile, badPwFile, 9229283, CompressionMode_YES)
	EncConfigTestHelper(t, TMPDIR, pwFile, badPwFile, 238493, CompressionMode_NO)
}
