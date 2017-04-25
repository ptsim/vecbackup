package main

import (
	"bytes"
	"crypto/sha512"
	"math/rand"
	"testing"
)

func makeName(b []byte) string {
	cs := sha512.Sum512_256(b)
	return MakeChunkName(cs[:], len(b))
}

func TestCM(t *testing.T) {
	removeAll(t, BKDIR)
	defer removeAll(t, BKDIR)
	cm := MakeChunkMgr(BKDIR, nil)
	buf := &Buf{}
	N := 10000
	data := make([]byte, N)
	rand.Seed(1)
	rand.Read(data)
	names := make([]string, N)
	for i := 0; i < N; i++ {
		b := buf.SetSize(i)
		copy(b, data[:i])
		name := makeName(b)
		err := cm.AddChunk(name, buf.B())
		if err != nil {
			t.Fatalf("AddChunk failed: %v %s", name, err)
		}
		names[i] = name
	}
	for i := 0; i < N; i++ {
		name := names[i]
		_, size, _ := DecodeChunkName(name)
		buf.SetSize(size)
		err := cm.ReadChunk(name, buf.B())
		if err != nil {
			t.Fatalf("ReadChunk failed: %v %s", name, err)
		}
		b := buf.B()
		n := len(b)
		if bytes.Compare(b, data[:n]) != 0 {
			t.Fatalf("Chunk %d is wrong\n", n)
		}
	}
}
