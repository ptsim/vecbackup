package main

import (
	"encoding/base32"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strconv"
)

const CS_LEN = 56

func MakeChunkName(cs CS, size int) string {
	return base32.HexEncoding.EncodeToString(cs) + fmt.Sprintf("%08x", size)
}

func DecodeChunkName(s string) (CS, int, bool) {
	cs, err := base32.HexEncoding.DecodeString(s[:CS_LEN])
	if err != nil {
		return cs, 0, false
	}
	size, err := strconv.ParseInt(s[CS_LEN:], 16, 32)
	if err != nil {
		return cs, 0, false
	}
	return cs, int(size), true
}

//---------------------------------------------------------------------------

type ChunkMgr interface {
	AddChunk(name string, buf []byte) error
	ReadChunk(name string, buf []byte) error
	FindChunk(name string) bool
	DeleteChunk(name string) bool
	GetAllChunks() map[string]int
}

func MakeChunkMgr(bkDir string, key *[32]byte) ChunkMgr {
	return &CMgr{dir: path.Join(bkDir, CHUNK_DIR), key: key}
}

//---------------------------------------------------------------------------

type CMgr struct {
	dir string
	key *[32]byte
}

func computeChunkDirPrefix(name string) string {
	if len(name) < 3 {
		return ""
	}
	return path.Join(name[:1], name[1:2], name[2:3])
}

func (cm *CMgr) FindChunk(name string) bool {
	fp := path.Join(cm.dir, computeChunkDirPrefix(name), name)
	_, err := os.Stat(fp)
	return err == nil
}

func (cm *CMgr) ReadChunk(name string, buffer []byte) error {
	fp := path.Join(cm.dir, computeChunkDirPrefix(name), name)
	ciphertext, err := ioutil.ReadFile(fp)
	if err != nil {
		return err
	}
	var text []byte
	if cm.key == nil {
		text, err = gunzipBytes(ciphertext)
	} else {
		text, err = decGunzipBytes(cm.key, ciphertext)
	}
	if err != nil {
		return err
	}
	if len(text) < len(buffer) {
		return errors.New("Chunk too small")
	} else if len(text) > len(buffer) {
		return errors.New("Chunk too big")
	}
	copy(buffer, text)
	return nil
}

func writeChunkToFile(p string, k *[32]byte, chunk []byte) error {
	var ciphertext []byte
	var err error = nil
	if k == nil {
		ciphertext = gzipBytes(chunk)
	} else {
		ciphertext, err = encGzipBytes(k, chunk)
	}
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, ciphertext, 0444)
}

func (cm *CMgr) AddChunk(name string, chunk []byte) error {
	size := len(chunk)
	if cm.FindChunk(name) {
		return nil
	}
	debugP("Adding chunk %s, len %d\n", name, size)
	dir := path.Join(cm.dir, computeChunkDirPrefix(name))
	fp := path.Join(dir, name)
	tfp := fp + TEMP_FILE_SUFFIX
	os.MkdirAll(dir, DEFAULT_DIR_PERM)
	err := writeChunkToFile(tfp, cm.key, chunk)
	if err == nil {
		err = os.Rename(tfp, fp)
	}
	if err != nil {
		os.Remove(tfp)
		return err
	}
	return nil
}

func (cm *CMgr) DeleteChunk(name string) bool {
	p := path.Join(cm.dir, computeChunkDirPrefix(name), name)
	return os.Remove(p) == nil
}

func (cm *CMgr) getChunks(sub string, depth int, items map[string]int) error {
	p := path.Join(cm.dir, sub)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return err
	}
	for _, f := range files {
		if depth < CHUNK_DIR_DEPTH {
			if f.IsDir() && len(f.Name()) == 1 {
				cm.getChunks(path.Join(sub, f.Name()), depth+1, items)
			}
		} else if depth == CHUNK_DIR_DEPTH {
			if !f.Mode().IsRegular() {
				continue
			}
			name := f.Name()
			if sub != computeChunkDirPrefix(name) {
				continue
			}
			_, size, ok := DecodeChunkName(name)
			if !ok {
				continue
			}
			if size > math.MaxInt32 {
				return errors.New(fmt.Sprintf("Chunk %s is too big: %v\n", f.Name(), f.Size()))
			}
			items[name] = 0
		}
	}
	return nil
}

func (cm *CMgr) GetAllChunks() map[string]int {
	counts := make(map[string]int)
	cm.getChunks("", 0, counts)
	return counts
}
