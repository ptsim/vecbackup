package main

import (
	"bytes"
	"flag"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	SRCDIR = "test_src"
	BKDIR  = "test_bk"
	RECDIR = "test_rec"
	PWFILE = "test_pw"
)

var longTest = flag.Bool("longtest", false, "Long test.")

var flags *Flags = InitFlags()

func setupTest(t testing.TB, name string) func() {
	flags.verbose = false
	flags.checksum = false
	flags.dryRun = false
	flags.testRun = false
	flags.version = "latest"
	flags.setVersion = ""
	flags.merge = false
	flags.pwFile = ""
	flags.out = ioutil.Discard
	flags.chunkSize = DEFAULT_CHUNK_SIZE

	removeAll(t, SRCDIR)
	removeAll(t, BKDIR)
	removeAll(t, RECDIR)
	removeAll(t, PWFILE)
	os.MkdirAll(SRCDIR, 0755)
	return func() { teardownTest(t) }
}

func teardownTest(t testing.TB) {
	removeAll(t, SRCDIR)
	removeAll(t, BKDIR)
	removeAll(t, RECDIR)
	removeAll(t, PWFILE)
}

func removeAll(t testing.TB, d string) {
	err := os.RemoveAll(d)
	if err != nil {
		t.Fatalf("Can't remove test dir at %v: %v", d, err)
	}
	_ = os.Remove(d)
}

func makeBytePattern(len, offset int) []byte {
	b := make([]byte, len, len)
	for i := 0; i < len; i++ {
		b[i] = byte(offset + i)
	}
	return b
}

func makeBytePatternFromName(name string) []byte {
	h := fnv.New32()
	h.Write([]byte(name))
	s := h.Sum(nil)
	size := int(s[0])*8 + int(s[1])
	offset := int(s[2])*8 + int(s[3])
	return makeBytePattern(size, offset)
}

type Op struct {
	op string
	p  string
}

type TestEnv struct {
	t  testing.TB
	ft time.Time
}

func (e *TestEnv) failIfError(name string, err error) {
	if err != nil {
		e.t.Fatalf("%s failed: %s\n", name, err)
	}
}

func (e *TestEnv) setPW(pw []byte) {
	if len(pw) == 0 {
		flags.pwFile = ""
	} else {
		err := ioutil.WriteFile(PWFILE, pw, 0444)
		if err != nil {
			e.t.Fatalf("setPW failed: %s", err)
		}
		flags.pwFile = PWFILE
	}
}

func (e *TestEnv) init() {
	e.failIfError("init", doInit(flags, BKDIR))
}

func (e *TestEnv) backup() {
	e.failIfError("backup", doBackup(flags, BKDIR, SRCDIR, nil))
}

func (e *TestEnv) backupSubpaths(subpaths []string) {
	e.failIfError("backup", doBackup(flags, BKDIR, SRCDIR, subpaths))
}

func (e *TestEnv) recover() []string {
	var b bytes.Buffer
	flags.out = &b
	e.failIfError("recover", doRecover(flags, BKDIR, RECDIR, nil))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) recoverFiles(patterns []string) []string {
	var b bytes.Buffer
	flags.out = &b
	e.failIfError("recover", doRecover(flags, BKDIR, RECDIR, patterns))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) verifyBackups() (numChunks, numOk, numFailed, numUnused int) {
	var r verifyBackupResults
	e.failIfError("verifyBackups", doVerifyBackups(flags, BKDIR, &r))
	return r.numChunks, r.numOk, r.numFailed, r.numUnused
}

func (e *TestEnv) purgeOldData() string {
	var b bytes.Buffer
	flags.out = &b
	e.failIfError("purgeOldData", doPurgeOldData(flags, BKDIR))
	return b.String()
}

func (e *TestEnv) add(f string) {
	h := fnv.New32()
	h.Write([]byte(f))
	s := h.Sum(nil)
	size := int(s[0])*8 + int(s[1])
	offset := int(s[2])*8 + int(s[3])
	e.addFile(f, size, offset)
}

func (e *TestEnv) addFile(f string, size, offset int) {
	e.addFileWithData(f, makeBytePattern(size, offset))
}

func (e *TestEnv) addFileWithData(f string, data []byte) {
	p := path.Join(SRCDIR, f)
	d := filepath.Dir(p)
	err := os.MkdirAll(d, 0777)
	if err != nil {
		e.t.Fatalf("mkdirall failed: %s", err)
	}
	err = ioutil.WriteFile(p, data, 0666)
	if err != nil {
		e.t.Fatalf("addFile failed: %s: %s", p, err)
	}
	err = os.Chtimes(p, e.ft, e.ft)
	if err != nil {
		e.t.Fatalf("addFile failed to chtimes: %s: %s", p, err)
	}
	e.ft = e.ft.Add(time.Second)
}

func (e *TestEnv) addSymlink(f, target string) {
	p := path.Join(SRCDIR, f)
	d := filepath.Dir(p)
	err := os.MkdirAll(d, 0777)
	if err != nil {
		e.t.Fatalf("mkdirall failed: %s: %s", d, err)
	}
	err = os.Symlink(target, p)
	if err != nil {
		e.t.Fatalf("addSymlink failed: %s: %s", p, err)
	}
}

func (e *TestEnv) addDir(f string) {
	p := path.Join(SRCDIR, f)
	err := os.MkdirAll(p, 0777)
	if err != nil {
		e.t.Fatalf("mkdirall failed: %s: %s", p, err)
	}
}

func (e *TestEnv) rm(f string) {
	err := os.Remove(path.Join(SRCDIR, f))
	if err != nil {
		e.t.Fatalf("rm failed: %s: %s", f, err)
	}
}

func (e *TestEnv) tryrm(f string) {
	os.Remove(path.Join(SRCDIR, f))
}

func (e *TestEnv) rmRec(f string) {
	err := os.Remove(path.Join(RECDIR, f))
	if err != nil {
		e.t.Fatalf("rm failed: %s: %s", f, err)
	}
}

func (e *TestEnv) versions() []string {
	var b bytes.Buffer
	flags.out = &b
	e.failIfError("versions", doVersions(flags, BKDIR))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) deleteVersion(version string) {
	e.failIfError("deleteVersion", doDeleteVersion(flags, BKDIR, version))
}

func (e *TestEnv) files(version string) []string {
	if version == "" {
		version = "latest"
	}
	flags.version = version
	var b bytes.Buffer
	flags.out = &b
	e.failIfError("files", doFiles(flags, BKDIR))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) filesMatch(version string, l []string) {
	l2 := e.files(version)
	ok := true
	if len(l) == len(l2) {
		for i, f := range l {
			if f != l2[i] {
				ok = false
				break
			}
		}
	} else {
		ok = false
	}
	if !ok {
		e.t.Errorf("filesMatch failed: Expected:%v, got %v, %v, %v", l, l2, len(l), len(l2))
	}
}

func (e *TestEnv) clean(what string) {
	if what == "rec" {
		removeAll(e.t, RECDIR)
	} else if what == "src" {
		removeAll(e.t, SRCDIR)
		os.MkdirAll(SRCDIR, 0755)
	} else if what == "bk" {
		removeAll(e.t, BKDIR)
	} else {
		e.t.Fatalf("Invalid command: clean %v", what)
	}
}

func (e *TestEnv) print(what string) {
	if what == "rec" {
		e.t.Logf("%v", walkDir(e.t, RECDIR, nil))
	} else if what == "src" {
		e.t.Logf("%v", walkDir(e.t, SRCDIR, nil))
	} else {
		e.t.Fatalf("Invalid command: print %v", what)
	}
}

func (e *TestEnv) chmod(f string, perm os.FileMode) {
	p := path.Join(SRCDIR, f)
	err := os.Chmod(p, os.FileMode(perm))
	if err != nil {
		e.t.Fatalf("chmod %v %v failed: %s", f, perm, err)
	}
}

func (e *TestEnv) checkSame() {
	if !compareDir(e.t, SRCDIR, RECDIR) {
		e.t.Fatalf("compare failed")
	}
}

func (e *TestEnv) checkExistDir(f string) {
	s, err := os.Stat(path.Join(RECDIR, f))
	if err != nil || !s.Mode().IsDir() {
		e.t.Fatalf("Dir %v does not exist", f)
	}
}

func (e *TestEnv) checkExistFile(f string) {
	s, err := os.Stat(path.Join(RECDIR, f))
	if err != nil || !s.Mode().IsRegular() {
		e.t.Fatalf("File %v does not exist", f)
	}
}

func (e *TestEnv) checkNotExist(f string) {
	_, err := os.Stat(path.Join(RECDIR, f))
	if !os.IsNotExist(err) {
		e.t.Fatalf("%v exists", f)
	}
}

type testFunc func(e *TestEnv)

func doTestSeq(t testing.TB, name string, tf testFunc) {
	defer setupTest(t, name)()
	e := TestEnv{t, time.Date(2017, time.January, 01, 02, 0, 0, 0, time.UTC)}
	tf(&e)
}

func compareDir(t testing.TB, p1, p2 string) bool {
	files1, err1 := ioutil.ReadDir(p1)
	if err1 != nil {
		t.Errorf("compareDir readdir failed: %v %v", p1, err1)
		return false
	}
	files2, err2 := ioutil.ReadDir(p2)
	if err1 != nil {
		t.Errorf("compareDir readdir failed: %v %v", p2, err2)
		return false
	}
	if len(files1) != len(files2) {
		t.Errorf("compareDir readdir len mismatch: %v %v %v %v", p1, len(files1), p2, len(files2))
		return false
	}
	for i, file1 := range files1 {
		file2 := files2[i]
		if file1.Name() != file2.Name() {
			t.Errorf("compareDir name mismatch: %v %v %v %v", p1, file1.Name(), p2, file2.Name())
			return false
		}
		if file1.Mode().IsRegular() && !file2.Mode().IsRegular() {
			t.Errorf("compareDir file2 should also be a file: %v %v %v %v", p1, file1.Mode().IsRegular(), p2, file2.Mode().IsRegular())
			return false
		}
		if file1.IsDir() && !file2.IsDir() {
			t.Errorf("compareDir file2 should also be a dir: %v %v %v %v", p1, file1.IsDir(), p2, file2.IsDir())
			return false
		}
		if IsSymlink(file1) && !IsSymlink(file2) {
			t.Errorf("compareDir file2 should also be a symlink: %v %v", p1, p2)
			return false
		}
		if !file1.Mode().IsRegular() && !file1.IsDir() && !IsSymlink(file1) {
			t.Errorf("compareDir file1 is not a file or dir or symlink: %v %v", p1, file1.Mode())
			return false
		}
		if !file2.Mode().IsRegular() && !file2.IsDir() && !IsSymlink(file2) {
			t.Errorf("compareDir file2 is not a file or dir or symlink: %v %v", p2, file2.Mode())
			return false
		}
		c1 := path.Join(p1, file1.Name())
		c2 := path.Join(p2, file2.Name())
		if file1.IsDir() {
			if file1.Mode().Perm() != file2.Mode().Perm() {
				t.Errorf("compareDir permissions mismatch: %v %v %v %v", p1, file1.Mode().Perm(), p2, file2.Mode().Perm())
				return false
			}
			if !compareDir(t, c1, c2) {
				return false
			}
		} else if file1.Mode().IsRegular() {
			if file1.ModTime() != file2.ModTime() {
				t.Errorf("compareDir time mismatch: %v %v %v %v", p1, file1.ModTime(), p2, file2.ModTime())
				return false
			}
			if file1.Mode().Perm() != file2.Mode().Perm() {
				t.Errorf("compareDir permissions mismatch: %v %v %v %v", p1, file1.Mode().Perm(), p2, file2.Mode().Perm())
				return false
			}
			if !compareFile(t, c1, c2) {
				t.Errorf("compareDir files mismatch: %v %v", c1, c2)
				return false
			}
		} else {
			t1, err := os.Readlink(c1)
			if err != nil {
				t.Errorf("Can't read link from %s: %v\n", c1, err)
				return false
			}
			t2, err := os.Readlink(c2)
			if err != nil {
				t.Errorf("Can't read link from %s: %v\n", c2, err)
				return false
			}
			if t1 != t2 {
				t.Errorf("compareDir symlinks mismatch: %v %v %v %v", c1, t1, c2, t2)
				return false
			}
		}
	}
	return true
}

func compareFile(t testing.TB, p1, p2 string) bool {
	b1, err1 := ioutil.ReadFile(p1)
	if err1 != nil {
		t.Errorf("compareFile open failed: %v %v", p1, err1)
		return false
	}
	b2, err2 := ioutil.ReadFile(p2)
	if err2 != nil {
		t.Errorf("compareFile open failed: %v %v", p2, err2)
		return false
	}
	same := bytes.Compare(b1, b2) == 0
	if !same {
		t.Errorf("compareFile failed: %v %v %v %v", p1, b1, p2, b2)
	}
	return same
}

func walkDir(t testing.TB, p string, out []string) []string {
	files, err := ioutil.ReadDir(p)
	if err != nil {
		t.Errorf("walkDir failed: %v %v", p, err)
		return out
	}
	for _, f := range files {
		fp := path.Join(p, f.Name())
		if f.IsDir() {
			out = append(out, fp+string(os.PathSeparator))
			out = walkDir(t, fp, out)
		} else if f.Mode().IsRegular() {
			out = append(out, fp)
		} else {
			t.Errorf("walkDir not file or dir: %v %v", fp, f.Mode())
		}
	}
	return out
}

func TestT01(t *testing.T) {
	doTestSeq(t, "T01 Empty", func(e *TestEnv) {
		e.init()
		e.backup()
		e.recover()
		e.checkSame()
	})
	doTestSeq(t, "T01 Empty PW", func(e *TestEnv) {
		e.setPW([]byte("hahahahahaa"))
		e.init()
		e.backup()
		e.recover()
		e.checkSame()
	})
}

func TestT02(t *testing.T) {
	doTestSeq(t, "T02 a few files", func(e *TestEnv) {
		e.init()
		e.add("aaa")
		e.add("bb")
		e.add("c")
		e.addSymlink("d", "aaa")
		e.addSymlink("e", "../../zzz")
		e.backup()
		e.recover()
		e.checkSame()
	})
}

func TestT03(t *testing.T) {
	doTestSeq(t, "T03 files and dirs", func(e *TestEnv) {
		e.init()
		e.add("z/aaa")
		e.add("z/bb")
		e.add("y/c")
		e.add("z/c")
		e.add("www/c")
		e.addSymlink("wwwx", "y/c")
		e.addSymlink("z/cc", "fsdfsdf/fsdfsdf/sdf/ds/fsd/")
		e.backup()
		e.recover()
		e.checkSame()
	})
}

func TestT04(t *testing.T) {
	doTestSeq(t, "T04 rm", func(e *TestEnv) {
		e.init()
		e.add("z/aa")
		e.add("z/bb")
		e.add("y/cc")
		e.add("z/cc")
		e.add("w/dd")
		e.addSymlink("x/ee", "/tmp/fdsfsdf")
		e.backup()
		e.recover()
		e.checkSame()
		e.rm("z/bb")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("y/cc")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("y")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("w/dd")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("w")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("x/ee")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("x")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z/aa")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z/cc")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
	})
}

func TestT05(t *testing.T) {
	doTestSeq(t, "T05 file -> dir -> symlink -> file", func(e *TestEnv) {
		e.init()
		e.add("z")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z")
		e.add("z/a")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z/a")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z")
		e.addSymlink("z", "what")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z")
		e.addSymlink("z", "huh")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.rm("z")
		e.add("z")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
	})
}

func TestT06(t *testing.T) {
	doTestSeq(t, "T06 Permissions", func(e *TestEnv) {
		e.init()
		e.add("aaa")
		e.add("bbb")
		e.add("zz/ccc")
		e.add("zz/ddd")
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.chmod("aaa", 0444)
		e.chmod("bbb", 0400)
		e.chmod("zz/ccc", 0440)
		e.chmod("zz/ddd", 0755)
		e.chmod("zz", 0742)
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.chmod("aaa", 0555)
		e.chmod("bbb", 0500)
		e.chmod("zz/ccc", 0550)
		e.chmod("zz/ddd", 0456)
		e.chmod("zz", 0750)
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.chmod("aaa", 0666)
		e.chmod("bbb", 0600)
		e.chmod("zz/ccc", 0660)
		e.chmod("zz/ddd", 0654)
		e.chmod("zz", 0767)
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
		e.chmod("aaa", 0777)
		e.chmod("bbb", 0700)
		e.chmod("zz/ccc", 0700)
		e.chmod("zz/ddd", 0765)
		e.chmod("zz", 0713)
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
	})
}

func TestT07(t *testing.T) {
	doTestSeq(t, "T07 Block sizes", func(e *TestEnv) {
		for i := 0; i < 20; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("c%d", fs), fs-1, i)
		}
		sizes := []int{1 << 12, 1 << 16, 1 << 20, 1 << 24}
		for _, size := range sizes {
			flags.chunkSize = size
			e.init()
			e.backup()
			e.recover()
			e.checkSame()
			e.clean("bk")
			e.clean("rec")
		}
	})
}

func TestT08(t *testing.T) {
	doTestSeq(t, "T08 Subpaths", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/d")
		e.add("e/f")
		e.add("e g/f")
		e.backupSubpaths([]string{"c"})
		e.filesMatch("", []string{"c/", "c/d"})
		e.clean("rec")
		e.recover()
		e.checkNotExist("a")
		e.checkNotExist("b")
		e.checkExistDir("c")
		e.checkExistFile("c/d")
		e.checkNotExist("e")
		e.checkNotExist("e/f")
		e.checkNotExist("e g/f")
		e.backupSubpaths([]string{"e", "c"})
		e.filesMatch("", []string{"c/", "c/d", "e/", "e/f"})
		e.clean("rec")
		e.recover()
		e.checkNotExist("a")
		e.checkNotExist("b")
		e.checkExistDir("c")
		e.checkExistFile("c/d")
		e.checkExistDir("e")
		e.checkExistFile("e/f")
		e.checkNotExist("e g/f")
		e.backupSubpaths([]string{})
		e.filesMatch("", []string{"a", "b", "c/", "c/d", "e/", "e/f", "e g/", "e g/f"})
		e.clean("rec")
		e.recover()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkExistDir("c")
		e.checkExistFile("c/d")
		e.checkExistDir("e")
		e.checkExistFile("e/f")
		e.checkExistFile("e g/f")
		e.checkSame()
	})
}

func TestT09(t *testing.T) {
	doTestSeq(t, "T09 Recover -merge", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("d/d2/d3/d4")
		e.backup()
		e.recover()
		e.checkSame()
		e.rmRec("b")
		e.checkNotExist("b")
		e.rmRec("c/c2")
		e.rmRec("c")
		e.checkNotExist("c")
		flags.merge = true
		e.recover()
		e.checkSame()
	})
}

func TestT10(t *testing.T) {
	doTestSeq(t, "T10 backup -n and files", func(e *TestEnv) {
		e.init()
		e.backup()
		e.filesMatch("", []string{})
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("d/d2/d3/d4")
		flags.dryRun = true
		e.backup()
		e.filesMatch("", []string{})
		flags.dryRun = false
		e.backup()
		e.filesMatch("", []string{"a", "b", "c/", "c/c2", "d/", "d/d2/", "d/d2/d3/", "d/d2/d3/d4"})
		e.recover()
		e.checkSame()
	})
}

func TestT11(t *testing.T) {
	doTestSeq(t, "T11 files -version and versions", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.backup()
		e.add("b")
		e.backup()
		e.add("c/c2")
		e.backup()
		e.add("d")
		e.backup()
		v := e.versions()
		allFiles := []string{"a", "b", "c/", "c/c2", "d"}
		e.filesMatch(v[0], allFiles[:1])
		e.filesMatch(v[1], allFiles[:2])
		e.filesMatch(v[2], allFiles[:4])
		e.filesMatch(v[3], allFiles)
	})
}

func TestT12(t *testing.T) {
	doTestSeq(t, "T12 backup -cs", func(e *TestEnv) {
		flags.checksum = true
		e.init()
		for i := 0; i < 20; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("c%d", fs), fs-1, i)
		}
		e.backup()
		e.recover()
		e.checkSame()
		e.backup()
		e.clean("rec")
		e.recover()
		e.checkSame()
	})
}

func TestT13(t *testing.T) {
	doTestSeq(t, "T13 recover -n and -t", func(e *TestEnv) {
		e.init()
		for i := 0; i < 20; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("c%d", fs), fs-1, i)
		}
		e.backup()
		flags.dryRun = true
		files := e.recover()
		e.checkNotExist("")
		e.filesMatch("", files)
		flags.dryRun = false
		flags.testRun = true
		files = e.recover()
		e.checkNotExist("")
		e.filesMatch("", files)
		flags.testRun = false
		e.recover()
		e.checkSame()
	})
}

func TestT14(t *testing.T) {
	doTestSeq(t, "T14 recover -version", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.backup()
		e.add("b")
		e.backup()
		e.add("c/c2")
		e.backup()
		e.add("d")
		e.backup()
		v := e.versions()
		flags.version = v[0]
		e.clean("rec")
		e.recover()
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c")
		e.checkNotExist("c/c2")
		e.checkNotExist("d")
		flags.version = v[1]
		e.clean("rec")
		e.recover()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c")
		e.checkNotExist("c/c2")
		e.checkNotExist("d")
		flags.version = v[2]
		e.clean("rec")
		e.recover()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkExistDir("c")
		e.checkExistFile("c/c2")
		e.checkNotExist("d")
		flags.version = v[3]
		e.clean("rec")
		e.recover()
		e.checkSame()
	})
}

func TestT15(t *testing.T) {
	doTestSeq(t, "T15 recover files", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("c/c3")
		e.add("d/d2/d3")
		e.backup()
		e.recoverFiles([]string{"a"})
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c/c2")
		e.checkNotExist("c/c3")
		e.checkNotExist("d/d2/d3")
		flags.merge = true
		e.recoverFiles([]string{"b"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c/c2")
		e.checkNotExist("c/c3")
		e.checkNotExist("d/d2/d3")
		e.recoverFiles([]string{"c/c3"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c/c2")
		e.checkExistFile("c/c3")
		e.checkNotExist("d/d2/d3")
		e.recoverFiles([]string{"c"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkExistFile("c/c2")
		e.checkExistFile("c/c3")
		e.checkNotExist("d/d2/d3")
	})
}

func TestT16(t *testing.T) {
	doTestSeq(t, "T16 delete-version", func(e *TestEnv) {
		e.init()
		e.add("a")
		e.backup()
		e.add("b")
		e.backup()
		e.add("c/c2")
		e.backup()
		e.add("d")
		e.backup()
		v := e.versions()
		if len(v) != 4 {
			e.t.Errorf("Should have 4 versions: %d", len(v))
		}
		e.deleteVersion(v[1])
		v2 := e.versions()
		if v[0] != v2[0] || v[2] != v2[1] || v[3] != v2[2] {
			e.t.Errorf("Versions do not match: %v==%v, %v==%v, %v==%v", v[0], v2[0], v[2], v2[1], v[3], v2[2])
		}
		e.deleteVersion(v[3])
		v2 = e.versions()
		if v[0] != v2[0] || v[2] != v2[1] {
			e.t.Errorf("Versions do not match: %v==%v, %v==%v", v[0], v2[0], v[2], v2[1])
		}
		e.deleteVersion(v[0])
		v2 = e.versions()
		if v[2] != v2[0] {
			e.t.Errorf("Versions do not match: %v==%v", v[2], v2[0])
		}
		e.deleteVersion(v[2])
		v2 = e.versions()
		if len(v2) != 0 {
			e.t.Errorf("Versions should be empty: %v", v2)
		}
	})
}

func TestT17(t *testing.T) {
	doTestSeq(t, "T17 setVersion", func(e *TestEnv) {
		e.init()
		e.add("a")
		flags.setVersion = "2011-02-03T04:05:06Z"
		e.backup()
		e.add("b")
		flags.setVersion = "2011-02-03T04:01:06Z"
		e.backup()
		e.add("c")
		flags.setVersion = "2011-02-03T04:09:06Z"
		e.backup()
		flags.setVersion = ""
		e.clean("rec")
		flags.version = "2011-02-03T04:01:06Z"
		e.recover()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c")
		e.clean("rec")
		flags.version = "2011-02-03T04:05:06Z"
		e.recover()
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c")
		e.clean("rec")
		flags.version = "2011-02-03T04:09:06Z"
		e.recover()
		e.checkExistFile("a")
		e.checkExistFile("c")
		e.checkExistFile("c")
	})
}

func TestT18(t *testing.T) {
	doTestSeq(t, "T18 verify-backups", func(e *TestEnv) {
		e.init()
		for i := 0; i < 100; i++ {
			fs := 1024*i + 1
			e.addFile(fmt.Sprintf("f%d", fs), fs, i)
			e.addFile(fmt.Sprintf("d/g%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("z/y/x%d", fs), fs-1, i)
			e.addFile(fmt.Sprintf("d1/d2/d3/d4/d5/d6/d7/f%d", fs), fs-1, i)
		}
		e.backup()
		numChunks, numOk, numFailed, numUnused := e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		flags.dryRun = true
		e.recover()
		e.checkNotExist("")
		flags.dryRun = false
		e.recover()
		e.checkSame()
	})
}

func TestT19(t *testing.T) {
	doTestSeq(t, "T19 purge-old-data", func(e *TestEnv) {
		e.init()
		for i := 0; i < 10; i++ {
			fs := 1024*i + 1
			e.addFile(fmt.Sprintf("f%d", fs), fs, i)
		}
		e.backup()
		numChunks, numOk, numFailed, numUnused := e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numFailed != 0 || numUnused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		for i := 0; i < 10; i++ {
			fs := 2024*i + 1
			e.addFile(fmt.Sprintf("g%d", fs), fs, i)
		}
		e.backup()
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numFailed != 0 || numUnused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		flags.dryRun = true
		e.recover()
		e.checkNotExist("")
		flags.dryRun = false
		e.recover()
		e.checkSame()
		v := e.versions()
		e.purgeOldData()
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numFailed != 0 || numUnused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		e.clean("rec")
		flags.version = v[0]
		e.recover()
		e.clean("rec")
		flags.version = v[1]
		e.recover()
		e.checkSame()
		flags.version = ""
		e.deleteVersion(v[1])
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numUnused == 0 {
			e.t.Fatal("Number of unused chunks should be more than zero")
		}
		e.purgeOldData()
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numFailed != 0 || numUnused != 0 {
			e.t.Fatal("Number of failed and unused chunks should be zero")
		}
		e.clean("rec")
		flags.version = v[0]
		e.recover()
		e.deleteVersion(v[0])
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numUnused == 0 {
			e.t.Fatalf("Number of unused chunks should be more than zero")
		}
		e.purgeOldData()
		numChunks, numOk, numFailed, numUnused = e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numChunks != 0 || numOk != 0 || numFailed != 0 || numUnused != 0 {
			e.t.Errorf("There should be zero chunks left")
		}
	})
}

func TestT20(t *testing.T) {
	n := 100000
	if !*longTest {
		n = 1000
	}
	totalTest(t, n, "")
}

func TestT21(t *testing.T) {
	n := 100000
	if !*longTest {
		n = 1000
	}
	totalTest(t, n, "nc784hlisjdfhlskhdfo8wnkljdsbvliv9odhvilsdfh")
}

func totalTest(t *testing.T, n int, pw string) {
	doTestSeq(t, fmt.Sprintf("Total test %d files", n), func(e *TestEnv) {
		e.setPW([]byte(pw))
		e.init()
		var td time.Duration
		tf := func(m string, f func()) time.Duration {
			start := time.Now()
			f()
			d := time.Now().Sub(start)
			e.t.Log(m, d)
			return d
		}
		td += tf("Create files completed:", func() {
			e.t.Log("Starting file creation")
			for i := 0; i < n; i++ {
				e.addFile(fmt.Sprintf("d%d/e%d/f%d", i/10000, i/100, i), i, i)
			}
		})
		td += tf("Backup completed", func() {
			e.backup()
		})
		td += tf("Verify backups completed", func() {
			numChunks, numOk, numFailed, numUnused := e.verifyBackups()
			e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
			if numFailed != 0 || numUnused != 0 {
				e.t.Error("Number of failed and unused chunks should be zero")
			}
		})
		td += tf("Files completed", func() {
			e.files("")
		})
		td += tf("Versions completed", func() {
			e.versions()
		})
		td += tf("Recover completed", func() {
			e.recover()
		})
		td += tf("CheckSame completed", func() {
			e.checkSame()
		})
		td += tf("Purge-old-data completed", func() {
			e.t.Log(e.purgeOldData())
		})
		td += tf("Verify backups completed", func() {
			numChunks, numOk, numFailed, numUnused := e.verifyBackups()
			e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
			if numFailed != 0 || numUnused != 0 {
				e.t.Error("Number of failed and unused chunks should be zero")
			}
		})
		e.t.Logf("Completed: %v", td)
	})
}

func TestT22(t *testing.T) {
	N := 100
	NF := 1000
	if !*longTest {
		N = 100
		NF = 100
	}
	rand.Seed(2)
	doTestSeq(t, "Random test", func(e *TestEnv) {
		e.setPW([]byte("ofinewnfd;fnadsfsmocewmocwfdsafsdafsadf"))
		flags.chunkSize = 99
		e.init()
		vf := make([][]string, N)
		nodes := make([]string, NF)
		for iter := 0; iter < N; iter++ {
			randomizeFiles(e, nodes)
			e.backup()
			vf[iter] = e.files("")
			e.clean("rec")
			e.recover()
			e.checkSame()
		}
		versions := e.versions()
		for iter := 0; iter < N; iter++ {
			e.clean("rec")
			flags.version = versions[iter]
			rf := e.recover()
			if !reflect.DeepEqual(vf[iter], rf) {
				e.t.Fatalf("Not equal, iter %d: %v\n%v", iter, vf[iter], rf)
			}
		}
		numChunks, numOk, numFailed, numUnused := e.verifyBackups()
		e.t.Logf("%d numChunks %d numOk, %d numFailed, %d numUnused", numChunks, numOk, numFailed, numUnused)
		if numFailed != 0 || numUnused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
	})
}

func randomizeFiles(e *TestEnv, nodes []string) {
	data := make([]byte, len(nodes)+100)
	rand.Read(data)
	for i := 0; i < len(nodes); i++ {
		r := rand.Float64()
		if r < 0.5 {
			continue
		}
		f := fmt.Sprintf("%03d", i)
		f = path.Join(f[:1], f[1:2], f[2:])
		if nodes[i] != "" {
			e.tryrm(f)
			e.tryrm(f[:4])
			e.tryrm(f[:2])
		}
		if r < 0.6 {
			nodes[i] = ""
			continue
		} else if r < 0.7 {
			e.addSymlink(f, fmt.Sprintf("blah"+f))
			nodes[i] = "s"
		} else if r < 0.8 {
			e.addDir(f)
			nodes[i] = "d"
		} else {
			e.addFileWithData(f, data[rand.Intn(100+i):])
			nodes[i] = "f"
		}
	}
	numDirs, numFiles, numSymlinks := 0, 0, 0
	for i := 0; i < len(nodes); i++ {
		if nodes[i] == "d" {
			numDirs++
		} else if nodes[i] == "f" {
			numFiles++
		} else if nodes[i] == "s" {
			numSymlinks++
		}
	}
	e.t.Logf("Num dirs %d, num files %d, num symlinks %d", numDirs, numFiles, numSymlinks)
}

func benchmarkBackup(numFiles int, b *testing.B) {
	doTestSeq(b, "benchmark backup", func(e *TestEnv) {
		for i := 0; i < numFiles; i++ {
			e.addFile(fmt.Sprintf("d%d/e%d/f%d", i/10000, i/100, i), i, i)
		}
		e.backup()
	})
}

func BenchmarkBackup1k(b *testing.B) {
	benchmarkBackup(1000, b)
}

func BenchmarkBackup10k(b *testing.B) {
	benchmarkBackup(10000, b)
}

func BenchmarkBackup20k(b *testing.B) {
	benchmarkBackup(20000, b)
}

func BenchmarkBackup40k(b *testing.B) {
	benchmarkBackup(40000, b)
}

func BenchmarkBackup60k(b *testing.B) {
	benchmarkBackup(60000, b)
}

func BenchmarkBackup80k(b *testing.B) {
	benchmarkBackup(80000, b)
}

func BenchmarkBackup100k(b *testing.B) {
	benchmarkBackup(100000, b)
}

func benchmarkBR(numFiles int, b *testing.B) {
	doTestSeq(b, "benchmark backup", func(e *TestEnv) {
		for i := 0; i < numFiles; i++ {
			e.addFile(fmt.Sprintf("d%d/e%d/f%d", i/10000, i/100, i), i, i)
		}
		e.backup()
		e.recover()
	})
}

func BenchmarkBR10k(b *testing.B) {
	benchmarkBR(10000, b)
}

func BenchmarkBR50k(b *testing.B) {
	benchmarkBR(50000, b)
}

func BenchmarkBR100k(b *testing.B) {
	benchmarkBR(100000, b)
}

func TestPathCompare(t *testing.T) {
	cases := []struct {
		path1, path2 string
		want         int
	}{
		{"", "", 0},
		{"a", "a", 0},
		{"a/b", "a/b", 0},
		{"a/b", "c", -1},
		{"a/b", "c/d", -1},
		{"a", "a/b", -1},
		{"a", "a/b/c", -1},
		{"a", "a b/b/c", -1},
		{"a/b", "a b/b", -1},
		{"a b/b", "a b/c", -1},
		{"d/e", "de", -1},
		{"d", "d.", -1},
	}
	for _, c := range cases {
		got := pathCompare(c.path1, c.path2)
		if got != c.want {
			t.Errorf("pathCompare(%s, %s) == %v, want %v", c.path1, c.path2, got, c.want)
		}
		got = pathCompare(c.path2, c.path1)
		if got != -c.want {
			t.Errorf("pathCompare(%s, %s) == %v, want %v", c.path2, c.path1, got, -c.want)
		}
	}
}

func TestMatchRecoveryPattern(t *testing.T) {
	cases := []struct {
		path, pat string
		want      bool
	}{
		{"", "", true},
		{"abc", "", true},
		{"abc/def", "", true},
		{"a/b", "a", true},
		{"a/b/c", "a", true},
		{"abc/def", "abc", true},
		{"abc/def/", "abc", true},
		{"abc/def/ghi", "abc", true},
		{"abc/def/ghi", "abc/def", true},
		{"abc/def", "a", false},
		{"abc/def", "ab", false},
		{"abc/def", "ab", false},
		{"abc/def", "abc/d", false},
		{"abc/def", "abc/de", false},
		{"abc", "abc", true},
		{"abc/def", "abc/def", true},
		{"abc/def/ghi", "abc/def/ghi", true},
		{"", "a", false},
	}
	for _, c := range cases {
		got := matchRecoveryPattern(c.path, c.pat)
		if got != c.want {
			t.Errorf("matchRecoveryPattern(%v, %v) == %v, want %v", c.path, c.pat, got, c.want)
		}
	}
}

func TestToIgnore(t *testing.T) {
	cases := []struct {
		patterns []string
		d        string
		f        string
		want     bool
	}{
		{[]string{}, "", "", false},
		{[]string{}, "", "a", false},
		{[]string{}, "", "abc", false},
		{[]string{"a"}, "", "abc", false},
		{[]string{"abc"}, "", "abc", true},
		{[]string{"a*c"}, "", "abc", true},
		{[]string{"*~"}, "", "abc", false},
		{[]string{"*~"}, "", "abc~", true},
		{[]string{".DS_Store", "*~"}, "", "abc~", true},
		{[]string{".DS_Store", "*~"}, "", ".DS_Store", true},
		{[]string{".DS_Store", "*~"}, "", ".DS_Store~", true},
		{[]string{".DS_Store", "*~"}, "", "a.DS_Store", false},
		{[]string{".DS_Store", "*~"}, "", ".DS_Storea", false},
		{[]string{"/a"}, "", "a", true},
		{[]string{"/a"}, "", "aa", false},
		{[]string{"/a"}, "b", "a", false},
		{[]string{"/a"}, "a", "a", false},
		{[]string{"/a/b"}, "a", "b", true},
		{[]string{"/a/b*"}, "a", "bzz", true},
		{[]string{"/a/b*"}, "a", "zz", false},
		{[]string{"/*/b"}, "a", "b", true},
		{[]string{"/*/b"}, "b", "b", true},
		{[]string{"/*/b"}, "z", "b", true},
		{[]string{"/*/b"}, "b", "z", false},
	}
	for _, c := range cases {
		got := toIgnore(c.patterns, c.d, c.f)
		if got != c.want {
			t.Errorf("toIgnore(%v, %v, %v) == %v, want %v", c.patterns, c.d, c.f, got, c.want)
		}
	}
}
