package main

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"
	"time"
)

const (
	MAGIC                   = "PTVBK"
	BKVERSION               = 1
	DEFAULT_CHUNK_SIZE      = 16 * 1024 * 1024
	CHUNK_DIR_DEPTH         = 3
	VERSION_FILENAME_PREFIX = "vecbackup-version-"
	IGNORE_FILENAME         = "vecbackup-ignore"
	LOCK_FILENAME           = "vecbackup-lock"
	TEMP_FILE_SUFFIX        = "-temp"
	CHUNK_DIR               = "chunks"
	VERSIONS_DIR            = "versions"
	DELETED_PREFIX          = "DELETED-"
	DEFAULT_DIR_PERM        = 0777
	DEFAULT_FILE_PERM       = 0666
	PATH_SEP                = string(os.PathSeparator)
)

//---------------------------------------------------------------------------

func readIgnoreFile(src string) ([]string, error) {
	ignorePatterns := []string{}
	in, err := os.Open(path.Join(src, IGNORE_FILENAME))
	if os.IsNotExist(err) {
		return ignorePatterns, nil
	} else if err != nil {
		return nil, err
	}
	defer in.Close()
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		l := scanner.Text()
		if len(l) > 0 {
			_, err := filepath.Match(l, "zzz")
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Bad pattern in ignore file: %v", l))
			} else {
				ignorePatterns = append(ignorePatterns, l)
			}
		}
	}
	return ignorePatterns, nil
}

func toIgnore(ignorePatterns []string, dir, fn string) bool {
	for _, p := range ignorePatterns {
		var matched bool
		var err error
		if p[0] == '/' {
			rp := filepath.ToSlash(PATH_SEP + path.Join(dir, fn))
			matched, err = filepath.Match(p, rp)
		} else {
			matched, err = filepath.Match(p, fn)
		}
		if err == nil && matched {
			return true
		}
	}
	return false
}

func cleanSubpath(sub string) (string, error) {
	if sub == "" {
		return "", nil
	}
	l := strings.Split(sub, PATH_SEP)
	nl := []string{}
	for _, p := range l {
		if p == "." || p == "" {
			continue
		}
		if p == ".." {
			return "", errors.New(fmt.Sprintf("Invalid subpath: %s", sub))
		}
		nl = append(nl, p)
	}
	return path.Join(nl...), nil
}

func IsSymlink(f os.FileInfo) bool {
	return f.Mode()&os.ModeSymlink != 0
}

func pathCompare(p1, p2 string) int {
	l1 := strings.Split(p1, PATH_SEP)
	n1 := len(l1)
	l2 := strings.Split(p2, PATH_SEP)
	n2 := len(l2)
	for i := 0; ; i++ {
		if i < n1 && i < n2 {
			if r := strings.Compare(l1[i], l2[i]); r != 0 {
				return r
			}
		} else if i == n1 && i == n2 {
			return 0
		} else if n1 < n2 {
			return -1
		} else {
			return 1
		}
	}
}

type ByDirFile []string

func (s ByDirFile) Len() int {
	return len(s)
}
func (s ByDirFile) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByDirFile) Less(i, j int) bool {
	return pathCompare(s[i], s[j]) < 0
}

func nodeExists(fp string) bool {
	_, err := os.Stat(fp)
	return !os.IsNotExist(err)
}

//---------------------------------------------------------------------------

type DirReader struct {
	src    string
	ignore []string
	lfp    []string
	lfi    []os.FileInfo
}

func (dr *DirReader) Push(fp string, f os.FileInfo) {
	dr.lfp = append(dr.lfp, fp)
	dr.lfi = append(dr.lfi, f)
}

func (dr *DirReader) Pop() (fp string, f os.FileInfo) {
	n := len(dr.lfp) - 1
	fp = dr.lfp[n]
	f = dr.lfi[n]
	dr.lfp = dr.lfp[:n]
	dr.lfi = dr.lfi[:n]
	return
}

func (dr *DirReader) PushChildren(fp string) error {
	files, err := ioutil.ReadDir(path.Join(dr.src, fp))
	if err != nil {
		return err
	}
	for i := len(files) - 1; i >= 0; i-- {
		name := files[i].Name()
		if toIgnore(dr.ignore, fp, name) {
			continue
		}
		dr.Push(path.Join(fp, name), files[i])
	}
	return nil
}

func (dr *DirReader) Next() (*FileData, error) {
	for len(dr.lfp) > 0 {
		fp, f := dr.Pop()
		if f.IsDir() {
			err := dr.PushChildren(fp)
			if err != nil {
				return nil, err
			}
			return NewDirectory(fp, f.Mode().Perm()), nil
		} else if f.Mode().IsRegular() {
			return NewRegularFile(fp, f.Size(), f.ModTime(), f.Mode().Perm(), nil), nil
		} else if IsSymlink(f) {
			if target, err := os.Readlink(path.Join(dr.src, fp)); err != nil {
				return nil, err
			} else {
				return NewSymlink(fp, target), nil
			}
		}
	}
	return nil, io.EOF
}

func (dr *DirReader) Close() error {
	dr.ignore = nil
	dr.lfp = nil
	dr.lfi = nil
	return nil
}

func getSpecifiedFiles(ignore []string, src string, subpaths []string) (FReader, error) {
	if len(subpaths) == 0 {
		subpaths = []string{"."}
	}
	sort.Sort(ByDirFile(subpaths))
	dr := &DirReader{src: src, ignore: ignore}
	for i := len(subpaths) - 1; i >= 0; i-- {
		p := subpaths[i]
		p, err := cleanSubpath(p)
		if err != nil {
			return nil, err
		}
		if p != "" {
			if f, err := os.Stat(path.Join(src, p)); err != nil {
				return dr, err
			} else {
				dr.Push(p, f)
			}
		} else {
			err = dr.PushChildren(p)
			if err != nil {
				return nil, err
			}
		}
	}
	return dr, nil
}

type NullFReader struct {
}

func (nr *NullFReader) Next() (*FileData, error) {
	return nil, io.EOF
}

func (nr *NullFReader) Close() error {
	debugP("NullFReader.Close\n")
	return nil
}

type NullFWriter struct {
}

func (nw *NullFWriter) Write(*FileData) error {
	return nil
}

func (nw *NullFWriter) Close() error {
	return nil
}

func (nw *NullFWriter) Abort() {
}

type Buf struct {
	bf []byte
	sz int
}

func (b *Buf) SetSize(size int) []byte {
	if cap(b.bf) < size {
		b.bf = make([]byte, size)
	}
	b.sz = size
	return b.bf[:size]
}

func (b *Buf) B() []byte {
	return b.bf[:b.sz]
}

//---------------------------------------------------------------------------

type mergeFunc func(f1 *FileData, f2 *FileData) bool

func isError(err error) bool {
	return err != nil && err != io.EOF
}

func mergeFileDataStream(s1, s2 FReader, f mergeFunc) error {
	f1, err1 := s1.Next()
	f2, err2 := s2.Next()
	for {
		if isError(err1) {
			return err1
		}
		if isError(err2) {
			return err2
		}
		if err1 == io.EOF && err2 == io.EOF {
			return nil
		}
		if err1 == io.EOF {
			if f(nil, f2) {
				return nil
			}
			f2, err2 = s2.Next()
		} else if err2 == io.EOF {
			if f(f1, nil) {
				return nil
			}
			f1, err1 = s1.Next()
		} else if f1.Name == f2.Name {
			if f(f1, f2) {
				return nil
			}
			f1, err1 = s1.Next()
			f2, err2 = s2.Next()
		} else if pathCompare(f1.Name, f2.Name) < 0 {
			if f(f1, nil) {
				return nil
			}
			f1, err1 = s1.Next()
		} else {
			if f(nil, f2) {
				return nil
			}
			f2, err2 = s2.Next()
		}
	}
}

func backupOneFile(cm ChunkMgr, src string, cs int, dryRun, checksum, verbose bool, buf *Buf, fw FWriter, out io.Writer, old *FileData, new *FileData) error {
	if old == nil && new != nil {
		if err := addChunks(new, cm, src, cs, dryRun, buf); err != nil {
			return err
		}
		if err := fw.Write(new); err != nil {
			return err
		}
		fmt.Fprintf(out, "+ %s\n", new.PrettyPrint())
	} else if old != nil && new == nil {
		fmt.Fprintf(out, "- %s\n", old.PrettyPrint())
	} else if new.Type != old.Type || (new.IsFile() && (new.Size != old.Size || new.ModTime != old.ModTime || (checksum && !fileMatchesChecksums(old, cm, src)))) || (new.IsSymlink() && new.Target != old.Target) {
		if err := addChunks(new, cm, src, cs, dryRun, buf); err != nil {
			return err
		}
		if err := fw.Write(new); err != nil {
			return err
		}
		fmt.Fprintf(out, "+ %s\n", new.PrettyPrint())
	} else {
		if new.IsFile() && (new.Perm&0400) == 0 {
			return errors.New(fmt.Sprintf("%s: read permission required", new.PrettyPrint()))
		} else if new.IsDir() && (new.Perm&0500) == 0 {
			return errors.New(fmt.Sprintf("%s: read and execute permission required", new.PrettyPrint()))
		}
		if !old.IsSymlink() {
			old.Perm = new.Perm
		}
		if err := fw.Write(old); err != nil {
			return err
		}
		if verbose {
			fmt.Fprintf(out, "= %v\n", new.PrettyPrint())
		}
	}
	return nil
}

func fileMatchesChecksums(fd *FileData, cm ChunkMgr, src string) bool {
	p := path.Join(src, fd.Name)
	file, err := os.Open(p)
	if err != nil {
		return false
	}
	defer file.Close()
	var theBuf Buf
	for _, s := range fd.Chunks {
		cs, bs, ok := DecodeChunkName(s)
		if !ok {
			return false
		}
		if !cm.FindChunk(s) {
			return false
		}
		b := theBuf.SetSize(bs)
		count, err := io.ReadFull(file, b)
		if count != bs || err != nil {
			return false
		}
		cs2 := sha512.Sum512_256(b)
		if bytes.Compare(cs, cs2[:]) != 0 {
			return false
		}
	}
	return true
}

func addChunks(fd *FileData, cm ChunkMgr, src string, bs int, dryRun bool, buf *Buf) error {
	if fd.IsDir() || fd.IsSymlink() || dryRun {
		return nil
	}
	h := sha512.New512_256()
	var css []string = nil
	var err error
	err2 := readFileChunks(path.Join(src, fd.Name), buf, bs, func(buf2 *Buf) bool {
		b := buf2.B()
		h.Write(b)
		cs := sha512.Sum512_256(b)
		n := MakeChunkName(cs[:], len(b))
		css = append(css, n)
		err = cm.AddChunk(n, b)
		return err != nil
	})
	if err == nil {
		err = err2
	}
	if err == nil {
		fd.Chunks = css
		fd.FileChecksum = h.Sum(nil)
	}
	return err
}

func readFileChunks(p string, buf *Buf, bs int, f func(buf *Buf) bool) error {
	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	buffer := buf.SetSize(bs)
	var len int64 = 0
	for {
		count, err := io.ReadFull(file, buffer)
		if count > 0 {
			len += int64(count)
			buf.SetSize(count)
			if f(buf) {
				return nil
			}
		}
		if len > fi.Size() {
			return errors.New(fmt.Sprintf("File size changed %s", p))
		}
		if err == io.EOF || count < bs {
			if len < fi.Size() {
				return errors.New(fmt.Sprintf("File size changed %s", p))
			}
			return nil
		}
	}
}

//---------------------------------------------------------------------------

func matchRecoveryPatterns(fp string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if matchRecoveryPattern(fp, pat) {
			return true
		}
	}
	return false
}

func matchRecoveryPattern(fp, pat string) bool {
	if fp == pat {
		return true
	}
	l := len(pat)
	return l == 0 || (len(fp) > l && fp[:l] == pat && os.IsPathSeparator(fp[l]))
}

func readBackupFileChunks(fd *FileData, cm ChunkMgr, buf *Buf, f func(string, CS) bool) error {
	for _, name := range fd.Chunks {
		cs, size, ok := DecodeChunkName(name)
		if !ok {
			return errors.New(fmt.Sprintf("Invalid chunk name: %s\n", name))
		}
		buf.SetSize(size)
		err := cm.ReadChunk(name, buf.B())
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to read chunk %s: %s", name, err))
		}
		if f(name, cs) {
			return nil
		}
	}
	return nil
}

func recoverFileToTemp(fd *FileData, cm ChunkMgr, recDir string, testRun bool, buf *Buf) (string, error) {
	var f *os.File
	fn := path.Join(recDir, fd.Name) + TEMP_FILE_SUFFIX
	if !testRun {
		d := filepath.Dir(fn)
		err := os.MkdirAll(d, DEFAULT_DIR_PERM)
		f, err = os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DEFAULT_FILE_PERM)
		if err != nil {
			return "", err
		}
		defer f.Close()
	}
	var l int64
	h := sha512.New512_256()
	var err error
	if err2 := readBackupFileChunks(fd, cm, buf, func(name string, cs CS) bool {
		b := buf.B()
		h.Write(b)
		cs2 := sha512.Sum512_256(b)
		if bytes.Compare(cs, cs2[:]) != 0 {
			err = errors.New(fmt.Sprintf("Checksum mismatch for chunk %s", name))
		} else {
			l += int64(len(b))
			if !testRun {
				_, err = f.Write(b)
			}
		}
		return err != nil
	}); err2 != nil && err == nil {
		err = err2
	}
	if err != nil {
		return fn, err
	}
	if l != fd.Size {
		return fn, errors.New(fmt.Sprintf("Length mismatch: %d vs %d", l, fd.Size))
	}
	if bytes.Compare(fd.FileChecksum, h.Sum(nil)) != 0 {
		return fn, errors.New("File checksum mismatch")
	}
	return fn, nil
}

func recoverNode(fd *FileData, cm ChunkMgr, recDir string, testRun bool, merge bool, buf *Buf) error {
	fp := path.Join(recDir, fd.Name)
	if fd.IsDir() {
		if !testRun {
			if err := os.MkdirAll(fp, DEFAULT_DIR_PERM); err != nil && !os.IsExist(err) {
				return err
			}
			return os.Chmod(fp, fd.Perm)
		}
		return nil
	}
	if fd.IsSymlink() {
		if !testRun {
			return os.Symlink(fd.Target, fp)
		}
		return nil
	}
	if merge {
		if fi, err := os.Stat(fp); err == nil && fi.Size() == fd.Size && fi.ModTime() == fd.ModTime {
			return nil
		}
	}
	tfp, err := recoverFileToTemp(fd, cm, recDir, testRun, buf)
	if err == nil {
		if testRun {
			return nil
		}
		err = os.Chtimes(tfp, fd.ModTime, fd.ModTime)
		if err == nil {
			err = os.Chmod(tfp, fd.Perm)
			if err == nil {
				err = os.Rename(tfp, fp)
				if err == nil {
					return nil
				}
			}
		}
	}
	if tfp != "" {
		os.Remove(tfp)
	}
	return err
}

//---------------------------------------------------------------------------

func verifyChunks(cm ChunkMgr, buf *Buf, counts map[string]int) (numChunks, numOk, numFailed, numUnused int) {
	numChunks = len(counts)
	numOk = 0
	numFailed = 0
	numUnused = 0
	for name, count := range counts {
		cs, size, ok := DecodeChunkName(name)
		if !ok {
			debugP("FAILED: %s: Invalid Chunk name\n", name)
			numFailed++
			continue
		}
		if count == 0 {
			numUnused++
			continue
		}
		buf.SetSize(size)
		err := cm.ReadChunk(name, buf.B())
		if err != nil {
			debugP("FAILED: %s: %s\n", name, err)
			numFailed++
			continue
		}
		cs2 := sha512.Sum512_256(buf.B())
		if bytes.Compare(cs, cs2[:]) != 0 {
			debugP("FAILED: %s: Checksum mismatch\n", name)
			numFailed++
			continue
		}
		numOk++
	}
	return
}

//---------------------------------------------------------------------------

func setup(bkDir string, pwFile string) (string, VersionMgr, ChunkMgr, error) {
	bkDir = filepath.Clean(bkDir)
	key, err := GetKey(pwFile, bkDir)
	if err != nil {
		return bkDir, nil, nil, err
	}
	vm := MakeVersionMgr(bkDir, key)
	cm := MakeChunkMgr(bkDir, key)
	return bkDir, vm, cm, nil
}

func writeLockfile(bkDir string) (string, error) {
	lfn := path.Join(bkDir, LOCK_FILENAME)
	lockFile, err := os.OpenFile(lfn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, DEFAULT_FILE_PERM)
	if os.IsExist(err) {
		return "", errors.New(fmt.Sprintf("Lock file %s already exist.", lfn))
	} else if err != nil {
		return "", err
	}
	lockFile.Close()
	return lfn, nil
}

func removeLockfile(lfn string) {
	os.Remove(lfn)
}

//---------------------------------------------------------------------------

func doInit(conf *config, bkDir string) error {
	bkDir = filepath.Clean(bkDir)
	if nodeExists(bkDir) {
		return errors.New(fmt.Sprintf("Backup directory already exist: %s", bkDir))
	}
	err := os.MkdirAll(bkDir, DEFAULT_DIR_PERM)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot create backup dir: %s", err))
	}
	err = os.MkdirAll(path.Join(bkDir, VERSIONS_DIR), DEFAULT_DIR_PERM)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot create backup dir: %s", err))
	}
	err = os.MkdirAll(path.Join(bkDir, CHUNK_DIR), DEFAULT_DIR_PERM)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot create backup dir: %s", err))
	}
	return WriteNewEncConfig(conf.pwFile, bkDir)
}

func doBackup(conf *config, src, bkDir string, subpaths []string) error {
	bkDir, vm, cm, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	lfn, err := writeLockfile(bkDir)
	if err != nil {
		return err
	}
	defer removeLockfile(lfn)
	src = filepath.Clean(src)
	v, err := vm.GetVersion("latest")
	if err != nil && !os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	}
	var header *Header
	var vfr FReader
	if v == "" {
		header = &Header{MAGIC, BKVERSION, conf.defaultChunkSize}
		vfr = &NullFReader{}
	} else {
		header, vfr, err = vm.LoadVersion(v)
		if err != nil {
			return errors.New(fmt.Sprintf("Cannot read version file: %s", err))
		}
	}
	ignorePatterns, err := readIgnoreFile(bkDir)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read ignore file: %s", err))
	}
	sfr, err := getSpecifiedFiles(ignorePatterns, src, subpaths)
	if err != nil {
		return err
	}
	var fw FWriter
	if !conf.dryRun {
		var t time.Time
		if conf.setVersion != "" {
			t2, err := time.Parse(time.RFC3339Nano, conf.setVersion)
			if err != nil {
				return errors.New(fmt.Sprintf("Invalid version %s", conf.version))
			}
			t = t2
		} else {
			t = time.Now()
			v2 := MakeVersionString(t)
			if v == v2 {
				time.Sleep(10 * time.Nanosecond)
				t = time.Now()
			}
		}
		fw, err = vm.SaveVersion(t, header)
		if err != nil {
			return err
		}
	} else {
		fw = &NullFWriter{}
	}
	buf := &Buf{}
	if err2 := mergeFileDataStream(vfr, sfr, func(saved, scanned *FileData) bool {
		err = backupOneFile(cm, src, header.ChunkSize, conf.dryRun, conf.checksum, conf.verbose, buf, fw, conf.out, saved, scanned)
		return err != nil
	}); err2 != nil && err == nil {
		err = err2
	}
	if err2 := vfr.Close(); err2 != nil && err == nil {
		err = err2
	}
	if err2 := sfr.Close(); err2 != nil && err == nil {
		err = err2
	}
	if err != nil {
		fw.Abort()
		return err
	}
	return fw.Close()
}

func doRecover(conf *config, bkDir, recDir string, patterns []string) error {
	bkDir, vm, cm, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	recDir = filepath.Clean(recDir)
	if nodeExists(recDir) && !conf.merge {
		return errors.New(fmt.Sprintf("Recovery dir %s already exists", recDir))
	}
	v, err := vm.GetVersion(conf.version)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	} else if v == "" {
		return errors.New("Backup is empty")
	}
	_, fr, err := vm.LoadVersion(v)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version file: %s", err))
	}
	if !conf.testRun && !conf.dryRun {
		os.MkdirAll(recDir, DEFAULT_DIR_PERM)
	}
	buf := &Buf{}
	var fd *FileData
	ok := true
	for fd, err = fr.Next(); err == nil; fd, err = fr.Next() {
		if matchRecoveryPatterns(fd.Name, patterns) {
			var err2 error
			if !conf.dryRun {
				err2 = recoverNode(fd, cm, recDir, conf.testRun, conf.merge, buf)
			}
			if err2 == nil {
				fmt.Fprintf(conf.out, "%s\n", fd.PrettyPrint())
			} else {
				debugP("Cannot recover file %s: %s\n", fd.PrettyPrint(), err)
				fmt.Fprintf(os.Stderr, "Failed: %s\n", fd.PrettyPrint())
				ok = false
			}
		}
	}
	if isError(err) {
		fr.Close()
		return errors.New(fmt.Sprintf("Error reading version file: %s", err))
	}
	if err = fr.Close(); err != nil {
		return errors.New(fmt.Sprintf("Error reading version file: %s", err))
	}
	if !ok {
		return errors.New("Failed! Some files were not recovered.")
	}
	return nil
}

func doFiles(conf *config, bkDir string) error {
	bkDir, vm, _, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	v, err := vm.GetVersion(conf.version)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	} else if v == "" {
		return errors.New("Backup is empty")
	}
	_, fr, err := vm.LoadVersion(v)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version file: %s", err))
	}
	var fd *FileData
	for fd, err = fr.Next(); err == nil; fd, err = fr.Next() {
		fmt.Fprintf(conf.out, "%s\n", fd.PrettyPrint())
	}
	if isError(err) {
		fr.Close()
		return errors.New(fmt.Sprintf("Error reading version file: %s", err))
	}
	if err = fr.Close(); err != nil {
		return errors.New(fmt.Sprintf("Error reading version file: %s", err))
	}
	return nil
}

func doVersions(conf *config, bkDir string) error {
	bkDir, vm, _, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	}
	for _, v := range versions {
		fmt.Fprintf(conf.out, "%s\n", v)
	}
	return nil
}

func doDeleteVersion(conf *config, bkDir, v string) error {
	bkDir, vm, _, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	err = vm.DeleteVersion(v)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot delete version %s: %s", v, err))
	}
	return nil
}

func doDeleteOldVersions(conf *config, bkDir string) error {
	bkDir, vm, _, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	}
	d := ReduceVersions(time.Now(), versions)
	for _, v := range d {
		fmt.Fprintf(conf.out, "Deleting version %s\n", v)
		if !conf.dryRun {
			err = vm.DeleteVersion(v)
			if err != nil {
				return errors.New(fmt.Sprintf("Cannot delete version %s: %s", v, err))
			}
		}
	}
	return nil
}

type verifyBackupResults struct {
	numChunks, numOk, numFailed, numUnused int
}

func doVerifyBackups(conf *config, bkDir string, r *verifyBackupResults) error {
	bkDir, vm, cm, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	}
	counts := cm.GetAllChunks()
	for _, v := range versions {
		_, fr, err := vm.LoadVersion(v)
		if err != nil {
			return errors.New(fmt.Sprintf("Cannot read version file: %s", err))
		}
		var fd *FileData
		for fd, err = fr.Next(); err == nil; fd, err = fr.Next() {
			for _, chunk := range fd.Chunks {
				counts[chunk]++
			}
		}
		if isError(err) {
			fr.Close()
			return errors.New(fmt.Sprintf("Error reading version file: %s", err))
		}
		if err = fr.Close(); err != nil {
			return errors.New(fmt.Sprintf("Error reading version file: %s", err))
		}
	}
	buf := &Buf{}
	numChunks, numOk, numFailed, numUnused := verifyChunks(cm, buf, counts)
	fmt.Fprintf(conf.out, "%d chunks, %d ok, %d failed, %d unused\n", numChunks, numOk, numFailed, numUnused)
	if r != nil {
		r.numChunks = numChunks
		r.numOk = numOk
		r.numFailed = numFailed
		r.numUnused = numUnused
	}
	return nil
}

func doPurgeOldData(conf *config, bkDir string) error {
	bkDir, vm, cm, err := setup(bkDir, conf.pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot read version files: %s", err))
	}
	counts := cm.GetAllChunks()
	for _, v := range versions {
		_, fr, err := vm.LoadVersion(v)
		if err != nil {
			return errors.New(fmt.Sprintf("Cannot read version file: %s", err))
		}
		var fd *FileData
		for fd, err = fr.Next(); err == nil; fd, err = fr.Next() {
			for _, chunk := range fd.Chunks {
				delete(counts, chunk)
			}
		}
		if isError(err) {
			fr.Close()
			return errors.New(fmt.Sprintf("Error reading version file: %s", err))
		}
		if err = fr.Close(); err != nil {
			return errors.New(fmt.Sprintf("Error reading version file: %s", err))
		}
	}
	numDeleted := 0
	numFailed := 0
	var sizeDeleted int64
	var sizeFailed int64
	for chunk, _ := range counts {
		_, size, ok := DecodeChunkName(chunk)
		if ok {
			ok = cm.DeleteChunk(chunk)
		}
		if ok {
			numDeleted++
			sizeDeleted += int64(size)
			debugP("DELETED %s (%d)\n", chunk, size)
		} else {
			numFailed++
			sizeFailed += int64(size)
			debugP("FAILED %s %d\n", chunk, size)
		}
	}
	notOne := func(x int) string {
		if x == 1 {
			return ""
		} else {
			return "s"
		}
	}
	fmt.Fprintf(conf.out, "Deleted %d chunk%s (%d bytes).\n", numDeleted, notOne(numDeleted), sizeDeleted)
	if numFailed > 0 {
		return errors.New(fmt.Sprintf("Failed to delete %d chunk%s (%d bytes).", numFailed, notOne(numFailed), sizeFailed))
	}
	return nil
}

//---------------------------------------------------------------------------

func usageAndExit() {
	fmt.Fprintf(os.Stderr, `Usage:
  vecbackup [-pw <password>] init <backupdir>
    Initialize a new backup directory.

  vecbackup backup [-v] [-cs] [-n] [-setversion <version>] [-pw <password>] <srcdir> <backupdir> [<subpath> ...]
    Incrementally and recursively backs up the files, directories and symbolic
    links (items) in <srcdir> to <backupdir>. If one or more <subpath> are
    specified, only backup those subpaths. <subpaths> are relative to <srcdir>.
    Prints the items that are added (+), removed (-) from or updated (*).
    Files that have not changed in same size and timestamp are not backed up
    again.
      -v            verbose, prints the names of all items backed up
      -cs           use checksums to detect if files have changed
      -n            dry run, show what would have been backed up
      -setversion   save as the given version, instead of the current time

  vecbackup versions [-pw <password>] <backupdir>
    Lists all backup versions in chronological order. The version name is a
    timestamp in UTC formatted with RFC3339Nano format (YYYY-MM-DDThh:mm:ssZ).

  vecbackup files [-version <version>] [-pw <password>] <backupdir>
    Lists files in <backupdir>.
    -version <version>   list the files in that version

  vecbackup recover [-n] [-t] [-version <version>] [-merge] [-pw <password>] <backupdir> <recoverydir> [<subpath> ...]
    Recovers all the items or the given <subpaths> to <recoverydir>.
    <recoverydir> must not already exist.
      -n            dry run, show what would have been recovered
      -t            test run, test recovering the files but don't write
      -version <version>
                    recover that given version. Defaults to "latest"
      -merge        merge the recovered files into the given <recoverydir>
                    if it already exists. Files of the same size and timestamp
                    are not extracted again. This can be used to resume
                    a previous recover operation.

  vecbackup delete-version [-pw <password>] <backupdir> <version>
    Deletes the given version. No chunks are deleted.

  vecbackup delete-old-versions [-n] [-pw <password>] <backupdir>
    Deletes old versions. No chunks are deleted.
    Keeps all versions wihin one day, one version per hour for the last week,
    one version per day in the last month, one version per week in the last 
    year and one version per month otherwise.
      -n            dry run, show versions that would have been deleted

  vecbackup verify-backups [-pw <password>] <backupdir>
    Verifies that all the chunks used by all the files in all versions
    can be read and match their checksums.

  vecbackup purge-old-data [-pw <password>] <backupdir>
    Deletes chunks that are not used by any file in any backup version.

Other common flags:
      -pw           file containing the password

Files:
  vecbackup-ignore

    If vecbackup-ignore exists in the <backupdir>, each line is treated as a
    pattern to match against the items. Any items with names that match
    the patterns are ignored for new backup operations. This does not affect
    the backups in previous versions.

    Ignore Patterns:

      Patterns that do not start with a '/' are matched against the filename
      only.
      Patterns that start with a '/' are matched against the sub-path relative
      to src directory.
      * matches any sequence of non-separator characters.
      ? matches any single non-separator character.
      See https://golang.org/pkg/path/filepath/#Match
`)
	os.Exit(1)
}

var debugF = flag.Bool("debug", false, "Show debug info.")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

type config struct {
	verbose          bool
	checksum         bool
	dryRun           bool
	testRun          bool
	setVersion       string
	version          string
	merge            bool
	pwFile           string
	out              io.Writer
	defaultChunkSize int
}

func InitConfig() *config {
	conf := &config{}
	flag.BoolVar(&conf.verbose, "v", false, "Verbose")
	flag.BoolVar(&conf.checksum, "cs", false, "Use checksums to check if file contents have changed.")
	flag.BoolVar(&conf.dryRun, "n", false, "Dry run.")
	flag.BoolVar(&conf.testRun, "t", false, "Test run.")
	flag.StringVar(&conf.version, "version", "latest", `The version to operate on or the special keyword "latest".`)
	flag.StringVar(&conf.setVersion, "setversion", "", "Set to this version instead of using the current time.")
	flag.BoolVar(&conf.merge, "merge", false, "Merge into existing directory.")
	flag.StringVar(&conf.pwFile, "pw", "", "File containing password")
	conf.out = os.Stdout
	conf.defaultChunkSize = DEFAULT_CHUNK_SIZE
	log.SetFlags(0)
	return conf
}

func debugP(fmt string, v ...interface{}) {
	if *debugF {
		log.Printf(fmt, v...)
	}
}

func exitIfError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func main() {
	conf := InitConfig()
	if len(os.Args) < 2 {
		usageAndExit()
	}
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create cpu profile: %v", err))
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if cmd == "init" {
		if flag.NArg() < 1 {
			usageAndExit()
		}
		exitIfError(doInit(conf, flag.Arg(0)))
	} else if cmd == "backup" {
		if flag.NArg() < 2 {
			usageAndExit()
		}
		exitIfError(doBackup(conf, flag.Arg(0), flag.Arg(1), flag.Args()[2:]))
	} else if cmd == "files" {
		if flag.NArg() != 1 {
			usageAndExit()
		}
		exitIfError(doFiles(conf, flag.Arg(0)))
	} else if cmd == "recover" {
		if flag.NArg() < 2 {
			usageAndExit()
		}
		exitIfError(doRecover(conf, flag.Arg(0), flag.Arg(1), flag.Args()[2:]))
	} else if cmd == "versions" {
		if flag.NArg() != 1 {
			usageAndExit()
		}
		exitIfError(doVersions(conf, flag.Arg(0)))
	} else if cmd == "delete-version" {
		if flag.NArg() != 2 {
			usageAndExit()
		}
		exitIfError(doDeleteVersion(conf, flag.Arg(0), flag.Arg(1)))
	} else if cmd == "delete-old-versions" {
		if flag.NArg() != 1 {
			usageAndExit()
		}
		exitIfError(doDeleteOldVersions(conf, flag.Arg(0)))
	} else if cmd == "verify-backups" {
		if flag.NArg() != 1 {
			usageAndExit()
		}
		exitIfError(doVerifyBackups(conf, flag.Arg(0), nil))
	} else if cmd == "purge-old-data" {
		if flag.NArg() != 1 {
			usageAndExit()
		}
		exitIfError(doPurgeOldData(conf, flag.Arg(0)))
	} else {
		usageAndExit()
	}
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create memory profile: %v", err))
		}
		//runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(fmt.Sprintf("could not write memory profile: %v", err))
		}
		f.Close()
	}
}
