package vecbackup

import "bytes"
import "google.golang.org/protobuf/types/known/timestamppb"
import "io"
import "testing"
import "time"

func TestVersionFileEnc(t *testing.T) {
	x1 := NodeDataProto{Name: "hello", Type: FileType_REGULAR_FILE, Size: 83032948, ModTime: timestamppb.New(time.Now()), Perm: 0755, FileChecksum: []byte("fsfasdfasdfsa"), Chunks: [][]byte{[]byte("fsfd"), []byte("fsfwef"), []byte("cscs")}, Sizes: []int32{234, 45, 5253, 22352}}
	x2 := NodeDataProto{Name: "world!!!", Type: FileType_DIRECTORY, Perm: 0644}
	x3 := NodeDataProto{Name: "93hoflds0230&^#", Type: FileType_SYMLINK, Target: "osfonscoasdijjfsa"}
	var buf bytes.Buffer
	l := []*NodeDataProto{&x1, &x2, &x3, &x1, &x1, &x2, &x3, &x2}
	nw, err := EncodeVersionFile(&buf)
	if err != nil {
		t.Fatal("Failed to encode version file", err)
	}
	for _, nd := range l {
		if err := EncodeOneNodeData(nd, nw); err != nil {
			t.Fatal("Failed to encode node data", err)
		}
	}
	if err = nw.Close(); err != nil {
		t.Fatal("Failed to close version file writer", err)
	}
	br, err := DecodeVersionFile(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal("Failed reading header", err)
	}
	for {
		nd, err := ReadNodeDataProto(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("Failed reading node data", err)
		}
		t.Log(nd)
	}
}
