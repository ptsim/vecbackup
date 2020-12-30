package vecbackup

import (
	"bytes"
	"crypto/sha512"
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
	removeAll(t, REPO)
	defer removeAll(t, REPO)
	sm, repo2 := GetStorageMgr(REPO)
	cm, err := MakeCMgr(sm, repo2, key, CompressionMode_YES)
	if err != nil {
		t.Fatalf("Can't create chunk manager: %s", err)
	}
	N := 100000
	data := make([]byte, N)
	rand.Seed(1)
	rand.Read(data)
	var names []FP
	for l := 0; l < N; l = 2*l + 1 {
		text := data[:]
		var fp FP = sha512.Sum512_256(text)
		_, _, err := cm.AddChunk(fp, text)
		if err != nil {
			t.Fatalf("AddChunk failed: %s %s", fp, err)
		}
		t.Logf("Added chunk %s\n", fp)
		names = append(names, fp)
	}
	for _, fp := range names {
		b, err := cm.ReadChunk(fp)
		if err != nil {
			t.Fatalf("ReadChunk failed: %s %s", fp, err)
		}
		n := len(b)
		if bytes.Compare(b, data[:n]) != 0 {
			t.Fatalf("Chunk %d is wrong\n", n)
		}
	}
}
