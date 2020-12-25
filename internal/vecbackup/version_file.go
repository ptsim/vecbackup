package vecbackup

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FP [32]byte

func (fp FP) String() string {
	return fmt.Sprintf("%x", []byte(fp[:]))
}

type FileData struct {
	Name         string
	Type         FileType
	Size         int64
	ModTime      time.Time
	Perm         os.FileMode
	FileChecksum []byte
	Target       string
	Sizes        []int32
	Chunks       []FP
}

//---------------------------------------------------------------------------
const RFC3339NanoFull = "2006-01-02T15:04:05.000000000Z07:00"

func DecodeVersionTime(v string) (time.Time, bool) {
	t, err := time.Parse(RFC3339NanoFull, v)
	return t, err == nil && t.Location() == time.UTC
}

func CreateNewVersion(last string) string {
	for {
		t := time.Now()
		new_version := t.UTC().Format(RFC3339NanoFull)
		if new_version > last {
			return new_version
		}
		time.Sleep(10 * time.Nanosecond)
	}
}

func DecodeVersionFileName(fn string) (version string, ok bool) {
	if !strings.HasPrefix(fn, VERSION_FILENAME_PREFIX) {
		return "", false
	}
	v := fn[len(VERSION_FILENAME_PREFIX):]
	if _, ok := DecodeVersionTime(v); !ok {
		return "", false
	}
	return v, true
}

func NewRegularFile(name string, size int64, modTime time.Time, perm os.FileMode, fileChecksum []byte, chunks []FP, sizes []int32) *FileData {
	return &FileData{Name: name, Type: FileType_REGULAR_FILE, Size: size, ModTime: modTime, Perm: perm, FileChecksum: fileChecksum, Chunks: chunks, Sizes: sizes}
}

func NewDirectory(name string, perm os.FileMode) *FileData {
	return &FileData{Name: name, Type: FileType_DIRECTORY, Perm: perm}
}

func NewSymlink(name, target string) *FileData {
	return &FileData{Name: name, Type: FileType_SYMLINK, Target: target}
}

func (fd *FileData) IsDir() bool {
	return fd.Type == FileType_DIRECTORY
}

func (fd *FileData) IsFile() bool {
	return fd.Type == FileType_REGULAR_FILE
}

func (fd *FileData) IsSymlink() bool {
	return fd.Type == FileType_SYMLINK
}

func (fd *FileData) IsValid() bool {
	if !fd.IsDir() && !fd.IsSymlink() && !fd.IsFile() {
		return false
	}
	if fd.IsDir() {
		return len(fd.Name) > 0
	}
	if fd.IsSymlink() {
		return len(fd.Name) > 0 && len(fd.Target) > 0
	}
	if fd.Size > 0 {
		return len(fd.Name) > 0 && !fd.ModTime.IsZero() && fd.Chunks != nil && len(fd.Chunks) == len(fd.Sizes)
	} else {
		return len(fd.Name) > 0 && !fd.ModTime.IsZero()
	}
}

func (fd *FileData) PrettyPrint() string {
	if fd.IsDir() {
		return fd.Name + PATH_SEP
	} else if fd.IsSymlink() {
		return fd.Name + "@"
	}
	return fd.Name
}

//---------------------------------------------------------------------------

type VMgr struct {
	dir string
	key *EncKey
}

func MakeVMgr(repo string, key *EncKey) *VMgr {
	return &VMgr{dir: repo, key: key}
}

func (vm *VMgr) GetLatestVersion() (string, error) {
	versions, err := vm.GetVersions()
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", nil
	}
	return versions[len(versions)-1], nil
}

func (vm *VMgr) GetVersions() ([]string, error) {
	md := path.Join(vm.dir, VERSIONS_DIR)
	files, err := ioutil.ReadDir(md)
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, f := range files {
		n := f.Name()
		if f.Mode().IsRegular() {
			version, ok := DecodeVersionFileName(n)
			if ok {
				versions = append(versions, version)
			}
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func (vm *VMgr) DeleteVersion(v string) error {
	fn := VERSION_FILENAME_PREFIX + v
	p := path.Join(vm.dir, VERSIONS_DIR, fn)
	if nodeExists(p) {
		if err := os.Remove(p); err != nil {
			return err
		}
	} else {
		return errors.New("Does not exist")
	}
	return nil
}

func ConvertFromNodeDataProto(nd *NodeDataProto) (*FileData, error) {
	if nd.Type == FileType_REGULAR_FILE {
		chunks := make([]FP, len(nd.Chunks))
		for i, b := range nd.Chunks {
			var fp FP
			if len(b) != len(fp) {
				return nil, errors.New("Bad fingerprint")
			}
			copy(chunks[i][:], b)
		}
		return NewRegularFile(filepath.FromSlash(nd.Name), nd.Size, nd.ModTime.AsTime(), os.FileMode(nd.Perm), nd.FileChecksum, chunks, nd.Sizes), nil
	} else if nd.Type == FileType_DIRECTORY {
		return NewDirectory(filepath.FromSlash(nd.Name), os.FileMode(nd.Perm)), nil
	} else if nd.Type == FileType_SYMLINK {
		return NewSymlink(filepath.FromSlash(nd.Name), nd.Target), nil
	}
	return nil, errors.New("Invalid type")
}

func ConvertToNodeDataProto(fd *FileData) *NodeDataProto {
	if !fd.IsValid() {
		return nil
	}
	if fd.IsFile() {
		chunks := make([][]byte, len(fd.Chunks))
		for i, b := range fd.Chunks {
			c := make([]byte, len(b))
			copy(c, b[:])
			chunks[i] = c
		}
		return &NodeDataProto{Name: filepath.ToSlash(fd.Name), Type: fd.Type, Size: fd.Size, Perm: int32(fd.Perm), FileChecksum: fd.FileChecksum, ModTime: timestamppb.New(fd.ModTime), Sizes: fd.Sizes, Chunks: chunks}
	} else if fd.IsDir() {
		return &NodeDataProto{Name: filepath.ToSlash(fd.Name), Type: fd.Type, Perm: int32(fd.Perm)}
	} else if fd.IsSymlink() {
		return &NodeDataProto{Name: filepath.ToSlash(fd.Name), Type: fd.Type, Target: fd.Target}
	}
	return nil
}

func EncodeVersionFile(w io.Writer) (io.WriteCloser, error) {
	vp := &VersionProto{Version: VV_VERSION}
	out, err := proto.Marshal(vp)
	if err != nil {
		return nil, err
	}
	zlw := zlib.NewWriter(w)
	if _, err := zlw.Write([]byte(VV_MAGIC)); err != nil {
		return nil, err
	}
	lbuf := make([]byte, binary.MaxVarintLen64)
	lbuf = lbuf[:binary.PutUvarint(lbuf, uint64(len(out)))]
	if _, err := zlw.Write(lbuf); err != nil {
		return nil, err
	}
	if _, err := zlw.Write(out); err != nil {
		return nil, err
	}
	return zlw, nil
}

func EncodeOneNodeData(vfd *NodeDataProto, w io.Writer) error {
	out, err := proto.Marshal(vfd)
	if err != nil {
		return err
	}
	lbuf := make([]byte, binary.MaxVarintLen64)
	lbuf = lbuf[:binary.PutUvarint(lbuf, uint64(len(out)))]
	if _, err := w.Write(lbuf); err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	return nil
}

func DecodeVersionFile(r io.Reader) (*bufio.Reader, error) {
	zlr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	var h [len(VV_MAGIC)]byte
	if _, err := io.ReadFull(zlr, h[:]); err != nil || bytes.Compare(h[:], []byte(VV_MAGIC)) != 0 {
		return nil, errors.New("Invalid version file.")
	}
	br := bufio.NewReader(zlr)
	n, err := binary.ReadUvarint(br)
	if n > math.MaxInt32 {
		return nil, errors.New("Invalid version file.")
	}
	b := make([]byte, int(n))
	_, err = io.ReadFull(br, b)
	if err != nil {
		return nil, err
	}
	m := &VersionProto{}
	if err := proto.Unmarshal(b, m); err != nil {
		return nil, err
	}
	if m.Version != VV_VERSION {
		return nil, errors.New("Incompatible version file.")
	}
	return br, nil
}

func ReadNodeDataProto(r *bufio.Reader) (*NodeDataProto, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	} else if n > math.MaxInt32 {
		return nil, errors.New("Invalid version file.")
	}
	b := make([]byte, int(n))
	if _, err = io.ReadFull(r, b); err != nil {
		return nil, err
	}
	m := &NodeDataProto{}
	if err := proto.Unmarshal(b, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (vm *VMgr) LoadFiles(v string) ([]*FileData, error, int) {
	fp := path.Join(vm.dir, VERSIONS_DIR, VERSION_FILENAME_PREFIX+v)
	ciphertext, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err, 0
	}
	var text []byte
	if vm.key == nil {
		text = ciphertext
	} else {
		text, err = decryptBytes(vm.key, ciphertext)
	}
	br, err := DecodeVersionFile(bytes.NewReader(text))
	if err != nil {
		return nil, err, 0
	}
	errs := 0
	var fds []*FileData
	for {
		nd, err := ReadNodeDataProto(br)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err, 0
		}
		fd, err := ConvertFromNodeDataProto(nd)
		if err != nil {
			fmt.Fprintf(stderr, "F %s: Invalid data: %s", nd.Name, err)
			errs++
		} else if !fd.IsValid() {
			fmt.Fprintf(stderr, "F %s: Invalid data", nd.Name)
			errs++
		} else {
			fds = append(fds, fd)
		}
	}
	return fds, nil, errs
}

func (vm *VMgr) SaveFiles(version string, fds []*FileData) error {
	var buf bytes.Buffer
	nw, err := EncodeVersionFile(&buf)
	if err != nil {
		return err
	}
	for _, fd := range fds {
		nd := ConvertToNodeDataProto(fd)
		if nd == nil {
			return fmt.Errorf("Writing invalid file data: %s", fd.Name)
		}
		if err := EncodeOneNodeData(nd, nw); err != nil {
			return err
		}
	}
	if err = nw.Close(); err != nil {
		return err
	}
	var result []byte
	if vm.key == nil {
		result = buf.Bytes()
	} else {
		var err error
		if result, err = encryptBytes(vm.key, buf.Bytes()); err != nil {
			return err
		}
	}
	fp := path.Join(vm.dir, VERSIONS_DIR, VERSION_FILENAME_PREFIX+version)
	tfp := fp + TEMP_FILE_SUFFIX
	if err := ioutil.WriteFile(tfp, result, DEFAULT_FILE_PERM); err != nil {
		return err
	}
	if err := os.Rename(tfp, fp); err != nil {
		os.Remove(tfp)
		return err
	}
	return nil
}

func ReduceVersions(cur time.Time, versions []string) []string {
	m := make(map[time.Time]bool)
	d := make([]string, 0)
	for _, v := range versions {
		ts, ok := DecodeVersionTime(v)
		if !ok {
			continue
		}
		if ts.After(cur.Add(-time.Hour * 24)) {
			// last day, all ok
		} else {
			var clipped time.Time
			if ts.After(cur.Add(-time.Hour * 24 * 7)) {
				// last week, one per hour
				clipped = ts.Truncate(time.Hour)
			} else if ts.After(cur.Add(-time.Hour * 24 * 30)) {
				// last month, one per day
				clipped = ts.Truncate(time.Hour * 24)
			} else if ts.After(cur.Add(-time.Hour * 24 * 365)) {
				// last year, one per week
				clipped = ts.Truncate(time.Hour * 24 * 7)
			} else {
				// otherwise, once per 30 days
				clipped = ts.Truncate(time.Hour * 24 * 30)
			}
			if !m[clipped] {
				m[clipped] = true
			} else {
				d = append(d, v)
			}
		}
	}
	return d
}
