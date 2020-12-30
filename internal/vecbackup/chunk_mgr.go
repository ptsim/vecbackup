package vecbackup

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

type ChunkMap map[FP]int

type CMgr struct {
	sm       StorageMgr
	dir      string
	key      *EncKey
	compress CompressionMode
	chunks   ChunkMap
}

func MakeCMgr(sm StorageMgr, repo string, key *EncKey, compress CompressionMode) (*CMgr, error) {
	cm := &CMgr{sm: sm, dir: sm.JoinPath(repo, CHUNK_DIR), key: key, compress: compress}
	if err := cm.scanChunks(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return cm, nil
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

func (cm *CMgr) scanChunks() error {
	cm.chunks = make(map[FP]int)
	return cm.sm.LsDir2(cm.dir, func(d, f string) {
		if d == f[:DIR_PREFIX_SIZE] {
			if fp, err := nameToFP(f); err == nil {
				cm.chunks[fp] = 0
			}
		}
	})
}

func (cm *CMgr) FindChunk(fp FP) bool {
	_, ok := cm.chunks[fp]
	return ok
}

func (cm *CMgr) ReadChunk(fp FP) ([]byte, error) {
	name := FPtoName(fp)
	f := cm.sm.JoinPath(cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE]), name)
	ciphertext, err := cm.sm.ReadFile(f)
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
	dir := cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE])
	f := cm.sm.JoinPath(dir, name)
	if err = cm.sm.MkdirAll(dir); err != nil {
		return false, 0, err
	}
	err = cm.sm.WriteFile(f, ciphertext)
	compressedLen := len(ciphertext)
	cm.chunks[fp] = 0
	return false, compressedLen, nil
}

func (cm *CMgr) DeleteChunk(fp FP) error {
	name := FPtoName(fp)
	p := cm.sm.JoinPath(cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE]), name)
	err := cm.sm.DeleteFile(p)
	if err == nil {
		delete(cm.chunks, fp)
	}
	return err
}

func (cm *CMgr) GetAllChunks() map[FP]int {
	m := make(map[FP]int)
	for k, v := range cm.chunks {
		m[k] = v
	}
	return m
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
