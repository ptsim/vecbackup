package vecbackup

import (
	"bytes"
	"crypto/sha512"
	"io/ioutil"
	"math/rand"
	"testing"
)

func TestCMNoEnc(t *testing.T) {
	testCMhelper(t, nil)
}

func TestCMEnc(t *testing.T) {
	var key EncKey = sha512.Sum512_256([]byte("f0839nskjdncw98ehjflsahflas"))
	testCMhelper(t, &key)
}

func testCMhelper(t *testing.T, key *EncKey) {
	repo, err := ioutil.TempDir("", "chunk_mgr_test-*")
	if err != nil {
		t.Fatal("Cannot get tempdir", err)
	}
	removeAll(t, repo)
	defer removeAll(t, repo)
	sm, repo2 := GetStorageMgr(repo)
	cm := MakeCMgr(sm, repo2, key, CompressionMode_YES)
	N := 100000
	mem := makeAddChunkMem(N)
	data := mem.buf()
	rand.Seed(1)
	rand.Read(data)
	var names []FP
	for l := 0; l < N; l = 2*l + 1 {
		text := data[:]
		var fp FP = sha512.Sum512_256(text)
		mem.setSize(len(text))
		added, _, err := cm.AddChunk(fp, mem)
		if err != nil {
			t.Fatalf("AddChunk failed: %s %s", fp, err)
		}
		t.Logf("Added chunk %s added %v\n", fp, added)
		names = append(names, fp)
	}
	mem2 := &readChunkMem{}
	for _, fp := range names {
		b, err := cm.ReadChunk(fp, mem2)
		if err != nil {
			t.Fatalf("ReadChunk failed: %s %s", fp, err)
		}
		n := len(b)
		if bytes.Compare(b, data[:n]) != 0 {
			t.Fatalf("Chunk %d is wrong\n", n)
		}
	}
}
