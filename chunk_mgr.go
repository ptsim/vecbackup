package main

import (
	"compress/gzip"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
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

func MakeChunkMgr(bkDir string, key []byte) ChunkMgr {
	return &CMgr{dir: path.Join(bkDir, CHUNK_DIR), k: key}
}

//---------------------------------------------------------------------------

type CMgr struct {
	dir string
	k   []byte
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
	in, err := os.Open(fp)
	defer in.Close()
	if err != nil {
		return err
	}
	var in2 io.Reader = in
	if cm.k != nil {
		ds, err := makeDecryptionStream(cm.k, in)
		if err != nil {
			return err
		}
		in2 = ds
	}
	gz, err := gzip.NewReader(in2)
	if err != nil {
		return err
	}
	defer gz.Close()
	n, err := io.ReadFull(gz, buffer)
	if n != len(buffer) {
		return errors.New("Chunk too small")
	}
	if err == io.EOF {
		err = nil
	}
	return err
}

func writeChunkToFile(p string, k []byte, chunk []byte) error {
	out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0444)
	if err != nil {
		return errors.New(fmt.Sprintf("Can't write chunk file %s: %s", p, err))
	}
	defer out.Close()
	var out2 io.Writer = out
	if k != nil {
		es, err := makeEncryptionStream(k, out)
		if err != nil {
			return err
		}
		defer es.Close()
		out2 = es
	}
	gz := gzip.NewWriter(out2)
	defer gz.Close()
	n, err := gz.Write(chunk)
	if n < len(chunk) {
		return io.ErrShortWrite
	}
	return nil
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
	err := writeChunkToFile(tfp, cm.k, chunk)
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
