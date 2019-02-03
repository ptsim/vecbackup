package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Header struct {
	Magic string
}

type FileData struct {
	Name         string
	Type         FileType
	Size         int64
	ModTime      time.Time
	Perm         os.FileMode
	Target       string
	Chunks       []string
	FileChecksum CS
}
type FileType int
type CS []byte

const (
	VERSION_MAGIC          = "PTVBKVSN"
	REGULAR_FILE  FileType = iota
	DIRECTORY
	SYMLINK
)

//---------------------------------------------------------------------------

func MakeVersionString(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339Nano)
}

func MakeVersionFileName(ts time.Time) string {
	return VERSION_FILENAME_PREFIX + MakeVersionString(ts)
}

func MakeVersionFileNameFromString(version string) string {
	return VERSION_FILENAME_PREFIX + version
}

func DecodeVersionFileName(fn string) (version string, ok bool) {
	if !strings.HasPrefix(fn, VERSION_FILENAME_PREFIX) {
		return "", false
	}
	ts := fn[len(VERSION_FILENAME_PREFIX):]
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil || t.Location() != time.UTC {
		return "", false
	}
	return ts, true
}

func fixDirPerm(perm os.FileMode) os.FileMode {
	return perm | 0700
}

func NewRegularFile(name string, size int64, modTime time.Time, perm os.FileMode, chunks []string) *FileData {
	return &FileData{Name: name, Type: REGULAR_FILE, Size: size, ModTime: modTime, Perm: perm, Chunks: chunks}
}

func NewDirectory(name string, perm os.FileMode) *FileData {
	return &FileData{Name: name, Type: DIRECTORY, Perm: fixDirPerm(perm)}
}

func NewSymlink(name, target string) *FileData {
	return &FileData{Name: name, Type: SYMLINK, Target: target}
}

func (fd *FileData) IsDir() bool {
	return fd.Type == DIRECTORY
}

func (fd *FileData) IsFile() bool {
	return fd.Type == REGULAR_FILE
}

func (fd *FileData) IsSymlink() bool {
	return fd.Type == SYMLINK
}

func (fd *FileData) IsValid() bool {
	if fd.IsDir() {
		return len(fd.Name) > 0
	}
	if fd.IsSymlink() {
		return len(fd.Name) > 0 && len(fd.Target) > 0
	}
	if fd.Size > 0 {
		return len(fd.Name) > 0 && !fd.ModTime.IsZero() && fd.Chunks != nil
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

type VersionMgr interface {
	GetVersion(version string) (string, error)
	GetVersions() ([]string, error)
	DeleteVersion(version string) error
	LoadVersion(v string) (FReader, error)
	SaveVersion(version time.Time) (FWriter, error)
}

type FWriter interface {
	Write(*FileData) error
	Close() error
	Abort()
}

type FReader interface {
	Next() (*FileData, error)
	Close() error
}

//---------------------------------------------------------------------------

type VMgr struct {
	dir string
	key *[32]byte
}

func MakeVersionMgr(bkDir string, key *[32]byte) VersionMgr {
	return &VMgr{dir: bkDir, key: key}
}

func (vm *VMgr) GetVersion(version string) (string, error) {
	if version != "latest" {
		return version, nil
	}
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
	fn := MakeVersionFileNameFromString(v)
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

func (vm *VMgr) LoadVersion(v string) (FReader, error) {
	fp := path.Join(vm.dir, VERSIONS_DIR, MakeVersionFileNameFromString(v))
	ciphertext, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	var text []byte
	if vm.key == nil {
		text, err = gunzipBytes(ciphertext)
	} else {
		text, err = decGunzipBytes(vm.key, ciphertext)
	}
	buf := bytes.NewBuffer(text)
	dec := gob.NewDecoder(buf)
	h := &Header{}
	dec.Decode(h)
	if h.Magic != VERSION_MAGIC {
		return nil, errors.New("Invalid version header: missing magic string")
	}
	return &VFReader{fp: fp, dec: dec, last: ""}, nil
}

type VFReader struct {
	fp   string
	dec  *gob.Decoder
	last string
}

func (vfr *VFReader) Next() (*FileData, error) {
	fd := &FileData{}
	err := vfr.dec.Decode(fd)
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to decode version file: %s", vfr.fp))
	}
	if !fd.IsValid() {
		debugP("Invalid file data: %v\n", fd)
		return nil, errors.New(fmt.Sprintf("Invalid file data: %s", vfr.fp))
	}
	fd.Name = filepath.FromSlash(fd.Name)
	if pathCompare(fd.Name, vfr.last) < 0 {
		debugP("Version file is not sorted: %s %s\n", vfr.last, fd.Name)
		return nil, errors.New(fmt.Sprintf("Version file is not sorted: %s", vfr.fp))
	}
	vfr.last = fd.Name
	return fd, nil
}

func (vfr *VFReader) Close() error {
	return nil
}

func (vm *VMgr) SaveVersion(version time.Time) (FWriter, error) {
	fp := path.Join(vm.dir, VERSIONS_DIR, MakeVersionFileName(version))
	vfw := &VFWriter{fp: fp, key: vm.key, last: ""}
	h := &Header{VERSION_MAGIC}
	vfw.enc = gob.NewEncoder(&vfw.buf)
	err := vfw.enc.Encode(h)
	if err != nil {
		return nil, err
	}
	return vfw, nil
}

type VFWriter struct {
	fp   string
	key  *[32]byte
	buf  bytes.Buffer
	enc  *gob.Encoder
	last string
}

func (vfw *VFWriter) Write(fd *FileData) error {
	if !fd.IsValid() {
		debugP("Writing invalid file data: %v\n", fd)
		return errors.New(fmt.Sprintf("Writing invalid file data: %s", fd.Name))
	}
	if pathCompare(fd.Name, vfw.last) < 0 {
		debugP("File list is not sorted: %s %s\n", vfw.last, fd.Name)
		return errors.New(fmt.Sprintf("File list is not sorted: %s", vfw.fp))
	}
	vfw.last = fd.Name
	fd2 := *fd
	fd2.Name = filepath.ToSlash(fd2.Name)
	err := vfw.enc.Encode(fd)
	if err != nil {
		return err
	}
	return nil
}

func (vfw *VFWriter) Close() error {
	var ciphertext []byte
	var err error = nil
	if vfw.key == nil {
		ciphertext = gzipBytes(vfw.buf.Bytes())
	} else {
		ciphertext, err = encGzipBytes(vfw.key, vfw.buf.Bytes())
	}
	if err != nil {
		return err
	}
	tfp := vfw.fp + TEMP_FILE_SUFFIX
	err = ioutil.WriteFile(tfp, ciphertext, 0666)
	if err != nil {
		return err
	}
	err = os.Rename(tfp, vfw.fp)
	if err != nil {
		os.Remove(tfp)
	}
	return err
}

func (vfw *VFWriter) Abort() {
}

func ReduceVersions(cur time.Time, versions []string) []string {
	m := make(map[time.Time]bool)
	d := make([]string, 0)
	for _, v := range versions {
		ts, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			panic(fmt.Sprintf("Bad version: %v", v))
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
