package vecbackup

import (
	"bytes"
	"flag"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	SRCDIR = "/tmp/test_src"
	REPO   = "/tmp/test_bk"
	RESDIR = "/tmp/test_res"
	PWFILE = "/tmp/test_pw"
)

var longTest = flag.Bool("longtest", false, "Long test.")
var debugFlag = flag.Bool("debug", false, "Debug.")

var opt struct {
	Verbose     bool
	Force       bool
	DryRun      bool
	VerifyOnly  bool
	Version     string
	Merge       bool
	PwFile      string
	ChunkSize   int
	Iterations  int
	Repo        string
	Target      string
	ExcludeFrom string
	Compress    CompressionMode
	LockFile    string
	MaxDop      int
}

func setupTest(t testing.TB, name string) func() {
	opt.Verbose = false
	opt.Force = false
	opt.DryRun = false
	opt.VerifyOnly = false
	opt.Version = ""
	opt.Merge = false
	opt.PwFile = ""
	opt.ChunkSize = 16 * 1024 * 1024
	opt.Iterations = 9999
	opt.Repo = REPO
	opt.Target = RESDIR
	opt.ExcludeFrom = ""
	opt.Compress = CompressionMode_AUTO
	opt.LockFile = ""
	opt.MaxDop = 10
	stdout.SetOutput(ioutil.Discard)
	debug = *debugFlag
	removeAll(t, SRCDIR)
	removeAll(t, REPO)
	removeAll(t, RESDIR)
	removeAll(t, PWFILE)
	os.MkdirAll(SRCDIR, 0755)
	return func() { teardownTest(t) }
}

func teardownTest(t testing.TB) {
	removeAll(t, SRCDIR)
	removeAll(t, REPO)
	removeAll(t, RESDIR)
	removeAll(t, PWFILE)
}

func removeAll(t testing.TB, d string) {
	filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			os.Chmod(path, 0700)
		}
		return nil
	})
	err := os.RemoveAll(d)
	if err != nil {
		t.Fatalf("Can't remove test dir at %v: %v", d, err)
	}
	_ = os.Remove(d)
}

func makeBytePattern(n, offset int) []byte {
	b := make([]byte, n, n)
	for i := 0; i < n; i++ {
		b[i] = byte(offset + i)
	}
	return b
}

func makeRepeatedPattern(n, repeat, offset int) []byte {
	x := makeBytePattern(1+n/repeat, offset)
	var y []byte
	for len(y) < n {
		y = append(y, x...)
	}
	y = y[:n]
	return y
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
		opt.PwFile = ""
	} else {
		err := ioutil.WriteFile(PWFILE, pw, 0444)
		if err != nil {
			e.t.Fatalf("setPW failed: %s", err)
		}
		opt.PwFile = PWFILE
	}
}

func (e *TestEnv) init() {
	e.failIfError("init", InitRepo(opt.PwFile, opt.Repo, int32(opt.ChunkSize), opt.Iterations, opt.Compress))
}

func (e *TestEnv) backup() {
	wk, err := os.Getwd()
	e.failIfError("Getwd", err)
	e.failIfError("Chdir to srcdir", os.Chdir(SRCDIR))
	stats := &BackupStats{}
	e.failIfError("backup", Backup(opt.PwFile, opt.Repo, opt.ExcludeFrom, opt.Version, opt.DryRun, opt.Force, opt.Verbose, opt.LockFile, opt.MaxDop, []string{"."}, stats))
	e.failIfError("Chdir to test dir", os.Chdir(wk))
}

func (e *TestEnv) backupSrcs(srcs []string) {
	wk, err := os.Getwd()
	e.failIfError("Getwd", err)
	e.failIfError("Chdir to srcdir", os.Chdir(SRCDIR))
	stats := &BackupStats{}
	e.failIfError("backup", Backup(opt.PwFile, opt.Repo, opt.ExcludeFrom, opt.Version, opt.DryRun, opt.Force, opt.Verbose, opt.LockFile, opt.MaxDop, srcs, stats))
	e.failIfError("Chdir to test dir", os.Chdir(wk))
}

func (e *TestEnv) restore() []string {
	var b bytes.Buffer
	save := stdout
	stdout = log.New(&b, "", 0)
	defer func() { stdout = save }()
	e.failIfError("restore", Restore(opt.PwFile, opt.Repo, opt.Target, opt.Version, opt.Merge, opt.VerifyOnly, opt.DryRun, opt.Verbose, opt.MaxDop, nil))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) restoreFiles(patterns []string) []string {
	var b bytes.Buffer
	save := stdout
	stdout = log.New(&b, "", 0)
	defer func() { stdout = save }()
	e.failIfError("restore", Restore(opt.PwFile, opt.Repo, opt.Target, opt.Version, opt.Merge, opt.VerifyOnly, opt.DryRun, opt.Verbose, opt.MaxDop, patterns))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) verifyRepo() *VerifyRepoResults {
	var r VerifyRepoResults
	e.failIfError("verifyRepo", VerifyRepo(opt.PwFile, opt.Repo, false, opt.MaxDop, &r))
	return &r
}

func (e *TestEnv) purgeUnused() string {
	var b bytes.Buffer
	save := stdout
	stdout = log.New(&b, "", 0)
	defer func() { stdout = save }()
	e.failIfError("purgeUnused", PurgeUnused(opt.PwFile, opt.Repo, opt.DryRun, opt.Verbose))
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

func (e *TestEnv) addFileRepeatedPattern(f string, size, repeat, offset int) {
	e.addFileWithData(f, makeRepeatedPattern(size, repeat, offset))
}

func (e *TestEnv) addFileWithData(f string, data []byte) {
	p := path.Join(SRCDIR, f)
	d := filepath.Dir(p)
	err := os.MkdirAll(d, 0755)
	if err != nil {
		e.t.Fatalf("mkdirall failed: %s", err)
	}
	err = ioutil.WriteFile(p, data, 0444)
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
	err := os.MkdirAll(d, 0755)
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
	err := os.MkdirAll(p, 0755)
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

func (e *TestEnv) rmRes(f string) {
	err := os.Remove(path.Join(RESDIR, f))
	if err != nil {
		e.t.Fatalf("rm failed: %s: %s", f, err)
	}
}

func (e *TestEnv) versions() []string {
	var b bytes.Buffer
	save := stdout
	stdout = log.New(&b, "", 0)
	defer func() { stdout = save }()
	e.failIfError("versions", Versions(opt.PwFile, opt.Repo))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) deleteVersion(version string) {
	opt.Version = version
	e.failIfError("deleteVersion", DeleteVersion(opt.PwFile, opt.Repo, opt.Version))
}

func (e *TestEnv) ls(version string) []string {
	opt.Version = version
	var b bytes.Buffer
	save := stdout
	stdout = log.New(&b, "", 0)
	defer func() { stdout = save }()
	e.failIfError("ls", Ls(opt.PwFile, opt.Repo, opt.Version))
	r := strings.Split(b.String(), "\n")
	return r[:len(r)-1]
}

func (e *TestEnv) filesMatch(version string, l []string) {
	l2 := e.ls(version)
	sort.Strings(l2)
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
	if what == "res" {
		removeAll(e.t, RESDIR)
	} else if what == "src" {
		removeAll(e.t, SRCDIR)
		os.MkdirAll(SRCDIR, 0755)
	} else if what == "repo" {
		removeAll(e.t, REPO)
	} else {
		e.t.Fatalf("Invalid: clean %v", what)
	}
}

func (e *TestEnv) print(what string) {
	if what == "res" {
		e.t.Logf("%v", walkDir(e.t, RESDIR, nil))
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
	if !compareDir(e.t, SRCDIR, RESDIR) {
		e.t.Fatalf("compare failed")
	}
}

func (e *TestEnv) checkExistDir(f string) {
	s, err := os.Stat(path.Join(RESDIR, f))
	if err != nil || !s.Mode().IsDir() {
		e.t.Fatalf("Dir %v does not exist", f)
	}
}

func (e *TestEnv) checkExistFile(f string) {
	s, err := os.Stat(path.Join(RESDIR, f))
	if err != nil || !s.Mode().IsRegular() {
		e.t.Fatalf("File %v does not exist", f)
	}
}

func (e *TestEnv) checkNotExist(f string) {
	_, err := os.Stat(path.Join(RESDIR, f))
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
		if os.IsPermission(err1) {
			return true
		}
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
		if isSymlink(file1) && !isSymlink(file2) {
			t.Errorf("compareDir file2 should also be a symlink: %v %v", p1, p2)
			return false
		}
		if !file1.Mode().IsRegular() && !file1.IsDir() && !isSymlink(file1) {
			t.Errorf("compareDir file1 is not a file or dir or symlink: %v %v", p1, file1.Mode())
			return false
		}
		if !file2.Mode().IsRegular() && !file2.IsDir() && !isSymlink(file2) {
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
		e.restore()
		e.checkSame()
	})
	doTestSeq(t, "T01 Empty with PW", func(e *TestEnv) {
		e.setPW([]byte("hahahahahaa"))
		e.init()
		e.backup()
		e.restore()
		e.checkSame()
	})
}

func TestT02(t *testing.T) {
	doTestSeq(t, "T02 a few files", func(e *TestEnv) {
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		e.init()
		e.add("aaa")
		e.add("bb")
		e.add("c")
		e.addSymlink("d", "aaa")
		e.addSymlink("e", "../../zzz")
		e.addDir("f")
		e.backup()
		e.restore()
		e.checkSame()
	})
}

func TestT03(t *testing.T) {
	doTestSeq(t, "T03 files and dirs", func(e *TestEnv) {
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		e.init()
		e.add("z/aaa")
		e.add("z/bb")
		e.add("y/c")
		e.add("z/c")
		e.add("www/c")
		e.addSymlink("wwwx", "y/c")
		e.addSymlink("z/cc", "fsdfsdf/fsdfsdf/sdf/ds/fsd/")
		e.backup()
		e.restore()
		e.checkSame()
	})
}

func TestT04(t *testing.T) {
	doTestSeq(t, "T04 rm", func(e *TestEnv) {
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		e.init()
		e.add("z/aa")
		e.add("z/bb")
		e.add("y/cc")
		e.add("z/cc")
		e.add("w/dd")
		e.addSymlink("x/ee", "/tmp/fdsfsdf")
		e.backup()
		e.restore()
		e.checkSame()
		e.rm("z/bb")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("y/cc")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("y")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("w/dd")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("w")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("x/ee")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("x")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z/aa")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z/cc")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
	})
}

func TestT05(t *testing.T) {
	doTestSeq(t, "T05 file -> dir -> symlink -> file", func(e *TestEnv) {
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		e.init()
		e.add("z")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z")
		e.add("z/a")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z/a")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z")
		e.addSymlink("z", "what")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z")
		e.addSymlink("z", "huh")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.rm("z")
		e.add("z")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
	})
}

func TestT06(t *testing.T) {
	doTestSeq(t, "T06 Permissions", func(e *TestEnv) {
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		e.init()
		e.add("aaa")
		e.add("bbb")
		e.add("zz/ccc")
		e.add("zz/ddd")
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("aaa", 0444)
		e.chmod("bbb", 0400)
		e.chmod("zz/ccc", 0440)
		e.chmod("zz/ddd", 0755)
		e.chmod("zz", 0742)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("aaa", 0555)
		e.chmod("bbb", 0500)
		e.chmod("zz/ccc", 0550)
		e.chmod("zz/ddd", 0456)
		e.chmod("zz", 0750)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("aaa", 0666)
		e.chmod("bbb", 0600)
		e.chmod("zz/ccc", 0660)
		e.chmod("zz/ddd", 0654)
		e.chmod("zz", 0767)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("aaa", 0777)
		e.chmod("bbb", 0700)
		e.chmod("zz/ccc", 0700)
		e.chmod("zz/ddd", 0765)
		e.chmod("zz", 0713)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("zz", 0500)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
		e.chmod("zz", 0700)
		e.rm("zz/ccc")
		e.rm("zz/ddd")
		e.chmod("zz", 0400)
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
	})
}

func TestT07(t *testing.T) {
	doTestSeq(t, "T07 Block sizes", func(e *TestEnv) {
		for i := 1; i < 20; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("c%d", fs), fs-1, i)
		}
		e.setPW([]byte("030hfiuh983hfhshfdla"))
		sizes := []int{1 << 12, 1 << 16, 1 << 20, 1 << 24}
		for _, size := range sizes {
			opt.ChunkSize = size
			t.Logf("Testing block size: %d\n", size)
			e.init()
			e.backup()
			e.restore()
			e.checkSame()
			e.clean("repo")
			e.clean("res")
		}
	})
}

func TestT08(t *testing.T) {
	doTestSeq(t, "T08 backup paths", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/d")
		e.add("e/f")
		e.add("e g/f")
		e.backupSrcs([]string{"c"})
		e.filesMatch("", []string{"c/", "c/d"})
		e.clean("res")
		e.restore()
		e.checkNotExist("a")
		e.checkNotExist("b")
		e.checkExistDir("c")
		e.checkExistFile("c/d")
		e.checkNotExist("e")
		e.checkNotExist("e/f")
		e.checkNotExist("e g/f")
		e.backupSrcs([]string{"e", "c"})
		e.filesMatch("", []string{"c/", "c/d", "e/", "e/f"})
		e.clean("res")
		e.restore()
		e.checkNotExist("a")
		e.checkNotExist("b")
		e.checkExistDir("c")
		e.checkExistFile("c/d")
		e.checkExistDir("e")
		e.checkExistFile("e/f")
		e.checkNotExist("e g/f")
		e.backupSrcs([]string{"."})
		e.filesMatch("", []string{"./", "a", "b", "c/", "c/d", "e g/", "e g/f", "e/", "e/f"})
		e.clean("res")
		e.restore()
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
	doTestSeq(t, "T09 Restore -merge", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("d/d2/d3/d4")
		e.backup()
		e.restore()
		e.checkSame()
		e.rmRes("b")
		e.checkNotExist("b")
		e.rmRes("c/c2")
		e.rmRes("c")
		e.checkNotExist("c")
		opt.Merge = true
		e.restore()
		e.checkSame()
	})
}

func TestT10(t *testing.T) {
	doTestSeq(t, "T10 backup -n and files", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
		e.init()
		e.backup()
		e.filesMatch("", []string{"./"})
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("d/d2/d3/d4")
		opt.DryRun = true
		e.backup()
		e.filesMatch("", []string{"./"})
		opt.DryRun = false
		e.backup()
		e.filesMatch("", []string{"./", "a", "b", "c/", "c/c2", "d/", "d/d2/", "d/d2/d3/", "d/d2/d3/d4"})
		e.restore()
		e.checkSame()
	})
}

func TestT11(t *testing.T) {
	doTestSeq(t, "T11 ls -version and versions", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
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
		allFiles := []string{"./", "a", "b", "c/", "c/c2", "d"}
		e.filesMatch(v[0], allFiles[:2])
		e.filesMatch(v[1], allFiles[:3])
		e.filesMatch(v[2], allFiles[:5])
		e.filesMatch(v[3], allFiles)
	})
}

func TestT12(t *testing.T) {
	doTestSeq(t, "T12 backup -f", func(e *TestEnv) {
		opt.Force = true
		e.setPW([]byte("kjdhskcnilaenwcnksajdfnsadfsa"))
		e.init()
		for i := 0; i < 20; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("c%d", fs), fs-1, i)
		}
		e.backup()
		e.restore()
		e.checkSame()
		e.backup()
		e.clean("res")
		e.restore()
		e.checkSame()
	})
}

func TestT13(t *testing.T) {
	doTestSeq(t, "T13 restore -n and -t", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
		e.init()
		for i := 0; i < 5; i++ {
			fs := 1 << uint(i)
			e.addFile(fmt.Sprintf("a%d", fs), fs, i)
			e.addFile(fmt.Sprintf("b%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("zz/c%d", fs), fs-1, i)
			e.addSymlink(fmt.Sprintf("d%d", fs), fmt.Sprintf("aaa%d", fs))
			e.addDir(fmt.Sprintf("e%d", fs))
		}
		e.backup()
		t.Logf("dryrun=true, verifyonly=true\n")
		opt.DryRun = true
		opt.Verbose = true
		files := e.restore()
		sort.Strings(files)
		e.checkNotExist("")
		e.filesMatch("", files)
		t.Logf("dryrun=false, verifyonly=run\n")
		opt.DryRun = false
		opt.VerifyOnly = true
		files = e.restore()
		sort.Strings(files)
		e.checkNotExist("")
		e.filesMatch("", files)
		t.Logf("dryrun=false, verifyonly=run\n")
		opt.VerifyOnly = false
		e.restore()
		e.checkSame()
	})
}

func TestT14(t *testing.T) {
	doTestSeq(t, "T14 restore -version", func(e *TestEnv) {
		e.setPW([]byte("o9nohsfhjsdg89(*&^ih"))
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
		opt.Version = v[0]
		e.clean("res")
		e.restore()
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c")
		e.checkNotExist("c/c2")
		e.checkNotExist("d")
		opt.Version = v[1]
		e.clean("res")
		e.restore()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c")
		e.checkNotExist("c/c2")
		e.checkNotExist("d")
		opt.Version = v[2]
		e.clean("res")
		e.restore()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkExistDir("c")
		e.checkExistFile("c/c2")
		e.checkNotExist("d")
		opt.Version = v[3]
		e.clean("res")
		e.restore()
		e.checkSame()
	})
}

func TestT15(t *testing.T) {
	doTestSeq(t, "T15 restore files", func(e *TestEnv) {
		e.setPW([]byte("c0-3'[sof[sdjfoasdfoh"))
		e.init()
		e.add("a")
		e.add("b")
		e.add("c/c2")
		e.add("c/c3")
		e.add("d/d2/d3")
		e.backup()
		e.restoreFiles([]string{"a"})
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c/c2")
		e.checkNotExist("c/c3")
		e.checkNotExist("d/d2/d3")
		opt.Merge = true
		e.restoreFiles([]string{"b"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c/c2")
		e.checkNotExist("c/c3")
		e.checkNotExist("d/d2/d3")
		e.restoreFiles([]string{"c/c3"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c/c2")
		e.checkExistFile("c/c3")
		e.checkNotExist("d/d2/d3")
		e.restoreFiles([]string{"c"})
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkExistFile("c/c2")
		e.checkExistFile("c/c3")
		e.checkNotExist("d/d2/d3")
	})
}

func TestT16(t *testing.T) {
	doTestSeq(t, "T16 delete-version", func(e *TestEnv) {
		e.setPW([]byte("c0-3'[sof[sdjfoasdfoh"))
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
	doTestSeq(t, "T17 set-version", func(e *TestEnv) {
		e.setPW([]byte("c0-3'[sof[sdjfoasdfoh"))
		e.init()
		e.add("a")
		opt.Version = "2011-02-03T04:05:06.000000000Z"
		e.backup()
		e.add("b")
		opt.Version = "2011-02-03T04:01:06.000000000Z"
		e.backup()
		e.add("c")
		opt.Version = "2011-02-03T04:09:06.000000000Z"
		e.backup()
		opt.Version = ""
		e.clean("res")
		opt.Version = "2011-02-03T04:01:06.000000000Z"
		e.restore()
		e.checkExistFile("a")
		e.checkExistFile("b")
		e.checkNotExist("c")
		e.clean("res")
		opt.Version = "2011-02-03T04:05:06.000000000Z"
		e.restore()
		e.checkExistFile("a")
		e.checkNotExist("b")
		e.checkNotExist("c")
		e.clean("res")
		opt.Version = "2011-02-03T04:09:06.000000000Z"
		e.restore()
		e.checkExistFile("a")
		e.checkExistFile("c")
		e.checkExistFile("c")
	})
}

func TestT18(t *testing.T) {
	doTestSeq(t, "T18 verify-backups", func(e *TestEnv) {
		e.setPW([]byte("938hjofnslknfsldnlsadkfsjdf990"))
		e.init()
		for i := 0; i < 100; i++ {
			fs := 1024*i + 1
			e.addFile(fmt.Sprintf("f%d", fs), fs, i)
			e.addFile(fmt.Sprintf("d/g%d", fs), fs+1, i)
			e.addFile(fmt.Sprintf("z/y/x%d", fs), fs-1, i)
			e.addFile(fmt.Sprintf("d1/d2/d3/d4/d5/d6/d7/f%d", fs), fs-1, i)
		}
		e.backup()
		r := e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		opt.DryRun = true
		e.restore()
		e.checkNotExist("")
		opt.DryRun = false
		e.restore()
		e.checkSame()
	})
}

func TestT19(t *testing.T) {
	doTestSeq(t, "T19 purge-unused", func(e *TestEnv) {
		e.setPW([]byte("938hjofnslknfsldnlsadkfsjdf990"))
		e.init()
		for i := 0; i < 10; i++ {
			fs := 1024*i + 1
			e.addFile(fmt.Sprintf("f%d", fs), fs, i)
		}
		e.backup()
		r := e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		for i := 0; i < 10; i++ {
			fs := 2024*i + 1
			e.addFile(fmt.Sprintf("g%d", fs), fs, i)
		}
		e.backup()
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		opt.DryRun = true
		e.restore()
		e.checkNotExist("")
		opt.DryRun = false
		e.restore()
		e.checkSame()
		v := e.versions()
		e.purgeUnused()
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
		e.clean("res")
		opt.Version = v[0]
		e.restore()
		e.clean("res")
		opt.Version = v[1]
		e.restore()
		e.checkSame()
		opt.Version = ""
		e.deleteVersion(v[1])
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Unused == 0 {
			e.t.Fatal("Number of unused chunks should be more than zero")
		}
		e.purgeUnused()
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Fatal("Number of failed and unused chunks should be zero")
		}
		e.clean("res")
		opt.Version = v[0]
		e.restore()
		e.deleteVersion(v[0])
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Unused == 0 {
			e.t.Fatalf("Number of unused chunks should be more than zero")
		}
		e.purgeUnused()
		r = e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Chunks != 0 || r.Ok != 0 || r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
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
				e.addFileRepeatedPattern(fmt.Sprintf("d%d/e%d/f%d", i/10000, i/100, i), i, 3+i/20, i)
			}
		})
		td += tf("Backup completed", func() {
			e.backup()
		})
		td += tf("Verify backups completed", func() {
			r := e.verifyRepo()
			e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
			if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
				e.t.Error("Number of failed and unused chunks should be zero")
			}
		})
		td += tf("Files completed", func() {
			e.ls("")
		})
		td += tf("Versions completed", func() {
			e.versions()
		})
		td += tf("Restore completed", func() {
			e.restore()
		})
		td += tf("CheckSame completed", func() {
			e.checkSame()
		})
		td += tf("PurgeUnused completed", func() {
			e.t.Log(e.purgeUnused())
		})
		td += tf("Verify backups completed", func() {
			r := e.verifyRepo()
			e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
			if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
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
		opt.ChunkSize = 1029
		e.init()
		vf := make([][]string, N)
		nodes := make([]string, NF)
		for iter := 0; iter < N; iter++ {
			randomizeFiles(e, nodes)
			e.backup()
			vf[iter] = e.ls("")
			sort.Strings(vf[iter])
			e.clean("res")
			e.restore()
			e.checkSame()
		}
		versions := e.versions()
		for iter := 0; iter < N; iter++ {
			e.clean("res")
			opt.Version = versions[iter]
			opt.Verbose = true
			resFiles := e.restore()
			sort.Strings(resFiles)
			if !reflect.DeepEqual(vf[iter], resFiles) {
				e.t.Fatalf("Not equal, iter %d: %v\n%v", iter, vf[iter], resFiles)
			}
		}
		r := e.verifyRepo()
		e.t.Logf("%d Chunks, %d Ok, %d Errors, %d Missing, %d Unused", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		if r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Error("Number of failed and unused chunks should be zero")
		}
	})
}

func randomizeFiles(e *TestEnv, nodes []string) {
	data := make([]byte, 7*len(nodes)+100)
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
		} else if r < 0.9 {
			e.addFileWithData(f, data[rand.Intn(100+i*7):])
			nodes[i] = "f"
		} else {
			e.addFileRepeatedPattern(f, 1000+i*9, 3, i)
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

func TestT23(t *testing.T) {
	doTestSeq(t, "T23 share chunks when encrypted", func(e *TestEnv) {
		data := make([]byte, 5000)
		rand.Read(data)
		e.setPW([]byte("fsdfsdfadfsdfasdd2349fhcif"))
		e.init()
		e.addFileWithData("f1", data)
		e.backup()
		r := e.verifyRepo()
		if r.Chunks != 1 || r.Ok != 1 || r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Errorf("Should be 1, 1, 0, 0, 0: numChunks=%d numOk=%d numErrors=%d numMissing=%d numUnused=%d", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		}
		e.backup()
		r = e.verifyRepo()
		if r.Chunks != 1 || r.Ok != 1 || r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Errorf("Should be 1, 1, 0, 0, 0: numChunks=%d numOk=%d numErrors=%d numMissing=%d numUnused=%d", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		}
		e.addFileWithData("f2", data)
		e.backup()
		r = e.verifyRepo()
		if r.Chunks != 1 || r.Ok != 1 || r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Errorf("Should be 1, 1, 0, 0, 0: numChunks=%d numOk=%d numErrors=%d numMissing=%d numUnused=%d", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		}
	})
}

func TestT24(t *testing.T) {
	doTestSeq(t, "T23 repeated content with parallel backup", func(e *TestEnv) {
		data1 := make([]byte, 5000)
		data2 := make([]byte, 5000)
		data3 := make([]byte, 5000)
		rand.Read(data1)
		rand.Read(data2)
		rand.Read(data3)
		e.setPW([]byte("fsdfsdfadfsdfasdd2349fhcif"))
		opt.ChunkSize = 1000
		e.init()
		j := 0
		for i := 0; i < 1000; i++ {
			e.addFileWithData(fmt.Sprintf("f%d", j), data1)
			j++
			e.addFileWithData(fmt.Sprintf("f%d", j), data2)
			j++
			e.addFileWithData(fmt.Sprintf("f%d", j), data3)
			j++
		}
		e.backup()
		r := e.verifyRepo()
		if r.Chunks != 15 || r.Ok != 15 || r.Errors != 0 || r.Missing != 0 || r.Unused != 0 {
			e.t.Errorf("Should be 3, 3, 0, 0, 0: numChunks=%d numOk=%d numErrors=%d numMissing=%d numUnused=%d", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
		}
		e.restore()
		e.checkSame()
	})
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
		e.restore()
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

func TestMatchRestorePattern(t *testing.T) {
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
		got := matchRestorePattern(c.path, c.pat)
		if got != c.want {
			t.Errorf("matchRestorePattern(%v, %v) == %v, want %v", c.path, c.pat, got, c.want)
		}
	}
}

func TestToExclude(t *testing.T) {
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
		got := toExclude(c.patterns, c.d, c.f)
		if got != c.want {
			t.Errorf("toExclude(%v, %v, %v) == %v, want %v", c.patterns, c.d, c.f, got, c.want)
		}
	}
}
