package vecbackup

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
)

type ChunkMap map[FP]int

type CMgr struct {
	sm       StorageMgr
	dir      string
	key      *EncKey
	compress CompressionMode
	chunks   ChunkMap
	pending  map[FP]bool
	mu       sync.Mutex
	cond     *sync.Cond
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

func MakeCMgr(sm StorageMgr, repo string, key *EncKey, compress CompressionMode) (*CMgr, error) {
	cm := &CMgr{sm: sm, dir: sm.JoinPath(repo, CHUNK_DIR), key: key, compress: compress}
	cm.mu.Lock()
	cm.cond = sync.NewCond(&cm.mu)
	cm.chunks = make(map[FP]int)
	cm.pending = make(map[FP]bool)
	err := cm.sm.LsDir2(cm.dir, func(d, f string) {
		if d == f[:DIR_PREFIX_SIZE] {
			if fp, err := nameToFP(f); err == nil {
				cm.chunks[fp] = 0
			}
		}
	})
	cm.mu.Unlock()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return cm, nil
}

func (cm *CMgr) FindChunk(fp FP) bool {
	cm.mu.Lock()
	_, ok := cm.chunks[fp]
	cm.mu.Unlock()
	return ok
}

type readChunkMem struct {
	readBuf bytes.Buffer // for read
	errBuf  bytes.Buffer // for err
	compBuf bytes.Buffer // for compression
	encBuf  []byte       // for decryption
}

func (cm *CMgr) ReadChunk(fp FP, mem *readChunkMem) ([]byte, error) {
	name := FPtoName(fp)
	f := cm.sm.JoinPath(cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE]), name)
	ciphertext, err := cm.sm.ReadFile(f, &mem.readBuf, &mem.errBuf)
	if err != nil {
		return nil, err
	}
	var text []byte
	if cm.key != nil {
		text, err = decryptBytes(cm.key, ciphertext, mem.encBuf)
		if err != nil {
			return nil, err
		}
		mem.encBuf = text
	} else {
		text = ciphertext
	}
	return uncompressChunk(text, &mem.compBuf)
}

type addChunkMem struct {
	chunkSize    int
	prefixAndBuf []byte       // 1 byte prefix plus up to chunkSize bytes
	size         int          // current data is in prefixAndBuf[1:1+size]
	compBuf      bytes.Buffer // for compression
	encBuf       []byte       // for encryption
}

func makeAddChunkMem(chunkSize int) *addChunkMem {
	return &addChunkMem{chunkSize: chunkSize, prefixAndBuf: make([]byte, 1+chunkSize), size: chunkSize}
}

func (mem *addChunkMem) setSize(size int) {
	if size > mem.chunkSize {
		panic("out of bounds")
	}
	mem.size = size
}

func (mem *addChunkMem) buf() []byte {
	return mem.prefixAndBuf[1 : 1+mem.size]
}

func (mem *addChunkMem) bufWithPrefix() []byte {
	return mem.prefixAndBuf[:1+mem.size]
}

func (mem *addChunkMem) setPrefix(p byte) {
	mem.prefixAndBuf[0] = p
}

func (cm *CMgr) AddChunk(fp FP, mem *addChunkMem) (bool, int, error) {
	cm.mu.Lock()
	for {
		_, ok := cm.chunks[fp]
		if ok {
			cm.mu.Unlock()
			return true, 0, nil
		}
		_, isPending := cm.pending[fp]
		if isPending {
			cm.cond.Wait()
		} else {
			cm.pending[fp] = true
			break
		}
	}
	cm.mu.Unlock()
	var ciphertext []byte
	var err error
	ciphertext, err = compressChunk(mem, cm.compress)
	if err != nil {
		return false, 0, err
	}
	if cm.key != nil {
		ciphertext, err = encryptBytes(cm.key, ciphertext, mem.encBuf)
		if err != nil {
			return false, 0, err
		}
	}
	mem.encBuf = ciphertext
	name := FPtoName(fp)
	dir := cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE])
	f := cm.sm.JoinPath(dir, name)
	if err = cm.sm.MkdirAll(dir); err != nil {
		return false, 0, err
	}
	err = cm.sm.WriteFile(f, ciphertext)
	if err != nil {
		return false, 0, err
	}
	compressedLen := len(ciphertext)
	cm.mu.Lock()
	cm.chunks[fp] = 0
	delete(cm.pending, fp)
	cm.cond.Broadcast()
	cm.mu.Unlock()
	return false, compressedLen, nil
}

func (cm *CMgr) DeleteChunk(fp FP) error {
	name := FPtoName(fp)
	p := cm.sm.JoinPath(cm.sm.JoinPath(cm.dir, name[:DIR_PREFIX_SIZE]), name)
	err := cm.sm.DeleteFile(p)
	if err == nil {
		cm.mu.Lock()
		delete(cm.chunks, fp)
		cm.mu.Unlock()
	}
	return err
}

func (cm *CMgr) GetAllChunks() map[FP]int {
	cm.mu.Lock()
	m := make(map[FP]int)
	for k, v := range cm.chunks {
		m[k] = v
	}
	cm.mu.Unlock()
	return m
}

func prefixAndCompress(d []byte, zlibBuf *bytes.Buffer) ([]byte, error) {
	zlibBuf.Reset()
	zlibBuf.WriteByte(byte(CompressionType_ZLIB))
	zlw := zlib.NewWriter(zlibBuf)
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

const PREFIX_CHECK_SIZE = 4096

func compressChunk(mem *addChunkMem, m CompressionMode) ([]byte, error) {
	text := mem.buf()
	if m == CompressionMode_AUTO {
		if len(text) < 128 {
			m = CompressionMode_NO
		} else if len(text) < PREFIX_CHECK_SIZE {
			out, err := prefixAndCompress(text, &mem.compBuf)
			if err != nil {
				return nil, err
			}
			if len(out) <= len(text) {
				return out, nil
			}
			m = CompressionMode_NO
		} else {
			test := text[:PREFIX_CHECK_SIZE]
			out, err := prefixAndCompress(test, &mem.compBuf)
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
		out, err := prefixAndCompress(text, &mem.compBuf)
		if err != nil {
			return nil, err
		}
		if len(out) <= len(text) {
			return out, nil
		}
		m = CompressionMode_NO
	}
	if m == CompressionMode_NO {
		mem.setPrefix(byte(CompressionType_NO_COMPRESSION))
		return mem.bufWithPrefix(), nil
	}
	return prefixAndCompress(text, &mem.compBuf)
}

func uncompressChunk(zlibText []byte, buf *bytes.Buffer) ([]byte, error) {
	if zlibText[0] == byte(CompressionType_NO_COMPRESSION) {
		return zlibText[1:], nil
	} else if zlibText[0] != byte(CompressionType_ZLIB) {
		return nil, errors.New("Not encrypted")
	}
	if zlr, err := zlib.NewReader(bytes.NewBuffer(zlibText[1:])); err == nil {
		defer zlr.Close()
		buf.Reset()
		_, err := buf.ReadFrom(zlr)
		return buf.Bytes(), err
	} else {
		return nil, err
	}
}
