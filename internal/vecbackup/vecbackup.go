package vecbackup

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	VC_VERSION              = 1
	VC_MAGIC                = "VBKC"
	VV_VERSION              = 1
	VV_MAGIC                = "VBKV"
	CONFIG_FILE             = "vecbackup-config"
	VERSION_DIR             = "versions"
	CHUNK_DIR               = "chunks"
	VERSION_FILENAME_PREFIX = "version-"
	LOCK_FILENAME           = "lock"
	RESTORE_TEMP_SUFFIX     = ".vbk.restore.temp"
	DEFAULT_DIR_PERM        = 0700
	DEFAULT_FILE_PERM       = 0600
	PATH_SEP                = string(os.PathSeparator)
)

var stdout = log.New(os.Stdout, "", 0)
var stderr = log.New(os.Stderr, "", 0) //log.Lshortfile)
var debug = false

func SetDebug(dbg bool) {
	debug = dbg
}

func debugP(fmt string, v ...interface{}) {
	if debug {
		log.Printf(fmt, v...)
	}
}

func readExcludeFile(fn string) ([]string, error) {
	if fn == "" {
		return nil, nil
	}
	excludePatterns := []string{}
	in, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		l := scanner.Text()
		if len(l) > 0 {
			_, err := filepath.Match(l, "zzz")
			if err != nil {
				return nil, fmt.Errorf("Bad exclude pattern: %s", l)
			} else {
				excludePatterns = append(excludePatterns, l)
			}
		}
	}
	return excludePatterns, nil
}

func toExclude(excludePatterns []string, dir, fn string) bool {
	for _, p := range excludePatterns {
		var matched bool
		var err error
		if p[0] == os.PathSeparator {
			matched, err = filepath.Match(p, filepath.ToSlash(PATH_SEP+path.Join(dir, fn)))
		} else {
			matched, err = filepath.Match(p, filepath.ToSlash(fn))
		}
		if err == nil && matched {
			return true
		}
	}
	return false
}

func isSymlink(f os.FileInfo) bool {
	return f.Mode()&os.ModeSymlink != 0
}

type fileDataMap struct {
	names []string
	files map[string]*FileData
}

func (fdm *fileDataMap) Init() {
	fdm.files = make(map[string]*FileData)
	fdm.names = nil
}

func (fdm *fileDataMap) AddItem(fd *FileData) bool {
	n := fd.Name
	if fdm.files[n] == nil {
		fdm.files[n] = fd
		fdm.names = append(fdm.names, n)
		return true
	}
	return false
}

func scanOneDir(src string, f os.FileInfo, excludes []string, fdm *fileDataMap) int {
	if f == nil {
		if fi, err := os.Lstat(src); err != nil {
			stderr.Printf("F %s: %s\n", src, err)
			return 1
		} else {
			f = fi
		}
	}
	if f.IsDir() {
		if fdm.AddItem(NewDirectory(src, f.Mode().Perm())) {
			if files, err := ioutil.ReadDir(src); err != nil {
				stderr.Printf("F %s: %s\n", src, err)
				return 1
			} else {
				errs := 0
				for _, child := range files {
					if toExclude(excludes, src, child.Name()) {
						continue
					}
					errs2 := scanOneDir(path.Join(src, child.Name()), child, excludes, fdm)
					errs = errs + errs2
				}
				return errs
			}
		}
	} else if f.Mode().IsRegular() {
		fdm.AddItem(NewRegularFile(src, f.Size(), f.ModTime(), f.Mode().Perm(), nil, nil, nil))
	} else if isSymlink(f) {
		if target, err := os.Readlink(src); err != nil {
			stderr.Printf("F %s: %s\n", src, err)
			return 1
		} else {
			fdm.AddItem(NewSymlink(src, target))
		}
	}
	return 0
}

func scanSrcs(excludes []string, srcs []string) (*fileDataMap, int) {
	errs := 0
	fdm := &fileDataMap{}
	fdm.Init()
	for _, src := range srcs {
		src = filepath.Clean(src)
		err2 := scanOneDir(src, nil, excludes, fdm)
		errs = errs + err2
	}
	return fdm, errs
}

func makeChunkFP(secret []byte, origFp FP) FP {
	if secret == nil {
		return origFp
	}
	s := append(append([]byte(nil), secret...), origFp[:]...)
	return sha512.Sum512_256(s)
}

func matchChunkFP(secret []byte, fp FP, b []byte) bool {
	fp2 := makeChunkFP(secret, sha512.Sum512_256(b))
	return fp == fp2
}

func statsRemoveFile(fd *FileData, stats *BackupStats) {
	if fd != nil {
		if fd.IsFile() {
			stats.FilesRemoved++
		} else if fd.IsDir() {
			stats.DirsRemoved++
		} else if fd.IsSymlink() {
			stats.SymlinksRemoved++
		}
	}
}

func backupOneNode(cm *CMgr, mem *addChunkMem, dryRun, force, checkChunks, verbose bool, old *FileData, new *FileData, secret []byte, mu *sync.Mutex, stats *BackupStats) (*FileData, error) {
	mu.Lock()
	defer mu.Unlock()
	if old != nil && new == nil {
		statsRemoveFile(old, stats)
		if verbose {
			stdout.Printf("- %s\n", old.PrettyPrint())
		}
		return nil, nil
	}
	to_add := old
	if force || old == nil && new != nil || new.Type != old.Type || (new.IsFile() && (new.Size != old.Size || !new.ModTime.Equal(old.ModTime))) || (new.IsSymlink() && new.Target != old.Target) {
		to_add = new
	} else if old.IsFile() {
		if checkChunks {
			for _, chunk := range old.Chunks {
				if !cm.FindChunk(chunk) {
					stderr.Printf("Missing chunk %s from file %s\n", chunk, old.PrettyPrint())
					to_add = new
					break
				}
			}
		}
	}
	var addSrcSize int64 = 0
	var addRepoSize int64 = 0
	if to_add == new {
		if new.IsFile() {
			if !dryRun {
				mu.Unlock()
				srcAdded, repoAdded, err := addChunks(new, cm, mem, dryRun, secret)
				mu.Lock()
				if err != nil {
					stderr.Printf("F %s: %s\n", to_add.PrettyPrint(), err)
					stats.Errors++
					return nil, err
				} else {
					addSrcSize = srcAdded
					addRepoSize = repoAdded
				}
			}
			if old == nil || !old.IsFile() {
				stats.FilesNew++
				statsRemoveFile(old, stats)
			} else {
				stats.FilesUpdated++
			}
		} else if new.IsDir() {
			if old == nil || !old.IsDir() {
				stats.DirsNew++
				statsRemoveFile(old, stats)
			} else {
				stats.DirsUpdated++
			}
		} else if new.IsSymlink() {
			if old == nil || !old.IsSymlink() {
				stats.SymlinksNew++
				statsRemoveFile(old, stats)
			} else {
				stats.SymlinksUpdated++
			}
		}
		if verbose {
			stdout.Printf("+ %v\n", to_add.PrettyPrint())
		}
	} else {
		if !old.IsSymlink() {
			old.Perm = new.Perm
		}
	}
	if to_add.IsFile() {
		stats.Files++
		stats.Size += new.Size
		stats.SrcAdded += addSrcSize
		stats.RepoAdded += addRepoSize
	} else if to_add.IsDir() {
		stats.Dirs++
	} else {
		stats.Symlinks++
	}
	return to_add, nil
}

func addChunks(fd *FileData, cm *CMgr, mem *addChunkMem, dryRun bool, secret []byte) (int64, int64, error) {
	h := sha512.New512_256()
	var chunks []FP = nil
	var sizes []int32
	file, err := os.Open(fd.Name)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	var n int64 = 0
	var srcAdded int64 = 0
	var repoAdded int64 = 0
	blockSize := mem.chunkSize
	for {
		mem.setSize(blockSize)
		buf := mem.buf()
		count, err := io.ReadFull(file, buf)
		if count > 0 {
			mem.setSize(count)
			buf := mem.buf()
			n += int64(count)
			h.Write(buf)
			var chunk FP = makeChunkFP(secret, sha512.Sum512_256(buf))
			dup, compressedLen, err := cm.AddChunk(chunk, mem)
			if err != nil {
				return 0, 0, err
			}
			srcAdded = srcAdded + int64(count)
			if !dup {
				repoAdded = repoAdded + int64(compressedLen)
			}
			chunks = append(chunks, chunk)
			sizes = append(sizes, int32(count))
		}
		if n > fd.Size {
			return 0, 0, fmt.Errorf("File size changed %s", fd.Name)
		}
		if err == io.EOF || count < blockSize {
			if n < fd.Size {
				return 0, 0, fmt.Errorf("File size changed %s", fd.Name)
			}
			break
		}
	}
	fd.Chunks = chunks
	fd.Sizes = sizes
	fd.FileChecksum = h.Sum(nil)
	return srcAdded, repoAdded, nil
}

func matchRestorePatterns(fp string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if matchRestorePattern(fp, pat) {
			return true
		}
	}
	return false
}

func matchRestorePattern(p, pat string) bool {
	if p == pat {
		return true
	}
	l := len(pat)
	return l == 0 || (len(p) > l && p[:l] == pat && os.IsPathSeparator(p[l]))
}

func restoreFileToTemp(fd *FileData, cm *CMgr, mem *readChunkMem, fn string, verifyOnly bool, secret []byte) error {
	var f *os.File
	if !verifyOnly {
		d := filepath.Dir(fn)
		err := os.MkdirAll(d, DEFAULT_DIR_PERM)
		f, err = os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fd.Perm|DEFAULT_FILE_PERM)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	var l int64
	h := sha512.New512_256()
	for _, chunk := range fd.Chunks {
		b, err := cm.ReadChunk(chunk, mem)
		if err == nil && !matchChunkFP(secret, chunk, b) {
			err = fmt.Errorf("Bad chunk, mismatch fingerpint %s", chunk)
		}
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Missing chunk %s", chunk)
			} else {
				return fmt.Errorf("Bad chunk %s", chunk)
			}
		}
		h.Write(b)
		l += int64(len(b))
		if !verifyOnly {
			_, err = f.Write(b)
			if err != nil {
				return err
			}
		}
	}
	if l != fd.Size {
		return fmt.Errorf("Length mismatch: %d vs %d", l, fd.Size)
	}
	cs := h.Sum(nil)
	if bytes.Compare(fd.FileChecksum, cs) != 0 {
		fmt.Println("CHECKSUMS", fd.FileChecksum, cs)
		return errors.New("File checksum mismatch")
	}
	return nil
}

func restoreDir(fd *FileData, resDir string, dryRun bool) (bool, error) {
	p := path.Join(resDir, fd.Name)
	fi, err := os.Lstat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		if dryRun {
			return true, nil
		}
		return true, os.MkdirAll(p, fd.Perm|DEFAULT_DIR_PERM)
	}
	if !fi.IsDir() {
		return false, errors.New("Cannot dir. File/symlink already exist at the path.")
	}
	if (fi.Mode() & 0300) != 0300 {
		return false, errors.New("Directory is not writable.")
	}
	return false, nil
}

func restoreSymlink(fd *FileData, resDir string, dryRun bool) (bool, error) {
	p := path.Join(resDir, fd.Name)
	fi, err := os.Lstat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		if dryRun {
			return true, nil
		}
		return true, os.Symlink(fd.Target, p)
	}
	if !isSymlink(fi) {
		return false, errors.New("Cannot restore symlink. File/dir already exist at the path.")
	}
	target, err := os.Readlink(p)
	if err != nil {
		return false, err
	}
	if target != fd.Target {
		return false, errors.New("Cannot restore symlink. Existing symlink points to wrong target.")
	}
	return false, nil
}

func restoreFile(fd *FileData, cm *CMgr, mem *readChunkMem, resDir string, merge, verifyOnly, dryRun bool, secret []byte) (bool, error) {
	p := path.Join(resDir, fd.Name)
	if merge {
		if fi, err := os.Lstat(p); err == nil && fi.Size() == fd.Size && fi.ModTime().Equal(fd.ModTime) {
			return false, nil
		}
	}
	if dryRun {
		return true, nil
	}
	tp := p + RESTORE_TEMP_SUFFIX
	err := restoreFileToTemp(fd, cm, mem, tp, verifyOnly, secret)
	if err == nil {
		if verifyOnly {
			return true, nil
		}
		err = os.Chtimes(tp, fd.ModTime, fd.ModTime)
		if err == nil {
			err = os.Chmod(tp, fd.Perm)
			if err == nil {
				err = os.Rename(tp, p)
				if err == nil {
					return true, nil
				}
			}
		}
	}
	if tp != "" {
		os.Remove(tp)
	}
	return false, err
}

func setup(repo string, pwFile string) (*VMgr, *CMgr, *Config, error) {
	sm, repo2 := GetStorageMgr(repo)
	cfg, err := GetConfig(pwFile, sm, repo2)
	if err != nil {
		return nil, nil, nil, err
	}
	vm := MakeVMgr(sm, repo2, cfg.EncryptionKey)
	cm := MakeCMgr(sm, repo2, cfg.EncryptionKey, cfg.Compress)
	return vm, cm, cfg, nil
}

//---------------------------------------------------------------------------

func InitRepo(pwFile, repo string, chunkSize int32, iterations int, compress CompressionMode) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	sm, repo2 := GetStorageMgr(repo)
	files, err := sm.LsDir(repo2)
	if !os.IsNotExist(err) && len(files) != 0 {
		return fmt.Errorf("Backup repository already exists: %s", repo)
	}
	err = sm.MkdirAll(repo2)
	if err != nil {
		return fmt.Errorf("Cannot create repo dir: %s", err)
	}
	cfg := &Config{ChunkSize: chunkSize, Compress: compress}
	err = WriteNewConfig(pwFile, sm, repo2, iterations, cfg)
	if err != nil {
		return fmt.Errorf("Cannot write encypted config file: %s", err)
	}
	return err
}

type BackupStats struct {
	Version         string
	Dirs            int
	DirsNew         int
	DirsUpdated     int
	DirsRemoved     int
	Files           int
	FilesUpdated    int
	FilesNew        int
	FilesRemoved    int
	Symlinks        int
	SymlinksNew     int
	SymlinksUpdated int
	SymlinksRemoved int
	Errors          int
	Size            int64
	SrcAdded        int64
	RepoAdded       int64
}

func Backup(pwFile, repo, excludeFrom, setVersion string, dryRun, force, checkChunks, verbose bool, lockFile string, maxDop int, srcs []string, stats *BackupStats) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if len(srcs) == 0 {
		return errors.New("At least one backup src must be specified")
	}
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	var sml StorageMgr
	var lockFile2 string
	if lockFile == "" {
		var repo2 string
		sml, repo2 = GetStorageMgr(repo)
		lockFile = sml.JoinPath(repo, LOCK_FILENAME)
		lockFile2 = sml.JoinPath(repo2, LOCK_FILENAME)
	} else {
		sml, lockFile2 = GetStorageMgr(lockFile)
	}
	excludePatterns, err := readExcludeFile(excludeFrom)
	if err != nil {
		return fmt.Errorf("Cannot read exclude-from file: %s", err)
	}
	if err = sml.WriteLockFile(lockFile2); os.IsExist(err) {
		return fmt.Errorf("Repository is locked. Lock file %s exists.", lockFile)
	} else if err != nil {
		return err
	}
	defer sml.RemoveLockFile(lockFile2)
	var new_version string
	if setVersion != "" {
		if _, ok := DecodeVersionTime(setVersion); ok {
			new_version = setVersion
		} else {
			return fmt.Errorf("Invalid version %s", setVersion)
		}
	}
	last_version, err := vm.GetLatestVersion()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	if new_version == "" {
		new_version = CreateNewVersion(last_version)
	}
	if verbose {
		stdout.Println("Scanning sources...")
	}
	sfdm, errs := scanSrcs(excludePatterns, srcs)
	stats.Errors += errs
	if len(sfdm.names) == 0 {
		return errors.New("Nothing to back up.")
	}
	var vfdm = &fileDataMap{}
	vfdm.Init()
	if last_version != "" {
		fds, err, errs := vm.LoadFiles(last_version)
		if err != nil {
			return fmt.Errorf("Failed reading previous version: %s", err)
		}
		stats.Errors += errs
		for _, fd := range fds {
			if !vfdm.AddItem(fd) {
				stderr.Printf("Ignoring duplicate item in version file: %s.\n", fd.Name)
			}
		}
	}
	comb := append(append([]string(nil), vfdm.names...), sfdm.names...)
	sort.Strings(comb)
	if verbose {
		if last_version == "" {
			stdout.Println("Starting inital backup...")
		} else {
			stdout.Printf("Starting backup from last version %s ...", last_version)
		}
	}
	//cm.CacheChunkInfo()
	var last string
	var fds []*FileData
	var wg sync.WaitGroup
	var mu sync.Mutex // protect fds and stats
	ch := make(chan *addChunkMem, maxDop)
	for i := 0; i < maxDop; i++ {
		ch <- makeAddChunkMem(int(cfg.ChunkSize))
	}
	for _, n := range comb {
		if n == last {
			continue
		}
		last = n
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			mem := <-ch
			defer func() { ch <- mem }()
			new_fd, err := backupOneNode(cm, mem, dryRun, force, checkChunks, verbose, vfdm.files[name], sfdm.files[name], cfg.FPSecret, &mu, stats)
			if err == nil && new_fd != nil {
				mu.Lock()
				fds = append(fds, new_fd)
				mu.Unlock()
			}
		}(n)
	}
	wg.Wait()
	for i := 0; i < maxDop; i++ {
		<-ch
	}
	ch = nil
	if !dryRun {
		if err = vm.SaveFiles(new_version, fds); err != nil {
			return err
		}
		stats.Version = new_version
	}
	return nil
}

func Restore(pwFile, repo, resDir, version string, merge, verifyOnly, dryRun, verbose bool, maxDop int, patterns []string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if resDir == "" && !verifyOnly {
		return errors.New("Target must be specified.")
	}
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	if !merge {
		if _, err := os.Lstat(resDir); !os.IsNotExist(err) {
			return fmt.Errorf("Restore dir %s already exists", resDir)
		}
	}
	if version == "" {
		version, err = vm.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("Cannot read version files: %s", err)
		}
		if version == "" {
			return errors.New("Error! Nothing to restore. Repository is empty")
		}
	}
	allFiles, err, errs := vm.LoadFiles(version)
	if err != nil {
		return fmt.Errorf("Cannot read version file: %s", err)
	}
	fdm := make(map[string]*FileData)
	var names []string
	for _, fd := range allFiles {
		if matchRestorePatterns(fd.Name, patterns) {
			fdm[fd.Name] = fd
			names = append(names, fd.Name)
		}
	}
	sort.Strings(names)
	allFiles = nil
	for _, name := range names {
		fd := fdm[name]
		if fd.IsDir() || fd.IsSymlink() {
			var acted = true
			if !verifyOnly {
				if fd.IsDir() {
					acted, err = restoreDir(fd, resDir, dryRun)
				} else {
					acted, err = restoreSymlink(fd, resDir, dryRun)
				}
			}
			if err == nil {
				if verbose && acted {
					stdout.Printf("%s\n", fd.PrettyPrint())
				}
			} else {
				stderr.Printf("F %s: %s\n", fd.PrettyPrint(), err)
				errs++
			}
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex // protects errs
	ch := make(chan *readChunkMem, maxDop)
	for i := 0; i < maxDop; i++ {
		ch <- &readChunkMem{}
	}
	for _, name := range names {
		fd := fdm[name]
		if fd.IsFile() {
			wg.Add(1)
			go func(fd *FileData) {
				defer wg.Done()
				mem := <-ch
				defer func() { ch <- mem }()
				acted, e := restoreFile(fd, cm, mem, resDir, merge, verifyOnly, dryRun, cfg.FPSecret)
				if e == nil {
					if verbose && acted {
						stdout.Printf("%s\n", fd.PrettyPrint())
					}
				} else {
					stderr.Printf("F %s: %s\n", fd.PrettyPrint(), e)
					mu.Lock()
					errs++
					mu.Unlock()
				}
			}(fd)
		}
	}
	wg.Wait()
	for i := 0; i < maxDop; i++ {
		<-ch
	}
	if !dryRun && !verifyOnly {
		for i := len(names) - 1; i >= 0; i-- {
			fd := fdm[names[i]]
			if fd.IsDir() {
				fi, err := os.Lstat(path.Join(resDir, fd.Name))
				if err != nil || fi.Mode() != fd.Perm {
					err = os.Chmod(path.Join(resDir, fd.Name), fd.Perm)
					if err != nil {
						stderr.Printf("F %s: %s\n", fd.PrettyPrint(), err)
						errs++
					}
				}
			}
		}
	}
	if errs > 0 {
		return errors.New("Errors occured during restore. Some files were not restored.")
	}
	return nil
}

func Ls(pwFile, repo, version string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	if version == "" {
		version, err = vm.GetLatestVersion()
	}
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	} else if version == "" {
		return nil
	}
	fds, err, errs := vm.LoadFiles(version)
	if err != nil {
		return fmt.Errorf("Cannot read version file: %s", err)
	}
	for _, fd := range fds {
		stdout.Printf("%s\n", fd.PrettyPrint())
	}
	if errs > 0 {
		return errors.New("Error! Some file info were invalid.")
	}
	return nil
}

func Versions(pwFile, repo string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	for _, v := range versions {
		stdout.Printf("%s\n", v)
	}
	return nil
}

func DeleteVersion(pwFile, repo, version string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if version == "" {
		return errors.New("Version must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	err = vm.DeleteVersion(version)
	if err != nil {
		return fmt.Errorf("Cannot delete version %s: %s", version, err)
	}
	return nil
}

func DeleteOldVersions(pwFile, repo string, dryRun bool) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	d := ReduceVersions(time.Now(), versions)
	for _, v := range d {
		stdout.Printf("Deleting version %s\n", v)
		if !dryRun {
			err = vm.DeleteVersion(v)
			if err != nil {
				return fmt.Errorf("Cannot delete version %s: %s", v, err)
			}
		}
	}
	return nil
}

type VerifyRepoResults struct {
	Chunks, Ok, Errors, Missing, Unused int
}

func VerifyRepo(pwFile, repo string, quick bool, maxDop int, r *VerifyRepoResults) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	allChunks := cm.GetAllChunks()
	allOk := make(map[FP]bool)
	allErrors := make(map[FP]bool)
	allMissing := make(map[FP]bool)
	fail := false
	totalFiles := 0
	totalBadFiles := 0
	totalDirs := 0
	totalSymlinks := 0
	var wg sync.WaitGroup
	var mu sync.Mutex // protects vr, allOk, allErrors, AllMissing
	ch := make(chan *readChunkMem, maxDop)
	for i := 0; i < maxDop; i++ {
		ch <- &readChunkMem{}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	for _, v := range versions {
		fds, err, nerrs := vm.LoadFiles(v)
		if err != nil {
			stderr.Printf("Failed to read version %s : %s\n", v, err)
			fail = true
			continue
		}
		if nerrs > 0 {
			fail = true
		}
		vChunks := make(map[FP]int32)
		for _, fd := range fds {
			for i, chunk := range fd.Chunks {
				vChunks[chunk] = fd.Sizes[i]
			}
		}
		var vr VerifyRepoResults
		vr.Chunks = len(vChunks)
		var size int64
		mu.Lock()
		for chunk, chunkSize := range vChunks {
			size = size + int64(chunkSize)
			if allOk[chunk] {
				vr.Ok++
			} else if allErrors[chunk] {
				vr.Errors++
			} else if allMissing[chunk] {
				vr.Missing++
			} else if quick {
				if cm.FindChunk(chunk) {
					vr.Ok++
					allOk[chunk] = true
				} else {
					stderr.Printf("Missing chunk %s\n", chunk)
					vr.Missing++
					allMissing[chunk] = true
				}
			} else {
				wg.Add(1)
				go func(cc FP) {
					defer wg.Done()
					mem := <-ch
					defer func() { ch <- mem }()
					b, err := cm.ReadChunk(cc, mem)
					if err == nil && !matchChunkFP(cfg.FPSecret, cc, b) {
						err = fmt.Errorf("Mismatch fingerpint %s", cc)
					}
					if err != nil {
						if os.IsNotExist(err) {
							stderr.Printf("Missing chunk %s\n", cc)
							mu.Lock()
							vr.Missing++
							allMissing[cc] = true
							mu.Unlock()
						} else {
							stderr.Printf("Error chunk %s\n", cc)
							mu.Lock()
							vr.Errors++
							allErrors[cc] = true
							mu.Unlock()
						}
					} else {
						mu.Lock()
						vr.Ok++
						allOk[cc] = true
						mu.Unlock()
					}
				}(chunk)
			}
		}
		mu.Unlock()
		wg.Wait()
		badFiles := 0
		files := 0
		dirs := 0
		symlinks := 0
		for _, fd := range fds {
			if fd.IsFile() {
				files++
				for _, chunk := range fd.Chunks {
					if allErrors[chunk] || allMissing[chunk] {
						badFiles++
						stderr.Printf("F %s\n", fd.Name)
						break
					}
				}
			} else if fd.IsDir() {
				dirs++
			} else {
				symlinks++
			}
		}
		totalFiles += files
		totalBadFiles += badFiles
		totalDirs += dirs
		totalSymlinks += symlinks
		nerrsMsg := ""
		if nerrs > 0 {
			nerrsMsg = fmt.Sprintf(" %d invalid file info.\n", nerrs)
		}
		chunkMsg := ""
		if !quick {
			chunkMsg = fmt.Sprintf("%d good, %d bad, ", vr.Ok, vr.Errors)
		}
		stdout.Printf("Version %s : %d bytes, %d chunk(s), %s%d missing. %d files, %d bad. %d dirs. %d symlinks.%s", v, size, vr.Chunks, chunkMsg, vr.Missing, files, badFiles, dirs, symlinks, nerrsMsg)
	}
	for i := 0; i < maxDop; i++ {
		<-ch
	}
	r.Chunks = len(allChunks)
	r.Ok = len(allOk)
	r.Errors = len(allErrors)
	r.Missing = len(allMissing)
	for chunk, _ := range allChunks {
		if !(allOk[chunk] || allErrors[chunk] || allMissing[chunk]) {
			r.Unused++
		}
	}
	chunkMsg := ""
	if !quick {
		chunkMsg = fmt.Sprintf("%d good, %d bad, ", r.Ok, r.Errors)
	}
	stdout.Printf("Summary: %d chunk(s), %s%d missing, %d unused. %d files, %d bad. %d dirs. %d symlinks.\n", r.Chunks, chunkMsg, r.Missing, r.Unused, totalFiles, totalBadFiles, totalDirs, totalSymlinks)
	if fail || r.Errors > 0 || r.Missing > 0 {
		return errors.New("Error verifying repo.")
	}
	return nil
}

func PurgeUnused(pwFile, repo string, dryRun, verbose bool) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, cm, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	counts := cm.GetAllChunks()
	total_chunks := len(counts)
	for _, v := range versions {
		fds, err, errs := vm.LoadFiles(v)
		if err != nil {
			return fmt.Errorf("Cannot read version file: %s", err)
		}
		if errs > 0 {
			return fmt.Errorf("Error! Some file info were invalid in version %s", v)
		}
		for _, fd := range fds {
			for _, chunk := range fd.Chunks {
				delete(counts, chunk)
			}
		}
	}
	numDeleted := 0
	numFailed := 0
	for chunk, _ := range counts {
		if dryRun {
			if verbose {
				stdout.Printf("Delete %s\n", chunk)
			}
			numDeleted++
		} else {
			if err := cm.DeleteChunk(chunk); err != nil {
				numFailed++
				if verbose {
					stdout.Printf("Failed to delete %s: %s\n", chunk, err)
				}
			} else {
				numDeleted++
				if verbose {
					stdout.Printf("Deleted %s\n", chunk)
				}
			}
		}
	}
	if dryRun {
		stdout.Printf("Chunks to be purged (dryrun): %d out of %d.\n", numDeleted, total_chunks)
	} else {
		stdout.Printf("Chunks purged: %d out of %d.\n", numDeleted, total_chunks)
	}
	if numFailed > 0 {
		return fmt.Errorf("Failed to purge %d chunk(s).", numFailed)
	}
	return nil
}

func RemoveLock(repo, lockFile string) error {
	var sml StorageMgr
	var lockFile2 string
	if lockFile == "" {
		var repo2 string
		sml, repo2 = GetStorageMgr(repo)
		lockFile2 = sml.JoinPath(repo2, LOCK_FILENAME)
	} else {
		sml, lockFile2 = GetStorageMgr(lockFile)
	}
	err := sml.RemoveLockFile(lockFile2)
	if os.IsNotExist(err) {
		return fmt.Errorf("Repo is not locked. Lock file %s does not exist", lockFile2)
	}
	return nil
}
