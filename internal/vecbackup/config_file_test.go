package vecbackup

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func EncConfigTestHelper(t *testing.T, tmpDir, pwFile, badPwFile string, chunk_size int) {
	t.Logf("Testing encconfig pwfile <%s> badpwfile <%s> chunksize %d", pwFile, badPwFile, chunk_size)
	cfg := MakeConfig(chunk_size)
	if cfg.Magic != CONFIG_MAGIC {
		t.Fatal("cfg.Magic is incorrect:", cfg)
	}
	_ = os.Remove(path.Join(tmpDir, CONFIG_FILE))
	defer os.Remove(path.Join(tmpDir, CONFIG_FILE))
	err := WriteNewConfig(pwFile, tmpDir, DEFAULT_PBKDF2_ITERATIONS, cfg)
	if err != nil {
		t.Fatal("Cannot save config:", err)
	}
	if pwFile != "" {
		_, err = GetConfig("", tmpDir)
		if err == nil {
			t.Fatal("Should not be able to load encrypted config without pw file")
		}
	}
	_, err = GetConfig(badPwFile, tmpDir)
	if err == nil {
		t.Fatal("Should not be able to load config with bad pw file")
	}
	cfg2, err := GetConfig(pwFile, tmpDir)
	if err != nil {
		t.Fatal("Cannot load enc config", err)
	}
	if !EqualConfig(cfg, cfg2) {
		t.Fatal("Configs in enc config do not match", cfg, cfg2)
	}
}

func TestEncConfig(t *testing.T) {
	const TMPDIR = "./test_enc_dir"
	pwFile := path.Join(TMPDIR, PWFILE)
	badPwFile := path.Join(TMPDIR, PWFILE+"_bad")
	os.Mkdir(TMPDIR, 0755)
	defer os.RemoveAll(TMPDIR)
	ioutil.WriteFile(pwFile, []byte("oicewoe90390j0w9jf0wejf0weh"), 0644)
	ioutil.WriteFile(badPwFile, []byte("f00fjsoidfjsodjhfosjd"), 0644)
	EncConfigTestHelper(t, TMPDIR, "", badPwFile, 38542)
	EncConfigTestHelper(t, TMPDIR, "", badPwFile, 1)
	EncConfigTestHelper(t, TMPDIR, pwFile, badPwFile, 9229283)
	EncConfigTestHelper(t, TMPDIR, pwFile, badPwFile, 238493)
}
