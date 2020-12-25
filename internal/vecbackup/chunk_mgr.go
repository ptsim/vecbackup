package vecbackup

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type CMgr struct {
	dir      string
	key      *EncKey
	compress CompressionMode
}

func MakeCMgr(repo string, key *EncKey, compress CompressionMode) *CMgr {
	return &CMgr{dir: path.Join(repo, CHUNK_DIR), key: key, compress: compress}
}

const DIR_PREFIX_SIZE = 2

func FPtoName(fp FP) string {
	return fmt.Sprintf("%x", [32]byte(fp))
}

func nameToFP(name string) (FP, error) {
	var fp FP
	x, err := hex.DecodeString(name)
	if err != nil || len(x) != len(fp) {
		return fp, fmt.Errorf("Invalid chunk name: %s\n", name)
	}
	copy(fp[:], x)
	return fp, nil
}

func (cm *CMgr) FindChunk(fp FP) bool {
	name := FPtoName(fp)
	p := path.Join(cm.dir, name[:DIR_PREFIX_SIZE], name)
	_, err := os.Stat(p)
	return err == nil
}

func (cm *CMgr) ReadChunk(fp FP) ([]byte, error) {
	name := FPtoName(fp)
	f := path.Join(cm.dir, name[:DIR_PREFIX_SIZE], name)
	ciphertext, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}
	var text []byte
	if cm.key != nil {
		text, err = decryptBytes(cm.key, ciphertext)
		if err != nil {
			return nil, err
		}
	} else {
		text = ciphertext
	}
	return uncompressChunk(text)
}

func (cm *CMgr) AddChunk(fp FP, chunk []byte) (bool, int, error) {
	var ciphertext []byte
	var err error
	if cm.FindChunk(fp) {
		return true, 0, nil
	}
	ciphertext, err = compressChunk(chunk, cm.compress)
	if err != nil {
		return false, 0, err
	}
	if cm.key != nil {
		ciphertext, err = encryptBytes(cm.key, ciphertext)
		if err != nil {
			return false, 0, err
		}
	}
	name := FPtoName(fp)
	dir := path.Join(cm.dir, name[:DIR_PREFIX_SIZE])
	f := path.Join(dir, name)
	tf := f + TEMP_FILE_SUFFIX
	os.MkdirAll(dir, DEFAULT_DIR_PERM)
	err = ioutil.WriteFile(tf, ciphertext, DEFAULT_FILE_PERM)
	compressedLen := len(ciphertext)
	if err == nil {
		err = os.Rename(tf, f)
	}
	if err != nil {
		os.Remove(tf)
		return false, 0, err
	}
	return false, compressedLen, nil
}

func (cm *CMgr) DeleteChunk(fp FP) bool {
	name := FPtoName(fp)
	p := path.Join(cm.dir, name[:DIR_PREFIX_SIZE], name)
	return os.Remove(p) == nil
}

func (cm *CMgr) getSubChunks(prefix string, items map[FP]int) error {
	files, err := ioutil.ReadDir(path.Join(cm.dir, prefix))
	if err != nil {
		return err
	}
	for _, f := range files {
		name := f.Name()
		if f.Mode().IsRegular() && prefix == name[:DIR_PREFIX_SIZE] {
			if fp, err := nameToFP(name); err == nil {
				items[fp] = 0
			}
		}
	}
	return nil
}

func (cm *CMgr) GetAllChunks() map[FP]int {
	items := make(map[FP]int)
	files, err := ioutil.ReadDir(cm.dir)
	if err != nil {
		return items // TODO error
	}
	for _, f := range files {
		if f.IsDir() && len(f.Name()) == DIR_PREFIX_SIZE {
			cm.getSubChunks(f.Name(), items) // TODO error
		}
	}
	return items
}

func prefixAndCompress(d []byte) ([]byte, error) {
	var zlibBuf bytes.Buffer
	zlibBuf.WriteByte(byte(CompressionType_ZLIB))
	zlw := zlib.NewWriter(&zlibBuf)
	if n, err := zlw.Write(d); err != nil {
		return nil, err
	} else if n != len(d) {
		return nil, errors.New("Zlib write failed")
	}
	if zlw.Close() != nil {
		return nil, errors.New("Zlib close failed")
	}
	return zlibBuf.Bytes(), nil
}

func prefixNoCompress(d []byte) ([]byte, error) {
	return append([]byte{byte(CompressionType_NO_COMPRESSION)}, d...), nil
}

const PREFIX_CHECK_SIZE = 4096

func compressChunk(text []byte, m CompressionMode) ([]byte, error) {
	if m == CompressionMode_AUTO {
		if len(text) < 128 {
			m = CompressionMode_NO
		} else if len(text) < PREFIX_CHECK_SIZE {
			out, err := prefixAndCompress(text)
			if err != nil {
				return nil, err
			}
			if len(out) <= len(text) {
				return out, nil
			}
			m = CompressionMode_NO
		} else {
			test := text[:PREFIX_CHECK_SIZE]
			out, err := prefixAndCompress(test)
			if err != nil {
				return nil, err
			}
			if len(out) <= len(test) {
				m = CompressionMode_SLOW
			} else {
				m = CompressionMode_NO
			}
		}
	}
	if m == CompressionMode_SLOW {
		out, err := prefixAndCompress(text)
		if err != nil {
			return nil, err
		}
		if len(out) <= len(text) {
			return out, nil
		}
		m = CompressionMode_NO
	}
	if m == CompressionMode_NO {
		return prefixNoCompress(text)
	}
	return prefixAndCompress(text)
}

func uncompressChunk(zlibText []byte) ([]byte, error) {
	if zlibText[0] == 0 {
		return zlibText[1:], nil
	} else if zlibText[0] != 1 {
		return nil, errors.New("Not encrypted")
	}
	if zlr, err := zlib.NewReader(bytes.NewBuffer(zlibText[1:])); err == nil {
		defer zlr.Close()
		return ioutil.ReadAll(zlr)
	} else {
		return nil, err
	}
}
